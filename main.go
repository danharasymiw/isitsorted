package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed static
var staticFS embed.FS

func newServer(rl *rateLimiter) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /is-sorted", rl.middleware(http.HandlerFunc(isSortedHandler)))
	mux.Handle("POST /check", rl.middleware(http.HandlerFunc(checkFormHandler)))
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return mux
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	rl := newRateLimiter(20, time.Minute)
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, newServer(rl)))
}
