# gcpql

A CLI for querying GCP Cloud Monitoring metrics and diagnosing CloudSQL and BigQuery health.

## Install

### Homebrew (Linux/macOS)

```bash
brew install moshebe/pkg/gcpql
```

### Go

```bash
go install github.com/moshebe/gcpql@latest
```

Or build from source:

```bash
git clone https://github.com/moshebe/gcpql
cd gcpql
go build -o gcpql
```

## Prerequisites

- GCP project with Cloud Monitoring API enabled
- Authenticated: `gcloud auth application-default login`
- IAM roles: `roles/monitoring.viewer` (all commands) + `roles/cloudsql.viewer` (CloudSQL commands) + BigQuery Data Viewer (BigQuery commands)

## Commands

### Raw PromQL query

```bash
gcpql query "cloudsql.googleapis.com/database/cpu/utilization" --project my-project
gcpql query '{__name__="cloudsql.googleapis.com/database/cpu/utilization",database_id="my-project:my-instance"}' --since 1h
```

### CloudSQL

#### List instances

```bash
gcpql cloudsql list --project my-project
gcpql cloudsql list --project my-project --format json
gcpql cloudsql list --project my-project --since 15m
```

```
Project: my-project  (2 instances)

┌──────────────────────────┬──────────┬─────────────┬─────────────┬─────────┬─────────┬──────┬──────┐
│ INSTANCE                 │ STATE    │ VERSION     │ REGION      │ CPU     │ MEM     │ vCPU │ RAM  │
├──────────────────────────┼──────────┼─────────────┼─────────────┼─────────┼─────────┼──────┼──────┤
│ my-project:prod-db       │ RUNNABLE │ POSTGRES_15 │ us-central1 │ 🟢 42% │ 🟡 71% │ 4    │ 15GB │
│ my-project:staging-db    │ RUNNABLE │ POSTGRES_14 │ us-east1    │ 🟢  8% │ 🟢 34% │ 2    │ 8GB  │
└──────────────────────────┴──────────┴─────────────┴─────────────┴─────────┴─────────┴──────┴──────┘
```

Status: 🟢 <70% · 🟡 70–90% · 🔴 ≥90% · `-` no data

#### Check instance health

```bash
gcpql cloudsql check my-instance --project my-project
gcpql cloudsql check my-project:my-instance --format table
gcpql cloudsql check my-project:my-instance --since 7d --query-insights
```

**Instance ID formats:** `my-instance` (needs `--project`), `my-project:my-instance`, `my-project:region:my-instance`

**Metrics:** CPU · Memory · Disk · Connections · Cache hit ratio · Query performance (P50/P99 latency, I/O wait, lock time) · Throughput · Replication · Checkpoints · XID wraparound · Recommendations · Query Insights (opt-in)

**Output formats:** `json` (default, pipe to `jq`) · `table` (human-readable with status indicators)

```bash
gcpql cloudsql check my-project:my-instance | jq '.derived_insights.cache_hit_ratio'
gcpql cloudsql check my-project:my-instance | jq '.connections.count.current'
```

#### Diagnose

Analyzes metrics against known problem patterns and returns prioritized findings with remediation steps:

```bash
gcpql cloudsql diagnose my-project:my-instance
gcpql cloudsql diagnose my-project:my-instance --format json
gcpql cloudsql diagnose my-project:my-instance --query-insights --since 7d
```

```
🔴 CRITICAL  XID Wraparound Imminent
            87.3% of PostgreSQL transaction IDs consumed (critical threshold: 80%)
            → Run VACUUM FREEZE on all databases immediately to reclaim XIDs
            → ...

🟡 WARNING   High Connection Utilization
            82.1% of max connections used (246 / 300)
            → Consider deploying a connection pooler (PgBouncer, pgpool-II)
            → ...
```

### BigQuery

#### Check health

```bash
gcpql bigquery check my-project
gcpql bigquery check my-project --format table
gcpql bigquery check my-project --dataset analytics --since 7d
```

**Metrics:** Slot utilization · Storage costs · Bytes scanned · Top expensive queries · Job summary (total, failed, cache hit rate)

```bash
gcpql bigquery check my-project | jq '.slots'
gcpql bigquery check my-project | jq '.top_queries[0]'
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | gcloud config | GCP project ID |

Project resolution order: `--project` flag → `GCP_PROJECT` env → `gcloud config get-value project`

## Exit Codes

- `0` Success
- `1` Error (config, auth, API, validation)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
