package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHealthAndVersion(t *testing.T) {
	handler := NewHandler("test-version", fstest.MapFS{})
	for _, test := range []struct {
		path, key, value string
	}{
		{"/health/live", "status", "ok"},
		{"/health/ready", "status", "ok"},
		{"/api/v1/version", "version", "test-version"},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body[test.key] != test.value {
				t.Fatalf("body = %v, want %s=%s", body, test.key, test.value)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("health and version responses must not be cached")
			}
		})
	}
}

func TestStaticAndAPIBoundaries(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>FaultHarbor</title>")},
		"assets/app.js": {Data: []byte("export {};")},
	}
	for _, test := range []struct {
		name, method, path string
		files              fstest.MapFS
		status             int
	}{
		{"index", "GET", "/", assets, 200},
		{"asset", "GET", "/assets/app.js", assets, 200},
		{"missing asset", "GET", "/assets/missing.js", assets, 404},
		{"no directory listing", "GET", "/assets/", assets, 404},
		{"unknown API", "GET", "/api/v1/missing", assets, 404},
		{"unknown health", "GET", "/health/missing", assets, 404},
		{"method", "POST", "/api/v1/version", assets, 405},
		{"unbuilt frontend", "GET", "/", fstest.MapFS{}, 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			NewHandler("dev", test.files).ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if strings.HasPrefix(test.path, "/api/") && strings.Contains(response.Body.String(), "<!doctype") {
				t.Fatal("API request returned frontend HTML")
			}
		})
	}
}
