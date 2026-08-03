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

func newServer(rl *rateLimiter, ctr *counter, act *activityLog) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /is-sorted", rl.middleware(isSortedHandler(ctr)))
	mux.Handle("POST /check", rl.middleware(checkFormHandler(ctr, act)))
	mux.Handle("GET /count", countHandler(ctr))
	mux.Handle("GET /count/sorted", sortedCountHandler(ctr))
	mux.Handle("GET /count/not-sorted", notSortedCountHandler(ctr))
	mux.Handle("GET /activity", activityHandler(act))
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
	act := newActivityLog(20, filepath.Join(dataDir, "activity.json"))
	rl := newRateLimiter(100, time.Minute)
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, newServer(rl, ctr, act)))
}
