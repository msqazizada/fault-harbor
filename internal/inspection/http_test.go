package inspection

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/msqazizada/fault-harbor/internal/inventory"
	"github.com/msqazizada/fault-harbor/internal/resource"
)

type fakeSource struct {
	collect func(context.Context) (inventory.Snapshot, error)
}

func (f fakeSource) Identity() resource.SourceID {
	return resource.SourceID{Environment: "test", Provider: "docker", Instance: "engine"}
}
func (f fakeSource) Collect(ctx context.Context) (inventory.Snapshot, error) { return f.collect(ctx) }
func TestRestrictedAPI(t *testing.T) {
	called := 0
	handler := NewHandler(fakeSource{collect: func(context.Context) (inventory.Snapshot, error) {
		called++
		return inventory.Snapshot{State: "complete"}, nil
	}})
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/health/live", 200}, {"GET", "/v1/snapshot", 200}, {"POST", "/v1/snapshot", 405}, {"GET", "/v1/snapshot?target=other", 400}, {"GET", "/containers/json", 404}, {"GET", "/v1.51/containers/id/archive", 404}} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil))
			if w.Header().Get("Content-Type") != "application/json" || !json.Valid(w.Body.Bytes()) {
				t.Fatalf("expected JSON response, got %q: %s", w.Header().Get("Content-Type"), w.Body.String())
			}
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
	if called != 1 {
		t.Fatalf("collections=%d", called)
	}
}
func TestSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{{errors.New("secret-marker"), "collection_failed"}, {&inventory.Failure{Code: "permission_denied"}, "permission_denied"}} {
		handler := NewHandler(fakeSource{collect: func(context.Context) (inventory.Snapshot, error) { return inventory.Snapshot{}, tc.err }})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), "GET", "/v1/snapshot", nil))
		if w.Code != 502 || strings.Contains(w.Body.String(), "secret-marker") || !strings.Contains(w.Body.String(), tc.code) {
			t.Fatalf("got %d %s", w.Code, w.Body.String())
		}
	}
}
func TestConcurrentCollectionRejected(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	handler := NewHandler(fakeSource{collect: func(ctx context.Context) (inventory.Snapshot, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
		}
		return inventory.Snapshot{}, nil
	}})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), "GET", "/v1/snapshot", nil))
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("collection did not start")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), "GET", "/v1/snapshot", nil))
	close(release)
	<-done
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d", w.Code)
	}
}
func TestSnapshotSizeBound(t *testing.T) {
	handler := NewHandler(fakeSource{collect: func(context.Context) (inventory.Snapshot, error) {
		return inventory.Snapshot{Containers: []inventory.Container{{Name: strings.Repeat("a", MaxSnapshotBytes)}}}, nil
	}})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), "GET", "/v1/snapshot", nil))
	if w.Code != 502 {
		t.Fatalf("got %d", w.Code)
	}
}
func TestPartialSnapshotPreserved(t *testing.T) {
	handler := NewHandler(fakeSource{collect: func(context.Context) (inventory.Snapshot, error) {
		return inventory.Snapshot{State: "partial", Issues: []inventory.Issue{{Code: "timeout", Message: "Inspection timed out."}}}, nil
	}})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), "GET", "/v1/snapshot", nil))
	var got inventory.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || got.State != "partial" || len(got.Issues) != 1 {
		t.Fatalf("got %+v", got)
	}
}
