package routing

import (
	"context"
	"fmt"
)

// Collect checks the provider boundary before a snapshot reaches diagnostics.
// A missing backend reference is valid evidence for diagnostics to investigate;
// it must not be discarded or turned into a fabricated backend by the adapter.
func Collect(ctx context.Context, source Source) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	identity := source.Identity()
	if !validSource(identity) {
		return Snapshot{}, fmt.Errorf("routing source requires environment, provider, and instance")
	}
	snapshot, err := source.Collect(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("collect routing source %s: %w", identity.Instance, err)
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Source != identity || snapshot.ObservedAt.IsZero() {
		return Snapshot{}, fmt.Errorf("routing snapshot requires matching source identity and observation time")
	}
	if snapshot.State != CollectionComplete && snapshot.State != CollectionPartial {
		return Snapshot{}, fmt.Errorf("routing snapshot requires an explicit collection state")
	}
	if (snapshot.State == CollectionPartial) != (len(snapshot.Issues) > 0) {
		return Snapshot{}, fmt.Errorf("partial routing snapshots require issues; complete snapshots must have none")
	}
	for _, issue := range snapshot.Issues {
		if issue.Code == "" || issue.Message == "" {
			return Snapshot{}, fmt.Errorf("collection issues require a code and message")
		}
		if issue.Resource != nil && !validReference(*issue.Resource, identity.Environment) {
			return Snapshot{}, fmt.Errorf("collection issue has an invalid resource reference")
		}
	}
	routes, backends := make(map[Reference]bool), make(map[Reference]bool)
	for _, route := range snapshot.Routes {
		if err := validateResource(route.Ref, route.Evidence, identity, routes); err != nil {
			return Snapshot{}, err
		}
		for _, backend := range route.Backends {
			if !validReference(backend, identity.Environment) {
				return Snapshot{}, fmt.Errorf("route %s has an invalid backend reference", route.Ref.ID)
			}
		}
	}
	for _, backend := range snapshot.Backends {
		if err := validateResource(backend.Ref, backend.Evidence, identity, backends); err != nil {
			return Snapshot{}, err
		}
	}
	return snapshot, nil
}

func validateResource(ref Reference, evidence []Evidence, source SourceID, seen map[Reference]bool) error {
	if ref.Source != source || ref.ID == "" || seen[ref] {
		return fmt.Errorf("routing resource requires a unique ID owned by the collecting source")
	}
	seen[ref] = true
	if len(evidence) == 0 {
		return fmt.Errorf("routing resource %s requires collection evidence", ref.ID)
	}
	for _, item := range evidence {
		if item.Resource != ref || item.Locator == "" || item.ObservedAt.IsZero() {
			return fmt.Errorf("routing resource %s has invalid collection evidence", ref.ID)
		}
	}
	return nil
}

func validSource(source SourceID) bool {
	return source.Environment != "" && source.Provider != "" && source.Instance != ""
}

func validReference(ref Reference, environment string) bool {
	return validSource(ref.Source) && ref.Source.Environment == environment && ref.ID != ""
}
