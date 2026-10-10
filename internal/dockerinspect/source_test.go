package dockerinspect

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

var containerID = strings.Repeat("a", 64)
var networkID = strings.Repeat("b", 64)

type fakeEngine struct {
	list       client.ContainerListResult
	listErr    error
	inspect    client.ContainerInspectResult
	inspectErr error
	networks   client.NetworkListResult
	networkErr error
}

func (f *fakeEngine) ContainerList(ctx context.Context, _ client.ContainerListOptions) (client.ContainerListResult, error) {
	if ctx.Err() != nil {
		return client.ContainerListResult{}, ctx.Err()
	}
	return f.list, f.listErr
}
func (f *fakeEngine) ContainerInspect(_ context.Context, _ string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return f.inspect, f.inspectErr
}
func (f *fakeEngine) NetworkList(_ context.Context, _ client.NetworkListOptions) (client.NetworkListResult, error) {
	return f.networks, f.networkErr
}
func fixture() (*Source, *fakeEngine) {
	f := &fakeEngine{list: client.ContainerListResult{Items: []container.Summary{{ID: containerID}}}, inspect: client.ContainerInspectResult{Container: container.InspectResponse{ID: containerID, Name: "/service", Image: "sha256:image", State: &container.State{Status: "running", Running: true}, Config: &container.Config{Env: []string{"PASSWORD=secret-marker"}, Labels: map[string]string{"token": "secret-marker"}}, NetworkSettings: &container.NetworkSettings{Networks: map[string]*network.EndpointSettings{"bridge": {NetworkID: networkID, IPAddress: netip.MustParseAddr("172.20.0.2")}}}}}, networks: client.NetworkListResult{Items: []network.Summary{{Network: network.Network{ID: networkID, Name: "bridge", Driver: "bridge", Labels: map[string]string{"token": "secret-marker"}}}}}}
	return &Source{cfg: Config{Environment: "production", Instance: "engine-1", Socket: "/docker.sock", Timeout: time.Second, RequestTimeout: time.Second, MaxContainers: 10, MaxNetworks: 10}, api: f}, f
}
func TestCollectStatesAndRedaction(t *testing.T) {
	for _, state := range []string{"running", "exited", "restarting", "paused"} {
		t.Run(state, func(t *testing.T) {
			s, f := fixture()
			f.inspect.Container.State.Status = container.ContainerState(state)
			f.inspect.Container.State.Restarting = state == "restarting"
			f.inspect.Container.RestartCount = 7
			f.inspect.Container.State.Health = &container.Health{Status: "unhealthy", FailingStreak: 3, Log: []*container.HealthcheckResult{{Output: "secret-marker"}}}
			f.inspect.Container.State.Error = "secret-marker"
			f.inspect.Container.Args = []string{"secret-marker"}
			snap, err := s.Collect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if snap.State != "complete" || len(snap.Containers) != 1 || len(snap.Networks) != 1 {
				t.Fatalf("unexpected snapshot: %+v", snap)
			}
			c := snap.Containers[0]
			if c.State != state || c.RestartCount != 7 || c.Health.Status != "unhealthy" || c.Networks[0].Address != "172.20.0.2" || c.Ref.Source != s.Identity() || c.ObservedAt.Before(snap.StartedAt) || c.ObservedAt.After(snap.ObservedAt) {
				t.Fatalf("unexpected observation: %+v", c)
			}
			data, err := json.Marshal(snap)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "secret-marker") {
				t.Fatal("secret escaped projection")
			}
		})
	}
}
func TestCollectionFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{{"permission", errdefs.ErrPermissionDenied, "permission_denied"}, {"unavailable", errors.New("sensitive runtime error"), "source_unavailable"}, {"deadline", context.DeadlineExceeded, "timeout"}, {"response size", errResponseLimit, "response_limit"}} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := fixture()
			f.listErr = tc.err
			snap, err := s.Collect(t.Context())
			var failure *Failure
			if !errors.As(err, &failure) || failure.Code != tc.code || snap.State != "" {
				t.Fatalf("got %+v, %v", snap, err)
			}
		})
	}
}
func TestPartialCollection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*Source, *fakeEngine)
		code  string
	}{
		{"disappearing container", func(_ *Source, f *fakeEngine) { f.inspectErr = errdefs.ErrNotFound }, "resource_disappeared"},
		{"networks denied", func(_ *Source, f *fakeEngine) { f.networkErr = errdefs.ErrPermissionDenied }, "permission_denied"},
		{"container limit", func(s *Source, f *fakeEngine) {
			s.cfg.MaxContainers = 1
			f.list.Items = append(f.list.Items, f.list.Items[0])
		}, "container_limit"},
		{"network limit", func(s *Source, f *fakeEngine) {
			s.cfg.MaxNetworks = 1
			f.networks.Items = append(f.networks.Items, f.networks.Items[0])
		}, "network_limit"},
		{"missing state", func(_ *Source, f *fakeEngine) { f.inspect.Container.State = nil }, "invalid_response"},
		{"invalid id", func(_ *Source, f *fakeEngine) { f.list.Items[0].ID = "../../secrets" }, "invalid_response"},
		{"oversized resource", func(_ *Source, f *fakeEngine) { f.inspect.Container.Name = strings.Repeat("a", 20<<10) }, "resource_limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := fixture()
			tc.setup(s, f)
			snap, err := s.Collect(t.Context())
			if err != nil || snap.State != "partial" || len(snap.Issues) == 0 || snap.Issues[0].Code != tc.code {
				t.Fatalf("got %+v, %v", snap, err)
			}
		})
	}
}
func TestCancellationAndNoHealthCheck(t *testing.T) {
	s, f := fixture()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := s.Collect(ctx)
	var failure *Failure
	if !errors.As(err, &failure) || failure.Code != "cancelled" {
		t.Fatalf("got %v", err)
	}
	snap, err := s.Collect(t.Context())
	if err != nil || snap.Containers[0].Health != nil {
		t.Fatalf("got %+v %v", snap, err)
	}
	f.list.Items = nil
	f.networks.Items = nil
	snap, err = s.Collect(t.Context())
	if err != nil || snap.State != "complete" || len(snap.Containers) != 0 {
		t.Fatalf("empty inventory: %+v %v", snap, err)
	}
}
func TestConfiguration(t *testing.T) {
	s, _ := fixture()
	if err := s.cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){func(c *Config) { c.Environment = "" }, func(c *Config) { c.Instance = "bad/path" }, func(c *Config) { c.Socket = "relative" }, func(c *Config) { c.Timeout = 2 * time.Minute }, func(c *Config) { c.RequestTimeout = 2 * time.Second }, func(c *Config) { c.MaxContainers = 0 }, func(c *Config) { c.MaxNetworks = 1001 }} {
		cfg := s.cfg
		mutate(&cfg)
		if cfg.Validate() == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
