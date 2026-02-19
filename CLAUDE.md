# gcpql — Agent Context

## Project

`gcpql` is a Go CLI that queries GCP Cloud Monitoring, Cloud SQL Admin API, and BigQuery to surface operational health metrics. Binary name: `gcpql`.

## Module

```
github.com/moshebe/gcpql
```

## Repo Layout

```
main.go
cmd/                   # Cobra subcommands
  root.go              # --project persistent flag
  query.go             # raw PromQL query
  cloudsql.go          # cloudsql parent command
  cloudsql_check.go    # cloudsql check <instance>
  cloudsql_list.go     # cloudsql list
  cloudsql_diagnose.go # cloudsql diagnose <instance>
  bigquery.go          # bigquery parent command
  bigquery_check.go    # bigquery check <project>
pkg/
  monitoring/          # GCP Prometheus API client (PromQL over HTTP)
  cloudsql/            # check, list, diagnose: collector, admin, enrichment, formatter, types, derived, aggregator
  bigquery/            # check: client, check_collector, formatter, types, aggregator
  timerange/           # --since flag parser (5m / 1h / 7d)
  output/              # raw JSON formatter (query command only)
internal/
  config/              # project resolution: --project > GCP_PROJECT > gcloud config
docs/
  ARCHITECTURE.md      # design reference
  cloudsql-metrics-guide.md
```

## Key Design Patterns

**Auth:** Application Default Credentials. Scopes: `monitoring.read` + `sqlservice.admin`.

**monitoring.Client** (`pkg/monitoring/client.go`):
- POSTs to `monitoring.googleapis.com/v1/projects/{project}/location/global/prometheus/api/v1/query`
- Range selector injected from `--since`. Returns `[]interface{}` matrix (Prometheus format).
- `database_id` label = `"project:instance"` (NOT `project:region:instance`)
- Query Insights metrics use `resource_id` label instead of `database_id`
- `HTTPClient()` accessor reuses auth transport for other GCP REST APIs

**Multi-series metrics:** Many Cloud SQL metrics are per-Postgres-database (one series per DB). Use `statsFromData()` / `currentSums` to sum last values across series for accurate instance totals.

**Parallel collection:** All metric goroutines fan out via `chan metricResult`; enrichment (Recommender + Query Insights) runs in a separate goroutine via `chan enrichResult`. Both complete before `CollectMetrics` returns.

**Error handling idiom:**
- Use gerund form: `fmt.Errorf("creating client: %w", err)` — never `"failed to create ..."`
- Graceful degradation: non-critical failures (Recommender, Query Insights, monitoring metrics) return empty/zero structs, not errors
- Critical failures (Admin API 404, auth) propagate to the command

**Float parsing:** Use `strconv.ParseFloat(s, 64)` — not `fmt.Sscanf`.

**Go version:** 1.25 — `min()` / `max()` builtins available.

## Adding a New Service

1. Create `pkg/{service}/` with `collector.go`, `types.go`, `formatter.go`.
2. Add `cmd/{service}.go` (parent) and `cmd/{service}_check.go` (subcommand).
3. Reuse `monitoring.Client` (pass `HTTPClient()` for REST API calls).
4. See `docs/ARCHITECTURE.md` for detailed patterns.

## Build & Test

```bash
go build -o gcpql          # build binary
go test ./...                     # run all tests
go vet ./...                      # lint
```

## Instance ID Parsing

`cloudsql.ParseInstanceID` accepts:
- `my-instance` (requires `--project`)
- `my-project:my-instance`
- `my-project:region:my-instance`
