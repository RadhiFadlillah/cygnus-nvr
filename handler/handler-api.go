package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	nurl "net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/julienschmidt/httprouter"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

var rxSavedVideo = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-(\d{2}:\d{2}:\d{2})\.mp4$`)

// APILogin is handler for POST /api/login
func (h *WebHandler) APILogin(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Decode request
	var request LoginRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	checkError(err)

	// Prepare function to generate session
	genSession := func(expTime time.Duration) {
		// Create session ID
		sessionID, err := uuid.NewV4()
		checkError(err)

		// Save session ID to cache
		strSessionID := sessionID.String()
		h.SessionCache.Set(strSessionID, request.Username, expTime)

		// Save user's session IDs to cache as well
		// useful for mass logout
		sessionIDs := []string{strSessionID}
		if val, found := h.UserCache.Get(request.Username); found {
			sessionIDs = val.([]string)
			sessionIDs = append(sessionIDs, strSessionID)
		}
		h.UserCache.Set(request.Username, sessionIDs, -1)

		// Return session ID to user in cookies
		http.SetCookie(w, &http.Cookie{
			Name:    "session-id",
			Value:   strSessionID,
			Path:    "/",
			Expires: time.Now().Add(expTime),
		})

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, strSessionID)
	}

	// Check if user's database is empty
	dbIsEmpty := false
	h.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("user"))
		dbIsEmpty = bucket == nil || bucket.Stats().KeyN == 0
		return nil
	})

	// If database still empty, and user uses default account, let him in
	if dbIsEmpty && request.Username == "admin" && request.Password == "admin" {
		genSession(time.Hour)
		return
	}

	// Get account data from database
	var hashedPassword []byte
	err = h.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("user"))
		if bucket == nil {
			return fmt.Errorf("user is not exist")
		}

		hashedPassword = bucket.Get([]byte(request.Username))
		if hashedPassword == nil {
			return fmt.Errorf("user is not exist")
		}

		return nil
	})
	checkError(err)

	// Compare password with database
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(request.Password))
	if err != nil {
		panic(fmt.Errorf("username and password don't match"))
	}

	// Calculate expiration time
	expTime := time.Hour
	if request.Remember > 0 {
		expTime = time.Duration(request.Remember) * time.Hour
	} else {
		expTime = -1
	}

	// Create session
	genSession(expTime)
}

// APILogout is handler for POST /api/logout
func (h *WebHandler) APILogout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Get session ID
	sessionID, err := r.Cookie("session-id")
	if err != nil {
		if err == http.ErrNoCookie {
			panic(fmt.Errorf("session is expired"))
		} else {
			panic(err)
		}
	}

	h.SessionCache.Delete(sessionID.Value)
	fmt.Fprint(w, 1)
}

// APIGetCameraList is handler for GET /api/camera
func (h *WebHandler) APIGetCameraList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	err := h.validateSession(r)
	checkError(err)

	// Read list of camera from database
	cameras := make(map[string]string)
	h.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("camera"))
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				camBucket := bucket.Bucket(k)
				camName := camBucket.Get([]byte("name"))
				cameras[string(k)] = string(camName)
			}
		}

		return nil
	})

	// Encode to JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&cameras)
	checkError(err)
}

// APISaveCamera is handler for POST /api/camera
func (h *WebHandler) APISaveCamera(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	err := h.validateSession(r)
	checkError(err)

	// Decode request
	var camera Camera
	err = json.NewDecoder(r.Body).Decode(&camera)
	checkError(err)

	// Make sure URL valid
	camera.URL = strings.TrimSuffix(camera.URL, "/")
	tmp, err := nurl.ParseRequestURI(camera.URL)
	if err != nil || tmp.Scheme == "" || tmp.Hostname() == "" {
		panic(fmt.Errorf("url is not valid"))
	}

	// Save camera to database
	h.DB.Update(func(tx *bolt.Tx) error {
		// Get camera bucket
		cameraBucket, _ := tx.CreateBucketIfNotExists([]byte("camera"))

		// Generate ID for new camera, and convert it to string
		if camera.ID == "" {
			id, _ := cameraBucket.NextSequence()
			camera.ID = fmt.Sprintf("%d", id)
		}

		// Save the new camera
		newCameraBucket, _ := cameraBucket.CreateBucketIfNotExists([]byte(camera.ID))
		newCameraBucket.Put([]byte("url"), []byte(tmp.String()))
		newCameraBucket.Put([]byte("name"), []byte(camera.Name))
		newCameraBucket.Put([]byte("username"), []byte(camera.Username))
		newCameraBucket.Put([]byte("password"), []byte(camera.Password))

		return nil
	})

	// Remove camera session cache
	h.CameraCache.Delete(camera.ID)

	fmt.Fprint(w, camera.ID)
}

// APIDeleteCamera is handler for DELETE /api/camera/:id
func (h *WebHandler) APIDeleteCamera(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	err := h.validateSession(r)
	checkError(err)

	// Decode request
	camID := ps.ByName("id")

	// Delete camera's session cache
	h.CameraCache.Delete(camID)

	// Delete camera in database
	h.DB.Update(func(tx *bolt.Tx) error {
		// Get camera bucket
		cameraBucket := tx.Bucket([]byte("camera"))
		if cameraBucket == nil {
			return nil
		}

		cameraBucket.DeleteBucket([]byte(camID))
		return nil
	})

	fmt.Fprint(w, 1)
}
