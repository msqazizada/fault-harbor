package routing

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fixtureSource struct {
	id       SourceID
	snapshot Snapshot
	err      error
	called   bool
}

func (s *fixtureSource) Identity() SourceID { return s.id }

func (s *fixtureSource) Collect(context.Context) (Snapshot, error) {
	s.called = true
	return s.snapshot, s.err
}

func newFixture(provider, expression string) *fixtureSource {
	id := SourceID{Environment: "production", Provider: provider, Instance: "edge"}
	ref := Reference{Source: id, ID: "app"}
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	return &fixtureSource{id: id, snapshot: Snapshot{
		Source: id, ObservedAt: now, State: CollectionComplete,
		Routes: []Route{{
			Ref: ref, Protocol: "http",
			Matches:  []Match{{Dialect: provider, Expression: expression}},
			Backends: []Reference{{Source: id, ID: "missing-backend"}},
			Evidence: []Evidence{{Resource: ref, Locator: "routes/app", ObservedAt: now}},
		}},
	}}
}

func TestProviderNeutralCollectionPreservesSemantics(t *testing.T) {
	// These synthetic sources exercise the contract, not real provider APIs.
	first := newFixture("traefik", "Host(`app.example.com`)")
	second := newFixture("nginx", "server_name app.example.com")
	var refs []Reference
	for _, source := range []*fixtureSource{first, second} {
		snapshot, err := Collect(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}
		route := snapshot.Routes[0]
		refs = append(refs, route.Ref)
		if route.Matches[0] != source.snapshot.Routes[0].Matches[0] {
			t.Fatal("collection changed provider matching semantics")
		}
		if len(snapshot.Backends) != 0 || route.Backends[0].ID != "missing-backend" {
			t.Fatal("collection must preserve unresolved references without fabricating backends")
		}
	}
	if refs[0] == refs[1] {
		t.Fatal("identical native IDs from different providers must remain distinct")
	}
}

func TestCollectionRejectsUntrustworthySnapshots(t *testing.T) {
	for name, mutate := range map[string]func(*fixtureSource){
		"wrong source":           func(s *fixtureSource) { s.snapshot.Source.Instance = "another" },
		"unknown completeness":   func(s *fixtureSource) { s.snapshot.State = "" },
		"missing timestamp":      func(s *fixtureSource) { s.snapshot.ObservedAt = time.Time{} },
		"partial without reason": func(s *fixtureSource) { s.snapshot.State = CollectionPartial },
		"complete with failures": func(s *fixtureSource) {
			s.snapshot.Issues = []Issue{{Code: "permission_denied", Message: "Backend collection denied"}}
		},
		"missing evidence": func(s *fixtureSource) { s.snapshot.Routes[0].Evidence = nil },
		"duplicate resource": func(s *fixtureSource) {
			s.snapshot.Routes = append(s.snapshot.Routes, s.snapshot.Routes[0])
		},
		"cross environment reference": func(s *fixtureSource) {
			s.snapshot.Routes[0].Backends[0].Source.Environment = "unrelated"
		},
	} {
		t.Run(name, func(t *testing.T) {
			source := newFixture("fixture", "path /app")
			mutate(source)
			if _, err := Collect(context.Background(), source); err == nil {
				t.Fatal("accepted an invalid snapshot")
			}
		})
	}
}

func TestPartialCollectionRetainsEvidenceAndIssues(t *testing.T) {
	source := newFixture("fixture", "path /app")
	source.snapshot.State = CollectionPartial
	source.snapshot.Issues = []Issue{{Code: "permission_denied", Message: "Backend collection denied"}}
	snapshot, err := Collect(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != CollectionPartial || len(snapshot.Routes) != 1 || len(snapshot.Issues) != 1 {
		t.Fatal("partial collection lost its evidence or failure status")
	}
}

func TestSourceFailuresDoNotBecomeEmptySuccess(t *testing.T) {
	source := newFixture("fixture", "path /app")
	source.err = errors.New("provider unavailable")
	if _, err := Collect(context.Background(), source); !errors.Is(err, source.err) {
		t.Fatalf("provider error was lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source.called = false
	if _, err := Collect(ctx, source); !errors.Is(err, context.Canceled) || source.called {
		t.Fatalf("cancelled collection invoked the provider or lost cancellation: %v", err)
	}
}
