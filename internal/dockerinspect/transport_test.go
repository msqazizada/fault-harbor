package dockerinspect

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }
func TestTransportAllowlist(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		allowed      bool
	}{
		{"GET", "/v1.51/containers/json?all=1", true}, {"GET", "/v1.51/containers/" + containerID + "/json", true}, {"GET", "/v1.51/networks", true}, {"HEAD", "/_ping", true}, {"GET", "/_ping", true},
		{"POST", "/v1.51/containers/" + containerID + "/stop", false}, {"DELETE", "/v1.51/containers/" + containerID, false}, {"GET", "/v1.51/containers/" + containerID + "/logs", false}, {"GET", "/v1.51/containers/" + containerID + "/archive", false}, {"GET", "/version", false}, {"GET", "/v1.51/containers/name/json", false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			called := false
			body := &trackedBody{Reader: strings.NewReader("ok")}
			tr := guardedTransport{maxBytes: 10, base: roundTripFunc(func(*http.Request) (*http.Response, error) {
				called = true
				return &http.Response{StatusCode: 200, Body: body}, nil
			})}
			req, err := http.NewRequestWithContext(t.Context(), tc.method, "http://docker"+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := tr.RoundTrip(req)
			if called != tc.allowed {
				t.Fatalf("upstream called=%v", called)
			}
			if !tc.allowed {
				if !errors.Is(err, errForbidden) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			data, err := io.ReadAll(resp.Body)
			if err != nil || string(data) != "ok" || !body.closed {
				t.Fatalf("body not preserved or closed: %q %v", data, err)
			}
		})
	}
}
func TestTransportResponseLimit(t *testing.T) {
	body := &trackedBody{Reader: strings.NewReader("123456")}
	tr := guardedTransport{maxBytes: 5, base: roundTripFunc(func(*http.Request) (*http.Response, error) { return &http.Response{Body: body}, nil })}
	req, err := http.NewRequestWithContext(t.Context(), "GET", "http://docker/v1.51/networks", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, errResponseLimit) || !body.closed {
		t.Fatalf("got %v closed=%v", err, body.closed)
	}
}
