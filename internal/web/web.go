package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

func NewHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		p = path.Clean(p)

		data, err := fs.ReadFile(sub, p)
		if err != nil {
			p = "index.html"
			data, err = fs.ReadFile(sub, p)
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
		}

		ct := "text/plain"
		switch {
		case strings.HasSuffix(p, ".html"):
			ct = "text/html; charset=utf-8"
		case strings.HasSuffix(p, ".css"):
			ct = "text/css; charset=utf-8"
		case strings.HasSuffix(p, ".js"):
			ct = "application/javascript; charset=utf-8"
		case strings.HasSuffix(p, ".png"):
			ct = "image/png"
		case strings.HasSuffix(p, ".ico"):
			ct = "image/x-icon"
		case strings.HasSuffix(p, ".svg"):
			ct = "image/svg+xml"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		w.Write(data)
	})
}
