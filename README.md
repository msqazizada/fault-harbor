# FaultHarbor

FaultHarbor is open-source infrastructure diagnostics software for investigating failures across services, networks, and routes. Its design separates infrastructure providers from diagnostic logic, with findings grounded in collected evidence.

The repository includes the Go server, React interface, Docker inspection collector, and provider-neutral routing contracts. Docker inspection is available through the CLI; diagnostic rules and inventory views are not yet connected to the interface.

## Run with Docker

```sh
docker compose up --build -d
```

Open [localhost:8080](http://localhost:8080). The container runs as a non-root user with a read-only filesystem. It has no Docker socket access. Stop it with `docker compose down`.

## Inspect Docker

The optional collector reads container state, health, restart counts, and network
attachments. The application receives sanitized snapshots through a restricted
Unix socket; only the collector has Docker Engine access.

```sh
export FAULTHARBOR_ENVIRONMENT=production
export FAULTHARBOR_SOURCE=engine-1
docker compose -f compose.yaml -f deployments/compose.inspect.yaml up --build -d
docker compose exec faultharbor faultharbor inspect docker
```

Inspection returns JSON and exits with `0` for complete collection, `2` for partial
collection, or `1` for failure. These statuses describe evidence collection, not
service health. See [Docker inspection](docs/docker-inspection.md) for limits,
permissions, and deployment details.

## Development

Requires Go 1.26+, Node.js 24 LTS, and npm.

```sh
npm ci --prefix web
make dev-api
```

In another terminal:

```sh
make dev-web
```

Open [localhost:5173](http://localhost:5173). Vite forwards API requests to the Go server on port 8080.

```sh
make check   # Formatting, static analysis, race tests, and frontend checks
make build   # Build web/dist and bin/faultharbor
./bin/faultharbor
```

Run the binary from the repository root, or set `FAULTHARBOR_WEB_DIR` to the built frontend directory. Use `make build VERSION=0.1.0` to set the reported version.

Keep changes focused and include a reproducible case for diagnostic fixes. Run `make check` and `make build` before opening a pull request. See [testing and analysis](docs/testing.md) for tool setup and individual checks.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `FAULTHARBOR_ADDR` | `127.0.0.1:8080` | HTTP listen address |
| `FAULTHARBOR_WEB_DIR` | `web/dist` | Built frontend directory |

The Docker image listens on `0.0.0.0:8080`; Compose publishes it on the host's loopback interface. This is a deployment default, not a restriction on the infrastructure FaultHarbor is designed to diagnose. Authentication is not implemented; deployment beyond loopback requires an authenticated reverse proxy with TLS.

The server exposes `GET /health/live`, `GET /health/ready`, and `GET /api/v1/version`. Readiness describes the FaultHarbor server, not the infrastructure it monitors.

## Structure

- `cmd/faultharbor`: startup and graceful shutdown.
- `internal/config`: environment configuration and validation.
- `internal/server`: HTTP endpoints and frontend serving.
- `cmd/faultharbor-collector`: isolated Docker collector.
- `internal/resource`: shared source and resource identities.
- `internal/inventory`: container and network observation contracts.
- `internal/dockerinspect`: Docker SDK adapter and read-only allowlist.
- `internal/inspection`: restricted collector protocol and client.
- `internal/routing`: provider contracts, routes, backends, and collection evidence.
- `web`: React, TypeScript, and Vite.

The HTTP server uses Go's standard library; the collector uses Docker's official SDK. Routing adapters translate provider data into shared snapshots; diagnostic rules consume those snapshots without importing provider SDKs. References include environment and source identity, and incomplete collection stays distinct from a healthy result. See [architecture](docs/architecture.md) for the integration boundaries.

## License

Apache-2.0. See [LICENSE](LICENSE).
