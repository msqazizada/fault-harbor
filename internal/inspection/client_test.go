package inspection

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/msqazizada/fault-harbor/internal/inventory"
	"github.com/msqazizada/fault-harbor/internal/resource"
)

func TestFetchOverUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "fh-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	socket := filepath.Join(dir, "test.sock")
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	valid := inventory.Snapshot{SchemaVersion: 1, Source: resource.SourceID{Environment: "test", Provider: "docker", Instance: "engine"}, StartedAt: now, ObservedAt: now, State: "complete"}
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/snapshot" {
			t.Error("unexpected request")
		}
		_ = json.NewEncoder(w).Encode(valid)
	})}
	defer func() { _ = srv.Close() }()
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()
	got, err := Fetch(t.Context(), socket)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != valid.Source || got.State != "complete" {
		t.Fatalf("got %+v", got)
	}
}
func TestFetchInvalidSocket(t *testing.T) {
	if _, err := Fetch(t.Context(), "relative"); err == nil {
		t.Fatal("accepted relative socket")
	}
}
