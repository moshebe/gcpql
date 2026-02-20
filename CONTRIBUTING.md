# Contributing

## Setup

```bash
git clone https://github.com/moshebe/gcpql
cd gcpql
go build -o gcpql
```

**Prerequisites:** Go 1.25+ (see `go.mod`), `gcloud` CLI authenticated with `gcloud auth application-default login`.

## Running Tests

```bash
go test ./...
go vet ./...
```

Tests are pure unit tests — no GCP credentials required.

## Code Style

- Idiomatic Go: short variable names, early returns, no unnecessary abstractions
- CI runs `golangci-lint`; `errcheck` is currently disabled with the goal of re-enabling once unchecked error returns are fixed
- Error messages use gerund form: `"creating client: %w"` not `"failed to create client: %w"`
- Float parsing: `strconv.ParseFloat` not `fmt.Sscanf`
- Parallel metric collection via goroutines + channels (see `pkg/cloudsql/collector.go`)
- Graceful degradation: non-critical API failures return zero values, not errors

## Project Structure

See `AGENTS.md` for agent context (all AI coding agents) and `docs/ARCHITECTURE.md` for design decisions.

## Adding a New GCP Service

1. Create `pkg/{service}/` — follow the `pkg/cloudsql/` pattern
2. Add `cmd/{service}.go` and `cmd/{service}_check.go`
3. Write unit tests with mock HTTP servers (see `pkg/cloudsql/admin_test.go`)
4. Update `README.md` with usage examples

## Pull Requests

- Target the default branch (`master`)
- Include tests for new functionality
- Run `go test ./... && go vet ./...` before submitting
