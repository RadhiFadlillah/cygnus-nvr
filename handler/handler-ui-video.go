package handler

import (
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

var httpClient = http.Client{Timeout: time.Minute}

// ServeLivePlaylist is handler for GET /cam/:camID/live/playlist
// which serve HLS playlist for live stream
func (h *WebHandler) ServeLivePlaylist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure login session still valid
	err := h.validateSession(r)
	checkError(err)

	camID := ps.ByName("camID")
	cam, err := h.getCamera(camID)
	checkError(err)

	err = h.proxyCameraLivePlaylist(cam, w)
	checkError(err)
}

// ServeLiveSegment is handler for GET /cam/:camID/live/stream/:index
// which serve the HLS segment for live stream
func (h *WebHandler) ServeLiveSegment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure login session still valid
	err := h.validateSession(r)
	checkError(err)

	camID := ps.ByName("camID")
	cam, err := h.getCamera(camID)
	checkError(err)

	idx := ps.ByName("index")
	err = h.proxyCameraLiveStream(cam, idx, w)
	checkError(err)
}
