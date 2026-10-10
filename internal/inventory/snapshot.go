// Package inventory defines container and network observations, without depending
// on Docker or routing. Observations are evidence, not diagnostic conclusions.
package inventory

import (
	"context"
	"time"

	"github.com/msqazizada/fault-harbor/internal/resource"
)

// Source returns a partial snapshot with issues, or an error when none is usable.
type Source interface {
	Identity() resource.SourceID
	Collect(context.Context) (Snapshot, error)
}

type Snapshot struct {
	SchemaVersion int               `json:"schemaVersion"`
	Source        resource.SourceID `json:"source"`
	StartedAt     time.Time         `json:"startedAt"`
	ObservedAt    time.Time         `json:"observedAt"`
	State         string            `json:"state"`
	Containers    []Container       `json:"containers"`
	Networks      []Network         `json:"networks"`
	Issues        []Issue           `json:"issues"`
}

type Issue struct {
	Code     string              `json:"code"`
	Message  string              `json:"message"`
	Resource *resource.Reference `json:"resource,omitempty"`
}

// Container deliberately excludes environment, commands, mounts, labels and logs.
// RestartCount is a counter observed once; it cannot establish a restart loop.
type Container struct {
	Ref          resource.Reference `json:"ref"`
	ObservedAt   time.Time          `json:"observedAt"`
	Name         string             `json:"name"`
	Image        string             `json:"image"`
	State        string             `json:"state"`
	Running      bool               `json:"running"`
	Restarting   bool               `json:"restarting"`
	OOMKilled    bool               `json:"oomKilled"`
	ExitCode     int                `json:"exitCode"`
	RestartCount int                `json:"restartCount"`
	Health       *Health            `json:"health,omitempty"`
	Networks     []Attachment       `json:"networks"`
}

// A nil health observation means no health check was reported, not healthy.
type Health struct {
	Status        string `json:"status"`
	FailingStreak int    `json:"failingStreak"`
}

type Attachment struct {
	Network     resource.Reference `json:"network"`
	Address     string             `json:"address,omitempty"`
	IPv6Address string             `json:"ipv6Address,omitempty"`
}

type Network struct {
	Ref        resource.Reference `json:"ref"`
	ObservedAt time.Time          `json:"observedAt"`
	Name       string             `json:"name"`
	Driver     string             `json:"driver"`
	Scope      string             `json:"scope"`
	Internal   bool               `json:"internal"`
}

// Failure contains a public collection error code, never a raw provider error.
type Failure struct {
	Code string `json:"error"`
}

func (f *Failure) Error() string { return "collection failed: " + f.Code }
