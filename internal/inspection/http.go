// Package inspection is the narrow transport between a collector and consumers.
// It depends on inventory contracts, never on a provider SDK or Docker socket.
package inspection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/msqazizada/fault-harbor/internal/inventory"
)

const MaxSnapshotBytes = 8 << 20

// NewHandler permits one collection at a time. A channel is a semaphore here:
// a non-blocking send acquires the slot; receiving in defer releases it.
func NewHandler(source inventory.Source) http.Handler {
	slot := make(chan struct{}, 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeFailure(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if r.URL.RawQuery != "" || r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			writeFailure(w, http.StatusBadRequest, "invalid_request")
			return
		}
		switch r.URL.Path {
		case "/health/live":
			_, _ = io.WriteString(w, `{"status":"ok"}`)
			return
		case "/v1/snapshot":
		default:
			writeFailure(w, http.StatusNotFound, "not_found")
			return
		}
		select {
		case slot <- struct{}{}:
			defer func() { <-slot }()
		default:
			w.Header().Set("Retry-After", "1")
			writeFailure(w, http.StatusServiceUnavailable, "collector_busy")
			return
		}
		snap, err := source.Collect(r.Context())
		if err != nil {
			failure := &inventory.Failure{Code: "collection_failed"}
			var known *inventory.Failure
			if errors.As(err, &known) {
				failure = known
			}
			writeFailure(w, http.StatusBadGateway, failure.Code)
			return
		}
		data, err := json.Marshal(snap)
		if err != nil || len(data) > MaxSnapshotBytes {
			writeFailure(w, http.StatusBadGateway, "snapshot_limit")
			return
		}
		_, _ = w.Write(data)
	})
}

// writeFailure preserves the same JSON response contract for every error path.
func writeFailure(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(inventory.Failure{Code: code})
}

// Fetch speaks only to a configured Unix socket. HTTP_PROXY and redirects cannot
// redirect observations elsewhere. Socket permissions are the access boundary.
func Fetch(ctx context.Context, socket string) (inventory.Snapshot, error) {
	if !filepath.IsAbs(socket) {
		return inventory.Snapshot{}, errors.New("collector socket must be an absolute path")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	cli := &http.Client{Transport: transport, Timeout: 65 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("collector redirects are forbidden") }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://collector/v1/snapshot", nil)
	if err != nil {
		return inventory.Snapshot{}, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return inventory.Snapshot{}, fmt.Errorf("contact collector: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxSnapshotBytes+1))
	if err != nil {
		return inventory.Snapshot{}, fmt.Errorf("read collector: %w", err)
	}
	if len(data) > MaxSnapshotBytes {
		return inventory.Snapshot{}, errors.New("collector response exceeds size limit")
	}
	if resp.StatusCode != http.StatusOK {
		var failure inventory.Failure
		if json.Unmarshal(data, &failure) == nil && len(failure.Code) > 0 && len(failure.Code) <= 64 {
			return inventory.Snapshot{}, &failure
		}
		return inventory.Snapshot{}, fmt.Errorf("collector returned HTTP %d", resp.StatusCode)
	}
	var snap inventory.Snapshot
	if err = json.Unmarshal(data, &snap); err != nil {
		return inventory.Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	validState := snap.State == "complete" || snap.State == "partial"
	validSource := snap.Source.Environment != "" && snap.Source.Instance != "" && snap.Source.Provider != ""
	validTimes := !snap.ObservedAt.IsZero() && !snap.StartedAt.IsZero() && !snap.ObservedAt.Before(snap.StartedAt)
	consistentIssues := (snap.State == "partial") == (len(snap.Issues) > 0)
	if snap.SchemaVersion != 1 || !validState || !validSource || !validTimes || !consistentIssues {
		return inventory.Snapshot{}, errors.New("invalid collector snapshot")
	}
	return snap, nil
}
