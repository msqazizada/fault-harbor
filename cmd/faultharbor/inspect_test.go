package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msqazizada/fault-harbor/internal/inventory"
	"github.com/msqazizada/fault-harbor/internal/resource"
)

func TestInspectExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		state  string
		want   int
	}{
		{"complete", 200, "complete", 0}, {"partial", 200, "partial", 2}, {"failed", 502, "", 1}, {"invalid snapshot", 200, "unknown", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "fh-cli-")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.RemoveAll(dir) }()
			socket := filepath.Join(dir, "s")
			listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				if tc.status != 200 {
					_ = json.NewEncoder(w).Encode(inventory.Failure{Code: "permission_denied"})
					return
				}
				now := time.Now().UTC()
				snap := inventory.Snapshot{SchemaVersion: 1, Source: resource.SourceID{Environment: "test", Provider: "docker", Instance: "engine"}, StartedAt: now, ObservedAt: now, State: tc.state}
				if tc.state == "partial" {
					snap.Issues = []inventory.Issue{{Code: "timeout", Message: "Inspection timed out."}}
				}
				_ = json.NewEncoder(w).Encode(snap)
			})}
			defer func() { _ = srv.Close() }()
			go func() {
				if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					t.Error(err)
				}
			}()
			var out, errOut bytes.Buffer
			code := inspect(t.Context(), []string{"inspect", "docker", "--socket", socket}, &out, &errOut)
			if code != tc.want {
				t.Fatalf("exit %d, want %d; stderr=%s", code, tc.want, errOut.String())
			}
			if code == 1 {
				if out.Len() != 0 || errOut.Len() == 0 {
					t.Fatal("failure must write only to stderr")
				}
			} else {
				var snap inventory.Snapshot
				if err = json.Unmarshal(out.Bytes(), &snap); err != nil {
					t.Fatal(err)
				}
				if snap.State != tc.state || errOut.Len() != 0 {
					t.Fatal("invalid CLI output")
				}
			}
			if tc.status == 502 && !strings.Contains(errOut.String(), "permission_denied") {
				t.Fatal("permission error lost")
			}
		})
	}
}
func TestInspectArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"inspect"}, {"inspect", "other"}, {"inspect", "docker", "--unknown"}, {"inspect", "docker", "extra"}, {"inspect", "docker", "--socket", "relative"}} {
		var out, errOut bytes.Buffer
		if inspect(t.Context(), args, &out, &errOut) != 1 || out.Len() != 0 || errOut.Len() == 0 {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
}
