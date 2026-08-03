package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed static
var staticFS embed.FS

func newServer(rl *rateLimiter, ctr *counter) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /is-sorted", rl.middleware(isSortedHandler(ctr)))
	mux.Handle("POST /check", rl.middleware(checkFormHandler(ctr)))
	mux.Handle("GET /count", countHandler(ctr))
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return mux
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	ctr := newCounter(filepath.Join(dataDir, "count.json"))
	rl := newRateLimiter(20, time.Minute)
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, newServer(rl, ctr)))
}
