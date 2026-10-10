// Package server exposes the HTTP API and serves the built web interface.
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
)

// NewHandler serves the API and the built frontend from the same origin.
// Accepting fs.FS instead of a disk path lets tests supply an in-memory filesystem.
func NewHandler(version string, assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	health := func(w http.ResponseWriter, r *http.Request) {
		// JSON tags set wire-format names independently of Go's exported fields.
		writeJSON(w, http.StatusOK, struct {
			Status string `json:"status"`
		}{Status: "ok"})
	}
	// ServeMux selects handlers by method and path; GET also accepts HEAD.
	mux.HandleFunc("GET /health/live", health)
	// Readiness covers this server only, independently of collector availability.
	mux.HandleFunc("GET /health/ready", health)
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Version string `json:"version"`
		}{Version: version})
	})
	// Keep unknown API paths out of the frontend handler: clients expect a
	// structured error here, not an HTML page that looks like a successful fetch.
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
	})
	mux.HandleFunc("GET /health/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	files := http.FileServerFS(assets)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		info, err := fs.Stat(assets, name)
		if err != nil {
			if r.URL.Path == "/" {
				http.Error(w, "Frontend unavailable. Run npm ci and npm run build in web/.", http.StatusServiceUnavailable)
				return
			}
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			// FileServer can list directories; only serve known files instead.
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})

	// HandlerFunc adapts a function to the http.Handler interface. Wrapping the
	// mux applies these headers to API, frontend, and error responses alike.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
