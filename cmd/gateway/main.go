// Command gateway runs the public-facing "Is It Sorted?" HTTP API. It is a
// thin proxy: it renders HTML for HTMX clients and forwards everything else
// to the job service over HTTP. It holds no direct dependency on Redis or
// S3 — those live behind the job service now.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"
)

//go:embed static
var staticFS embed.FS

// Gateway holds the dependencies needed to serve the public HTTP API.
type Gateway struct {
	client   *JobClient
	limiter  *Limiter
	staticFS embed.FS
}

// Handler builds the top-level HTTP handler for the gateway.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /is-sorted", g.limiter.Middleware(http.HandlerFunc(g.submitHandler)))
	mux.HandleFunc("GET /is-sorted/{id}", g.statusHandler)
	mux.HandleFunc("GET /is-sorted/{id}/events", g.sseHandler)
	mux.HandleFunc("GET /upload", g.uploadHandler)
	mux.HandleFunc("POST /upload/{id}/check", g.uploadCheckHandler)
	mux.HandleFunc("GET /count", g.countHandler)
	mux.HandleFunc("GET /activity", g.activityHandler)
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs.html", http.StatusMovedPermanently)
	})

	staticSub, err := fs.Sub(g.staticFS, "static")
	if err != nil {
		staticSub = g.staticFS
	}
	mux.Handle("GET /", http.FileServerFS(staticSub))

	return mux
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jobServiceURL := os.Getenv("JOB_SERVICE_URL")
	if jobServiceURL == "" {
		logger.Error("JOB_SERVICE_URL is required")
		os.Exit(1)
	}

	gw := &Gateway{
		client:   NewJobClient(jobServiceURL),
		limiter:  NewLimiter(100),
		staticFS: staticFS,
	}

	statusMux := http.NewServeMux()
	staticSub, _ := fs.Sub(staticFS, "static")
	statusMux.HandleFunc("GET /", statusHandler(gw.client))
	statusMux.Handle("GET /status.css", http.FileServerFS(staticSub))

	handler := hostRouter(statusMux, gw.Handler())

	logger.Info("gateway starting", "port", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func hostRouter(statusH, appH http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Host) >= 7 && r.Host[:7] == "status." {
			statusH.ServeHTTP(w, r)
			return
		}
		appH.ServeHTTP(w, r)
	})
}

type statusDay struct {
	Date string
}

type componentStatus struct {
	Name   string
	Status string
}

type statusData struct {
	Days       []statusDay
	Components []componentStatus
}

var statusTmpl *template.Template

func init() {
	f, err := fs.ReadFile(staticFS, "static/status.html")
	if err != nil {
		panic(fmt.Sprintf("gateway: reading embedded status.html: %v", err))
	}
	statusTmpl = template.Must(template.New("status").Funcs(template.FuncMap{
		"mkSlice": func(args ...any) []any { return args },
	}).Parse(string(f)))
}

func statusHandler(client *JobClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		jobServiceStatus := "Operational"
		redisStatus := "Operational"
		storageStatus := "Operational"
		workerStatus := "Operational"

		_, body, err := client.Health(ctx)
		if err != nil {
			jobServiceStatus = "Degraded"
			redisStatus = "Unknown"
			storageStatus = "Unknown"
		} else {
			var health HealthResponse
			json.Unmarshal(body, &health)
			if health.Redis != "ok" {
				redisStatus = "Degraded"
			}
			if health.Storage != "ok" {
				storageStatus = "Degraded"
			}
		}

		days := make([]statusDay, 90)
		now := time.Now()
		for i := range days {
			days[i] = statusDay{Date: now.AddDate(0, 0, -i).Format("2006-01-02")}
		}

		data := statusData{
			Days: days,
			Components: []componentStatus{
				{Name: "API Gateway", Status: "Operational"},
				{Name: "Job Service", Status: jobServiceStatus},
				{Name: "Worker", Status: workerStatus},
				{Name: "Redis", Status: redisStatus},
				{Name: "Object Storage", Status: storageStatus},
			},
		}

		w.Header().Set("Content-Type", "text/html")
		statusTmpl.Execute(w, data)
	}
}
