// Package dockerinspect implements inventory collection through Docker's SDK.
// Only the collector executable imports this package.
package dockerinspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/msqazizada/fault-harbor/internal/inventory"
	"github.com/msqazizada/fault-harbor/internal/resource"
)

// MaxResponseBytes bounds each upstream response before decoding.
const MaxResponseBytes = 2 << 20

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
var nativeID = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Config identifies an Engine and bounds the work permitted per collection.
type Config struct {
	Environment    string
	Instance       string
	Socket         string
	Timeout        time.Duration
	RequestTimeout time.Duration
	MaxContainers  int
	MaxNetworks    int
}

func (c Config) Validate() error {
	if !identifier.MatchString(c.Environment) || !identifier.MatchString(c.Instance) {
		return errors.New("environment and source must be 1–64 letters, digits, dots, underscores or hyphens, starting with a letter or digit")
	}
	if !filepath.IsAbs(c.Socket) {
		return errors.New("docker socket must be an absolute path")
	}
	if c.Timeout <= 0 || c.Timeout > time.Minute || c.RequestTimeout <= 0 || c.RequestTimeout > c.Timeout {
		return errors.New("timeouts must be positive; request timeout must not exceed collection timeout (maximum 1 minute)")
	}
	if c.MaxContainers < 1 || c.MaxContainers > 1000 || c.MaxNetworks < 1 || c.MaxNetworks > 1000 {
		return errors.New("resource limits must be between 1 and 1000")
	}
	return nil
}

// Small consumer-owned interfaces make failure cases testable without a daemon.
type engine interface {
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	NetworkList(context.Context, client.NetworkListOptions) (client.NetworkListResult, error)
}

// Source owns its SDK client and connection pool; Close releases idle sockets.
type Source struct {
	cfg   Config
	api   engine
	close func() error
}

func New(cfg Config) (*Source, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", cfg.Socket)
	}, MaxConnsPerHost: 1, MaxIdleConns: 1, IdleConnTimeout: 30 * time.Second}
	httpClient := &http.Client{Transport: guardedTransport{base: transport, maxBytes: MaxResponseBytes}, Timeout: cfg.RequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errForbidden }}
	api, err := client.New(client.WithHost("unix://"+cfg.Socket), client.WithHTTPClient(httpClient))
	if err != nil {
		transport.CloseIdleConnections()
		return nil, fmt.Errorf("create Docker client: %w", err)
	}
	return &Source{cfg: cfg, api: api, close: func() error { transport.CloseIdleConnections(); return api.Close() }}, nil
}

func (s *Source) Close() error { return s.close() }
func (s *Source) Identity() resource.SourceID {
	return resource.SourceID{Environment: s.cfg.Environment, Provider: "docker", Instance: s.cfg.Instance}
}

// Failure reports a safe code. Raw daemon errors can contain sensitive content.
type Failure = inventory.Failure

func failureCode(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, errResponseLimit):
		return "response_limit"
	case errdefs.IsPermissionDenied(err), errdefs.IsUnauthorized(err), errors.Is(err, os.ErrPermission):
		return "permission_denied"
	case errdefs.IsNotFound(err):
		return "resource_disappeared"
	default:
		return "source_unavailable"
	}
}

func (s *Source) Collect(ctx context.Context) (inventory.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	snap := inventory.Snapshot{
		SchemaVersion: 1,
		Source:        s.Identity(),
		StartedAt:     time.Now().UTC(),
		State:         "complete",
		Containers:    []inventory.Container{},
		Networks:      []inventory.Network{},
		Issues:        []inventory.Issue{},
	}
	list, err := s.api.ContainerList(ctx, client.ContainerListOptions{All: true, Limit: s.cfg.MaxContainers + 1})
	if err != nil {
		return inventory.Snapshot{}, &Failure{Code: failureCode(err)}
	}
	issue := func(code, message string, ref *resource.Reference) {
		snap.State = "partial"
		snap.Issues = append(snap.Issues, inventory.Issue{Code: code, Message: message, Resource: ref})
	}
	if len(list.Items) > s.cfg.MaxContainers {
		issue("container_limit", "Container limit reached; inventory is incomplete.", nil)
		list.Items = list.Items[:s.cfg.MaxContainers]
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].ID < list.Items[j].ID })
	for _, item := range list.Items {
		ref := resource.Reference{Source: s.Identity(), ID: item.ID}
		if !nativeID.MatchString(item.ID) {
			issue("invalid_response", "Container identifier is invalid.", nil)
			continue
		}
		result, inspectErr := s.api.ContainerInspect(ctx, item.ID, client.ContainerInspectOptions{})
		if inspectErr != nil {
			issue(failureCode(inspectErr), "Container inspection did not complete.", &ref)
			if ctx.Err() != nil {
				break
			}
			continue
		}
		if result.Container.ID != item.ID || result.Container.State == nil {
			issue("invalid_response", "Container identity or state is missing.", &ref)
			continue
		}
		projected := projectContainer(s.Identity(), result.Container)
		data, marshalErr := json.Marshal(projected)
		if marshalErr != nil || len(data) > 16<<10 {
			issue("resource_limit", "Container observation exceeds size limit.", &ref)
			continue
		}
		snap.Containers = append(snap.Containers, projected)
	}
	networks, err := s.api.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		issue(failureCode(err), "Network inspection did not complete.", nil)
	} else {
		if len(networks.Items) > s.cfg.MaxNetworks {
			issue("network_limit", "Network limit reached; inventory is incomplete.", nil)
			networks.Items = networks.Items[:s.cfg.MaxNetworks]
		}
		for _, n := range networks.Items {
			if len(n.Name) > 1024 || len(n.Driver) > 256 || len(n.Scope) > 256 || !nativeID.MatchString(n.ID) {
				issue("invalid_response", "Network identity or metadata is invalid.", nil)
				continue
			}
			snap.Networks = append(snap.Networks, inventory.Network{Ref: resource.Reference{Source: s.Identity(), ID: n.ID}, ObservedAt: time.Now().UTC(), Name: n.Name, Driver: n.Driver, Scope: n.Scope, Internal: n.Internal})
		}
	}
	sort.Slice(snap.Networks, func(i, j int) bool { return snap.Networks[i].Ref.ID < snap.Networks[j].Ref.ID })
	snap.ObservedAt = time.Now().UTC()
	return snap, nil
}

func projectContainer(source resource.SourceID, c container.InspectResponse) inventory.Container {
	item := inventory.Container{
		Ref:          resource.Reference{Source: source, ID: c.ID},
		ObservedAt:   time.Now().UTC(),
		Name:         strings.TrimPrefix(c.Name, "/"),
		Image:        c.Image,
		State:        string(c.State.Status),
		Running:      c.State.Running,
		Restarting:   c.State.Restarting,
		OOMKilled:    c.State.OOMKilled,
		ExitCode:     c.State.ExitCode,
		RestartCount: c.RestartCount,
		Networks:     []inventory.Attachment{},
	}
	if c.State.Health != nil {
		item.Health = &inventory.Health{Status: string(c.State.Health.Status), FailingStreak: c.State.Health.FailingStreak}
	}
	if c.NetworkSettings != nil {
		for _, n := range c.NetworkSettings.Networks {
			if n == nil || n.NetworkID == "" {
				continue
			}
			a := inventory.Attachment{Network: resource.Reference{Source: source, ID: n.NetworkID}}
			if n.IPAddress.IsValid() {
				a.Address = n.IPAddress.String()
			}
			if n.GlobalIPv6Address.IsValid() {
				a.IPv6Address = n.GlobalIPv6Address.String()
			}
			item.Networks = append(item.Networks, a)
		}
	}
	sort.Slice(item.Networks, func(i, j int) bool { return item.Networks[i].Network.ID < item.Networks[j].Network.ID })
	return item
}
