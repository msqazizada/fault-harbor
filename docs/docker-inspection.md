# Docker inspection

FaultHarbor collects container and network evidence through a dedicated process.
Configure one collector per Docker Engine with stable environment and source
identifiers. Two Engines in the same environment must use different source IDs.
Identifiers accept 1–64 letters, digits, dots, underscores, or hyphens and must
start with a letter or digit. Renaming one changes its resource identities.

## Deployment

Use the Compose commands in the [README](../README.md). The collector receives
`/var/run/docker.sock`; the application receives only `/run/faultharbor/collector.sock`.
For rootless Docker, set `FAULTHARBOR_DOCKER_SOCKET` to the Engine's socket path.
This transport supports Unix sockets. Authenticated remote transport is not
implemented; deployment location does not change the inventory contracts.

The Engine socket grants powerful host access even when mounted `:ro`: the mount
flag does not make Docker API calls read-only. The collector is a trusted process
with that privilege. FaultHarbor restricts its SDK transport to reads, exposes no
raw proxy endpoint, drops container capabilities, and disables collector networking.
Its dedicated Unix socket is mode `0660`, shared with group `65532`; membership in
that group grants access to inventory. Names and IP addresses remain operational
metadata, so protect the socket and exported snapshots accordingly.

The default deployment uses a root collector for the root-owned Engine socket.
A deployment with matching socket group permissions can use a non-root collector.
The application runs as user and group `65532`.

To stop both services, use the same Compose files:

```sh
docker compose -f compose.yaml -f deployments/compose.inspect.yaml down
```

The collector removes its socket on graceful shutdown. It refuses to replace an
existing socket. After an unclean termination, confirm no collector owns the
socket before removing the stale socket file and restarting the service.

## Native execution

After `make build`, create a private directory and start the collector:

```sh
mkdir -m 750 /tmp/faultharbor
./bin/faultharbor-collector --environment production --source engine-1 \
  --docker-socket /var/run/docker.sock \
  --listen-socket /tmp/faultharbor/collector.sock
```

In another terminal:

```sh
./bin/faultharbor inspect docker --socket /tmp/faultharbor/collector.sock
```

The invoking user must be able to connect to the Engine socket and write to the
collector directory. `--help` lists collector configuration flags.

## Evidence and limits

Schema version `1` includes source identity, collection start/end times,
per-resource observation times, container state, health-check status, exit code,
OOM status, restart count, and assigned network addresses. Image references are
immutable Engine image IDs. Networks include identity, name, driver, scope, and
internal status. Environment variables, arbitrary labels, commands, mounts,
health-check output, and raw daemon errors are omitted.

An absent `health` field means no health check was reported. Restart counts are
point-in-time evidence. Collection success does not mean services are healthy.
Network attachment is configuration evidence, not proof of connectivity.

| Limit | Default | Configuration |
| --- | --- | --- |
| Collection deadline | 20 seconds | `--timeout`, at most 1 minute |
| Individual request | 5 seconds | `--request-timeout` |
| Containers | 256 | `--max-containers`, at most 1000 |
| Networks | 256 | `--max-networks`, at most 1000 |
| Docker response | 2 MiB | Fixed, checked before decoding |
| Container observation | 16 KiB | Fixed |
| Snapshot response | 8 MiB | Fixed |
| Concurrent collections | 1 | Fixed; additional requests receive HTTP 503 |

Container listing failure returns no snapshot and exits `1`. Individual inspect
failures or inventory limits produce `state: partial`, structured issues, and exit
`2`. Complete snapshots exit `0`. Issues distinguish permission denial, timeout,
disappearing resources, invalid responses, size limits, and unavailable sources.
Raw provider errors are never returned. A complete snapshot describes the bounded
collection interval, not a transactionally consistent state of the Engine.

## Verification

`make check` runs unit tests and static analysis. After building
`docker build -t faultharbor:test .`, run `make test-integration` with Docker Compose
and Python 3 installed. The test creates a uniquely named disposable stack with
healthy, unhealthy, stopped, and restarting containers, checks their observations,
and removes that stack and its volume on exit. The collector reads the selected
Engine inventory; the test asserts only against its own fixture resources and
does not print other containers. CI runs the same fixture test.
