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
	"strings"
	"time"

	"github.com/sirupsen/logrus"
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
		Remember: 6,
	}

	// Encode request to JSON
	buffer := bytes.NewBuffer(nil)
	err = json.NewEncoder(buffer).Encode(&loginRequest)
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %v", err)
	}

	// Create HTTP request, then send it
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

	// Parse response
	btSessionID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse camera login response: %v", err)
	}

	// Save camera session id to cache
	sessionID := string(btSessionID)
	h.CameraCache.Set(cam.ID, sessionID, 6*time.Hour)

	// Add log
	logrus.Infoln("log in into camera", cam.ID)

	return sessionID, nil
}

func (h *WebHandler) proxyCameraLivePlaylist(cam Camera, w http.ResponseWriter) error {
	var err error

	// Fetch session ID for camera from the cache
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

	// Create HTTP request for getting playlist
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	req.AddCookie(&http.Cookie{
		Name:  "session-id",
		Value: strSessionID,
	})

	// Send request to camera. If it somehow failed, assume the camera is disconnected
	// and delete session id for this camera.
	resp, err := httpClient.Do(req)
	if err != nil {
		h.CameraCache.Delete(cam.ID)
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}
	defer resp.Body.Close()

	// Read playlist content and replace HLS URL
	playlistContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	newURL := path.Join("/", "cam", cam.ID, "live", "stream")
	strPlaylistContent := string(playlistContent)
	strPlaylistContent = strings.ReplaceAll(strPlaylistContent, "/live/stream", newURL)

	// Set response header
	w.Header().Set("Content-Type", "application/x-mpegURL")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Send the new playlist to writer
	_, err = w.Write([]byte(strPlaylistContent))
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	return nil
}

func (h *WebHandler) proxyCameraLiveStream(cam Camera, index string, w http.ResponseWriter) error {
	// Fetch session ID for camera from the cache
	sessionID, exist := h.CameraCache.Get(cam.ID)
	if !exist {
		return fmt.Errorf("failed to connect to camera %s: session is expired", cam.ID)
	}

	// Since our cache save data as interface, assert it as string
	strSessionID := sessionID.(string)

	// Create URL
	reqURL, err := nurl.ParseRequestURI(cam.URL)
	if err != nil || reqURL.Scheme == "" || reqURL.Hostname() == "" {
		return fmt.Errorf("failed to connect to camera %s: camera url is not valid", cam.ID)
	}
	reqURL.Path = path.Join("live", "stream", index)

	// Create HTTP request for getting live stream
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	req.AddCookie(&http.Cookie{
		Name:  "session-id",
		Value: strSessionID,
	})

	// Send request to camera. If it somehow failed, assume the camera is disconnected
	// and delete session id for this camera.
	resp, err := httpClient.Do(req)
	if err != nil {
		h.CameraCache.Delete(cam.ID)
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}
	defer resp.Body.Close()

	// Set response header
	w.Header().Set("Content-Type", "video/MP2T")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Copy result to writer
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to connect to camera %s: %v", cam.ID, err)
	}

	return nil
}
