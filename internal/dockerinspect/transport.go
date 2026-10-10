package dockerinspect

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"regexp"
)

var (
	errResponseLimit = errors.New("docker response exceeds size limit")
	errForbidden     = errors.New("docker operation is not allowlisted")
	readPath         = regexp.MustCompile(`^/v[0-9]+\.[0-9]+/(containers/json|containers/[a-f0-9]{64}/json|networks)$`)
)

// guardedTransport is defense in depth: even an accidental SDK mutation cannot
// reach the socket. The public collector API never forwards raw Docker requests.
type guardedTransport struct {
	base     http.RoundTripper
	maxBytes int64
}

func (t guardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ping := req.URL.Path == "/_ping" && (req.Method == http.MethodHead || req.Method == http.MethodGet)
	read := req.Method == http.MethodGet && readPath.MatchString(req.URL.Path)
	if !ping && !read {
		return nil, errForbidden
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	// Bound bytes before SDK JSON decoding, which otherwise trusts daemon input.
	body := resp.Body
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, t.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > t.maxBytes {
		return nil, errResponseLimit
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	return resp, nil
}
