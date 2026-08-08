// Command gateway runs the HTTP-facing "Is It Sorted?" API: job
// submission, SSE status streaming, presigned uploads, counter/activity
// endpoints, the static HTMX frontend, and a host-routed status page.
package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/gateway"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/ratelimit"
	"sorted/internal/storage"
)

//go:embed static
var staticFS embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		logger.Error("REDIS_URL is required")
		os.Exit(1)
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("invalid REDIS_URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opts)

	ctx := context.Background()
	store, err := storage.New(ctx, storage.Config{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
	})
	if err != nil {
		logger.Error("failed to create storage client", "error", err)
		os.Exit(1)
	}

	gw := gateway.New(
		queue.New(rdb),
		pubsub.New(rdb),
		store,
		counter.New(rdb),
		activity.New(rdb),
		ratelimit.New(rdb, 100, time.Minute),
		staticFS,
	)

	// Status page handler (host-routed on status.*)
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		logger.Error("failed to load static assets", "error", err)
		os.Exit(1)
	}
	statusMux := http.NewServeMux()
	statusMux.HandleFunc("GET /", statusHandler(rdb, store))
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

func statusHandler(rdb *redis.Client, store *storage.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		redisStatus := "Operational"
		if err := rdb.Ping(ctx).Err(); err != nil {
			redisStatus = "Degraded"
		}

		bucketStatus := "Operational"
		if _, err := store.GetState(ctx, "counter.json"); err != nil {
			bucketStatus = "Degraded"
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
				{Name: "Worker", Status: "Operational"},
				{Name: "Redis", Status: redisStatus},
				{Name: "Object Storage", Status: bucketStatus},
				{Name: "Website", Status: "Operational"},
			},
		}

		w.Header().Set("Content-Type", "text/html")
		statusTmpl.Execute(w, data)
	}
}
