// +build ignore

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
)

const (
	secretPath = "handler/secret-prod.go"
)

func main() {
	fmt.Println("writing", secretPath)

	// Create random 32-bit key
	btKey := make([]byte, 32)
	_, err := rand.Read(btKey)
	if err != nil {
		log.Fatalln(err)
	}

	// Convert key to safe string
	key := base64.URLEncoding.EncodeToString(btKey)
	key = key[:32]

	// Prepare content of secret file
	content := "" +
		"// +build !dev\n\n" +
		"package handler\n\n" +
		`var secretKey = []byte("` + key + `")` +
		"\n"

	// Save secret to file
	dst, err := os.Create(secretPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer dst.Close()

	_, err = dst.WriteString(content)
	if err != nil {
		log.Fatalln(err)
	}
}
