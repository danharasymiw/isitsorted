package main

import (
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

type statusDay struct {
	Date string
}

type statusData struct {
	Days []statusDay
}

var statusTmpl *template.Template

func init() {
	sub, _ := fs.Sub(staticFS, "static")
	funcs := template.FuncMap{
		"mkSlice": func(args ...string) []string { return args },
	}
	statusTmpl = template.Must(template.New("status.html").Funcs(funcs).ParseFS(sub, "status.html"))
}

func statusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		days := make([]statusDay, 90)
		for i := range days {
			d := now.AddDate(0, 0, -(89 - i))
			days[i] = statusDay{Date: d.Format("Jan 2, 2006")}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		statusTmpl.Execute(w, statusData{Days: days})
	}
}

func hostRouter(statusH, appH http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Host, "status.") {
			statusH.ServeHTTP(w, r)
			return
		}
		appH.ServeHTTP(w, r)
	})
}
