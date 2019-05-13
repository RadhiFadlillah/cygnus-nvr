//go:generate go run assets-generator.go

package main

import (
	"fmt"
	"net/http"
	"os"
	fp "path/filepath"
	"time"

	"github.com/RadhiFadlillah/cygnus-nvr/handler"
	"github.com/julienschmidt/httprouter"
	cch "github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
)

var (
	portNumber = 8081
	dbPath     = "cygnus-nvr.db"
)

func main() {
	// Make sure required directories exists
	err := os.MkdirAll(fp.Dir(dbPath), os.ModePerm)
	if err != nil {
		logrus.Fatalln("failed to create database dir:", err)
	}

	// Open database
	db, err := bbolt.Open(dbPath, os.ModePerm, nil)
	if err != nil {
		logrus.Fatalln("failed to open database:", err)
	}
	defer db.Close()

	// Serve app
	serveApp(db)
}

func serveApp(db *bbolt.DB) {
	// Prepare web handler
	hdl := handler.WebHandler{
		DB:           db,
		UserCache:    cch.New(time.Hour, 10*time.Minute),
		SessionCache: cch.New(time.Hour, 10*time.Minute),
		CameraCache:  cch.New(time.Hour, 10*time.Minute),
	}

	// Prepare router
	router := httprouter.New()

	router.GET("/js/*filepath", hdl.ServeJsFile)
	router.GET("/res/*filepath", hdl.ServeFile)
	router.GET("/css/*filepath", hdl.ServeFile)
	router.GET("/fonts/*filepath", hdl.ServeFile)

	router.GET("/", hdl.ServeIndexPage)
	router.GET("/login", hdl.ServeLoginPage)
	router.GET("/cam/:camID/live/playlist", hdl.ServeLivePlaylist)
	router.GET("/cam/:camID/live/stream/:index", hdl.ServeLiveSegment)

	router.POST("/api/login", hdl.APILogin)
	router.POST("/api/logout", hdl.APILogout)

	router.GET("/api/camera", hdl.APIGetCameraList)
	router.POST("/api/camera", hdl.APISaveCamera)
	router.DELETE("/api/camera/:id", hdl.APIDeleteCamera)

	router.GET("/api/user", hdl.APIGetUsers)
	router.POST("/api/user", hdl.APIInsertUser)
	router.DELETE("/api/user/:username", hdl.APIDeleteUser)

	router.GET("/api/setting", hdl.APIGetSetting)

	router.PanicHandler = func(w http.ResponseWriter, r *http.Request, arg interface{}) {
		http.Error(w, fmt.Sprint(arg), 500)
	}

	// Serve app
	addr := fmt.Sprintf(":%d", portNumber)
	logrus.Infoln("Serve NVR in", addr)
	logrus.Fatalln(http.ListenAndServe(addr, router))
}
