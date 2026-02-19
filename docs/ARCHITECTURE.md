# Architecture

## Overview

`gcp-metrics` is a Go CLI that queries GCP Cloud Monitoring via PromQL. It exposes service-specific subcommands that fetch, aggregate, and format metrics.

## Layout

```
cmd/            # Cobra subcommands (root, query, cloudsql)
pkg/
  monitoring/   # GCP Monitoring API client (PromQL over HTTP)
  cloudsql/     # CloudSQL check: collector, aggregator, formatter, types
  timerange/    # Parse --since flag (5m / 1h / 7d)
  output/       # Generic JSON formatter (used by query command)
internal/
  config/       # Project resolution: --project > GCP_PROJECT > gcloud config
main.go
```

## Key Design Points

**Auth:** Application Default Credentials (`gcloud auth application-default login`). Scopes: `monitoring.read` + `sqlservice.admin`.

**PromQL client** (`pkg/monitoring/client.go`): POSTs to `monitoring.googleapis.com/v1/projects/{project}/location/global/prometheus/api/v1/query`. Range selector injected from `--since`. `HTTPClient()` accessor lets other packages reuse the auth transport.

**CloudSQL check** (`pkg/cloudsql/`):
1. `admin.go` — calls Cloud SQL Admin API (`sqladmin.googleapis.com/v1/projects/{p}/instances/{i}`) for region, db version, and authoritative `max_connections`. Returns error on 404.
2. `collector.go` — fetches ~35 metrics in parallel goroutines, populates `CheckResult`.
3. `aggregator.go` — `CalculateStats(points, unit)` captures `current` (last point, time-ascending) before sorting; returns `{current, p50, p99, max, min, avg}`.
4. `formatter.go` — `FormatJSON` / `FormatTable`. Table sections: DERIVED INSIGHTS, RESOURCES, CONNECTIONS, QUERY PERFORMANCE, CACHE PERFORMANCE, THROUGHPUT, DATABASE HEALTH, CHECKPOINTS, REPLICATION (skipped if empty).
5. `types.go` — `CheckResult` and all nested structs.

**Time window:** `timerange.Parse` supports `m`, `h`, `d` suffixes. `cmd/cloudsql_check.go` uses it (not `time.ParseDuration`).

**Metadata:** `metrics_collected`, `metrics_no_data` (no error but empty series), `metrics_unavailable` (API error), `collection_duration_ms`.

## Adding a New Service

1. Create `pkg/{service}/` with `collector.go`, `types.go`, `formatter.go`.
2. Add `cmd/{service}.go` and `cmd/{service}_check.go`.
3. Reuse `monitoring.Client` (pass `HTTPClient()` if you need the raw transport for a REST API).
