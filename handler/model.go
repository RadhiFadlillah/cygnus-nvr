package handler

// User is person that given access to NVR
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Camera is camera that saved in NVR
type Camera struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest is login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember int    `json:"remember"`
}
