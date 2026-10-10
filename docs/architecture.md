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
Container and network collection will have their own capability contracts;
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
