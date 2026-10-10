# Testing and analysis

Install Go 1.26+, Node.js 24, and
[golangci-lint v2.14.0](https://golangci-lint.run/docs/welcome/install/local/).
Use the published binary for your operating system and verify it against the
release checksum file. Keep the linter version aligned with CI.

```sh
npm ci --prefix web
make check
make build
```

## Checks

| Command | Checks |
| --- | --- |
| `make fmt` | Apply Go formatting |
| `make fmt-check` | Reject unformatted Go files |
| `make lint-go` | Run the configured Go analyzers |
| `make lint-web` | Run type-aware ESLint and TypeScript checks |
| `make test` | Run Go race tests and frontend tests |
| `make coverage` | Run Go race tests and write text/HTML coverage reports |
| `make security` | Run govulncheck and npm's vulnerability audit |
| `make check` | Run formatting checks, lint, type checks, and tests |

Set `GOLANGCI_LINT=/path/to/golangci-lint` if the binary is not on your PATH.
Security checks require network access. The pinned govulncheck version is run
separately from application dependencies and does not modify the root Go module.

## Static analysis

`.golangci.yml` enables a selected set of analyzers rather than every available
style rule. Staticcheck and go vet find suspicious or incorrect Go constructs;
errcheck and errorlint check error handling; gosec checks security patterns.
Additional checks cover unused code, ineffective assignments, HTTP response
cleanup, missing request contexts, and contradictory error branches.

Fix findings at their source. A necessary suppression must name the specific
linter and explain why it is safe. Type-aware ESLint checks the frontend for
issues such as unhandled promises and unsafe operations on untyped values.

## Test conventions

- Test observable behavior, especially negative cases and incomplete evidence.
- Keep tests beside the implementation: `*_test.go` and `*.test.ts`.
- Use fixed timestamps, synthetic provider fixtures, and in-memory HTTP handlers.
- Preserve provider errors and cancellation; never interpret unavailable evidence as success.
- Go tests run with race detection, shuffled order, a timeout, and the test cache disabled.
- Environment-mutating tests must not call `t.Parallel`.

Coverage reports live in the ignored `coverage/` directory. Use them to find
untested behavior, not to justify tests that merely repeat the implementation.
The command startup layer is included in the report even when it has no unit
coverage. Unit fixtures test provider contracts; Docker integration tests exercise the
collector and CLI against a real Engine.

## Continuous integration

Pushes and pull requests run static analysis, tests, coverage, vulnerability scans,
application builds, a Docker build, and disposable Docker integration fixtures. Go coverage is retained as a workflow
artifact for 14 days. A failed check fails the job; checks do not modify files or
commit changes. Branch protection must be configured separately in GitHub to
require this job before merging.

## Docker integration

```sh
docker build -t faultharbor:test .
make test-integration
```

Requires Docker Engine, Docker Compose, and Python 3. The test uses a unique Compose
project and cleans up its own resources. It verifies real container states,
network references, timestamps, secret omission, and the consumer's lack of Docker
socket access. Set `FAULTHARBOR_TEST_IMAGE` to reuse another built image.
