// Package portal serves the embedded CHIOTRON Enterprise AI Platform Web UI.
//
// In accordance with single-binary Go architecture, all static frontend assets
// are embedded directly into the Go binary at compile time via //go:embed.
// The Go server serves both the high-performance AI Gateway REST/SSE API and
// the user portal without requiring Node.js or Nginx at runtime.
package portal

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed dist/*
var assets embed.FS

// Handler returns an http.Handler that serves embedded portal assets with SPA routing fallback.
func Handler() http.Handler {
	distFS, err := fs.Sub(assets, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "portal assets not found in binary", http.StatusNotFound)
		})
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := strings.TrimPrefix(r.URL.Path, "/")
		if reqPath == "" {
			reqPath = "index.html"
		}

		// Check if file exists in embedded dist
		if f, err := distFS.Open(reqPath); err == nil {
			_ = f.Close()
			// Set proper content-type header based on file extension
			ext := filepath.Ext(reqPath)
			switch ext {
			case ".json", ".webmanifest":
				w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
			case ".js", ".mjs":
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			case ".css":
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case ".svg":
				w.Header().Set("Content-Type", "image/svg+xml")
			case ".png":
				w.Header().Set("Content-Type", "image/png")
			case ".ico":
				w.Header().Set("Content-Type", "image/x-icon")
			default:
				if mimeType := mime.TypeByExtension(ext); mimeType != "" {
					w.Header().Set("Content-Type", mimeType)
				}
			}
			if strings.HasPrefix(reqPath, "assets/") {
				// Cache immutable static assets for 1 year
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				// Prevent caching for index.html
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for Single Page Application client-side routing (e.g. /chat, /analyze, /admin)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
