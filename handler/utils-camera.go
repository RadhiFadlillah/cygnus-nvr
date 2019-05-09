package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	nurl "net/url"
	"path"
	"strconv"
	"strings"

	bolt "go.etcd.io/bbolt"
)

func (h *WebHandler) getCamera(id string) (Camera, error) {
	cam := Camera{}
	err := h.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("camera"))
		if bucket == nil {
			return fmt.Errorf("camera %s doesn't exist", id)
		}

		cameraBucket := bucket.Bucket([]byte(id))
		if cameraBucket == nil {
			return fmt.Errorf("camera %s doesn't exist", id)
		}

		cam.ID = id
		cam.URL = string(cameraBucket.Get([]byte("url")))
		cam.Name = string(cameraBucket.Get([]byte("name")))
		cam.Username = string(cameraBucket.Get([]byte("username")))
		cam.Password = string(cameraBucket.Get([]byte("password")))
		return nil
	})

	return cam, err
}

func (h *WebHandler) loginToCamera(cam Camera) (string, error) {
	// Create URL
	reqURL, err := nurl.ParseRequestURI(cam.URL)
	if err != nil || reqURL.Scheme == "" || reqURL.Hostname() == "" {
		return "", fmt.Errorf("camera url is not valid")
	}
	reqURL.Path = "/api/login"

	// Create login request
	loginRequest := LoginRequest{
		Username: cam.Username,
		Password: cam.Password,
		Remember: -1,
	}

	buffer := bytes.NewBuffer(nil)
	err = json.NewEncoder(buffer).Encode(&loginRequest)
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %v", err)
	}

	// Send request
	req, err := http.NewRequest("POST", reqURL.String(), buffer)
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send login request: %v", err)
	}
	defer resp.Body.Close()

	btSessionID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse camera login response: %v", err)
	}

	// Save camera session id to cache
	sessionID := string(btSessionID)
	h.CameraCache.Set(cam.ID, sessionID, -1)

	return sessionID, nil
}

func (h *WebHandler) proxyCameraLivePlaylist(cam Camera, w http.ResponseWriter) error {
	var err error

	// Check if camera's session id already cached
	sessionID, exist := h.CameraCache.Get(cam.ID)
	if !exist {
		sessionID, err = h.loginToCamera(cam)
		if err != nil {
			return fmt.Errorf("failed to login to camera %s: %v", cam.ID, err)
		}
	}

	// Since our cache save data as interface, assert it as string
	strSessionID := sessionID.(string)

	// Create URL
	reqURL, err := nurl.ParseRequestURI(cam.URL)
	if err != nil || reqURL.Scheme == "" || reqURL.Hostname() == "" {
		return fmt.Errorf("failed to connect to camera %s: camera url is not valid", cam.ID)
	}
	reqURL.Path = "/live/playlist"

	// Send request to camera
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	req.AddCookie(&http.Cookie{
		Name:  "session-id",
		Value: strSessionID,
	})

	resp, err := httpClient.Do(req)
	if err != nil {
		if err.Error() == "session is not exist" || err.Error() == "session has been expired" {
			h.CameraCache.Delete(cam.ID)
		}

		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	// Copy response header
	for key, vals := range resp.Header {
		for _, val := range vals {
			w.Header().Set(key, val)
		}
	}

	// Read playlist content and replace HLS URL
	playlistContent, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	newURL := path.Join("/", "cam", cam.ID, "live", "stream")
	strPlaylistContent := string(playlistContent)
	strPlaylistContent = strings.ReplaceAll(strPlaylistContent, "/live/stream", newURL)

	finalLength := len(strPlaylistContent)
	w.Header().Set("Content-Length", strconv.Itoa(finalLength))

	_, err = w.Write([]byte(strPlaylistContent))
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	return nil
}

func (h *WebHandler) proxyCameraLiveStream(cam Camera, index string, w http.ResponseWriter) error {
	// Check if camera's session id already cached
	sessionID, exist := h.CameraCache.Get(cam.ID)
	if !exist {
		return fmt.Errorf("failed to connect to camera %s: session is expired", cam.ID)
	}
	strSessionID := sessionID.(string)

	// Create URL
	reqURL, err := nurl.ParseRequestURI(cam.URL)
	if err != nil || reqURL.Scheme == "" || reqURL.Hostname() == "" {
		return fmt.Errorf("failed to connect to camera %s: camera url is not valid", cam.ID)
	}
	reqURL.Path = path.Join("live", "stream", index)

	// Send request to camera
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	req.AddCookie(&http.Cookie{
		Name:  "session-id",
		Value: strSessionID,
	})

	resp, err := httpClient.Do(req)
	if err != nil {
		if err.Error() == "session is not exist" || err.Error() == "session has been expired" {
			h.CameraCache.Delete(cam.ID)
		}
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	// Copy result to writer
	for key, vals := range resp.Header {
		for _, val := range vals {
			w.Header().Set(key, val)
		}
	}

	_, err = io.Copy(w, resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	return nil
}
