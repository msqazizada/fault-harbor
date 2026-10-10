// Package routing defines the evidence exchanged between routing providers and
// diagnostics. It has no dependency on a provider SDK or deployment mechanism.
package routing

import (
	"context"
	"time"

	"github.com/msqazizada/fault-harbor/internal/resource"
)

// Source is implemented by routing adapters. Collect must honor cancellation.
// Partial results carry collection issues; an error means no usable snapshot.
// Implementations only inspect configuration and status, never modify them.
// Go satisfies interfaces implicitly: an adapter needs these methods, with no
// inheritance or implements declaration. The context carries cancellation and
// deadlines through the adapter's outbound requests.
type Source interface {
	Identity() SourceID
	Collect(context.Context) (Snapshot, error)
}

// SourceID identifies one configured provider instance in one environment.
// Provider names are identifiers, not a closed enumeration.
type SourceID = resource.SourceID

// Reference identifies a resource within its configured source.
type Reference = resource.Reference

// CollectionState is a distinct string type. Constants provide known values;
// Collect still validates it because arbitrary strings can be converted to it.
type CollectionState string

const (
	CollectionComplete CollectionState = "complete"
	CollectionPartial  CollectionState = "partial"
)

// Snapshot contains one collection result. Its slice fields hold variable-length
// inventories; an empty inventory alone says nothing about collection success.
type Snapshot struct {
	Source     SourceID        `json:"source"`
	ObservedAt time.Time       `json:"observedAt"`
	State      CollectionState `json:"state"`
	Routes     []Route         `json:"routes"`
	Backends   []Backend       `json:"backends"`
	Issues     []Issue         `json:"issues"`
}

// Evidence preserves where an observation came from. Locator is a sanitized
// provider object or field path, never credentials or a raw configuration dump.
type Evidence struct {
	Resource   Reference `json:"resource"`
	Locator    string    `json:"locator"`
	ObservedAt time.Time `json:"observedAt"`
}

// Issue records a collection limitation. A nil Resource pointer means the issue
// applies to the source as a whole; omitempty leaves that field out of JSON.
type Issue struct {
	Code     string     `json:"code"`
	Message  string     `json:"message"`
	Resource *Reference `json:"resource,omitempty"`
}

// Route links provider match expressions to zero or more backend references.
type Route struct {
	Ref      Reference   `json:"ref"`
	Name     string      `json:"name"`
	Protocol string      `json:"protocol"`
	Matches  []Match     `json:"matches"`
	Backends []Reference `json:"backends"`
	Evidence []Evidence  `json:"evidence"`
}

// Match retains provider syntax rather than pretending all routing engines
// have equivalent matching, priority, or rewrite semantics.
type Match struct {
	Dialect    string `json:"dialect"`
	Expression string `json:"expression"`
}

// Backend groups configured destinations and their collection evidence.
type Backend struct {
	Ref       Reference  `json:"ref"`
	Name      string     `json:"name"`
	Endpoints []Endpoint `json:"endpoints"`
	Evidence  []Evidence `json:"evidence"`
}

// Endpoint describes a configured destination, not verified reachability.
// Address can be a hostname, IP address, or Unix socket path. Protocol preserves
// the provider's transport identifier; unknown values must not imply HTTP.
type Endpoint struct {
	Address  string `json:"address"`
	Port     uint16 `json:"port,omitempty"`
	Protocol string `json:"protocol"`
}
