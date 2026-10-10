# Architecture

FaultHarbor separates collection, correlation, and diagnosis. Deployment location
and infrastructure provider are independent concerns. An environment identifies
an infrastructure scope; it does not imply localhost, Docker, or one machine.

## Integration boundary

Routing adapters implement `routing.Source`. They translate provider responses
into routes, backends, endpoints, and collection evidence. The shared collection
boundary validates identity, timestamps, completeness, and evidence before data
can enter diagnostics. These contracts exist in `internal/routing`; adapters and
diagnostic execution are not wired into the HTTP server yet.

Traefik is the first planned routing adapter. Another provider implements the
same interface without changing diagnostic rules or the frontend. Provider SDKs,
authentication, API versions, and configuration parsing belong inside adapters.
Container and network collection uses the separate `inventory.Source` contract;
routing providers should not need to implement Docker operations.

## Identity and evidence

Resource references contain the environment, provider, configured source instance,
and native resource ID. Display names are not identifiers. Route and backend IDs
are unique within their resource type and source. Cross-source relationships
require explicit references within an environment; matching names alone is not
evidence of a relationship. Cross-environment links need a separate explicit
contract before they can be correlated.

Snapshots carry observation times and an explicit complete or partial state.
Partial collection includes structured issues. An adapter error means no usable
snapshot; it must never become an empty, healthy inventory. Resource evidence
records a sanitized source locator and observation time. Missing backend
references are preserved so diagnostics can evaluate them against collection
completeness.

## Routing semantics

A route can reference multiple backends. A backend can have multiple endpoints
and need not be a container. HTTP, TCP, UDP, and other provider protocols remain
explicit; unknown protocol values are not treated as HTTP.

Provider match expressions retain their dialect. Generic rules can inspect
relationships without interpreting a Traefik rule or assuming an NGINX location
has the same semantics. Rules that need provider-specific matching must declare
that capability and decline unsupported dialects. Unrepresented features must
produce a partial snapshot and an issue instead of silently losing information.

Configured destinations do not establish network reachability. Active probes
need a separately authorized capability, with their origin and observation time
recorded. A probe from one network does not prove connectivity from another.

## Diagnostic flow

`Adapters → validated snapshots → correlated resources → rules → findings → API/UI`

Rules consume shared evidence and report confirmed findings, supported hypotheses,
or missing information. Correlation preserves ambiguity and timestamps. Adapters
do not produce UI copy or decide root causes. Future storage persists these
contracts; it does not become a dependency of provider integrations.

The current server binds to loopback as a secure deployment default. Remote
collection requires explicit endpoint configuration, scoped credentials, TLS,
timeouts, and authentication at the collection boundary. Those capabilities must
be implemented before claiming remote collection support.

## Docker inspection boundary

`Docker Engine → collector → Unix socket → inspection CLI`

The collector is a separate executable. Its Docker transport permits only API
negotiation, container listing/inspection, and network listing. It projects
provider responses into `inventory.Snapshot` before serving them. Shared resource
identities live in `internal/resource`; routing reuses those types.

The main application imports the inspection client, not the Docker adapter or
SDK. Its shared socket volume is read-only and it has no Docker socket mount.
The public HTTP API does not expose inventory. Socket filesystem permissions
control access to snapshots until an authenticated remote protocol is available.

Collection is a sequence of observations, not an atomic view of the Engine.
Container deletion during inspection, timeouts, and resource limits produce
explicit partial snapshots. A failed container listing returns a collection
error. Absence of a health check does not establish health, and one restart
counter cannot establish a restart loop.
