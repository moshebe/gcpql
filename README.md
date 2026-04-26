# gcpql — Google Cloud Monitoring, in your terminal

[![CI](https://github.com/moshebe/gcpql/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/moshebe/gcpql/actions/workflows/ci.yml)
[![Go 1.24](https://img.shields.io/badge/go-1.24-00ADD8?logo=go)](https://go.dev/doc/go1.24)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`gcpql` runs PromQL against GCP Cloud Monitoring from your terminal — query any metric instantly.

**Per-product extensions** go beyond raw metrics, combining monitoring data with Admin API
context to surface actionable findings and remediation steps.

## Install

### Homebrew (macOS / Linux)

```bash
brew install moshebe/pkg/gcpql
```

### Go

```bash
go install github.com/moshebe/gcpql@latest
```

### Source

```bash
git clone https://github.com/moshebe/gcpql
cd gcpql
go build -o gcpql
```

## Prerequisites

- GCP project with Cloud Monitoring API enabled
- `gcloud auth application-default login`
- IAM: `roles/monitoring.viewer` (all commands); extension-specific roles (e.g. Cloud SQL viewer, BigQuery Data Viewer, Error Reporting viewer, PubSub viewer) only when using those extensions

## Quick start

```bash
# Query any GCP metric with PromQL
gcpql query '{__name__="cloudsql.googleapis.com/database/cpu/utilization"}' --project my-project | jq '.time_series'
gcpql query 'up{job="my-job"}' --project my-project --since 1h

# Per-product health checks and diagnostics
gcpql cloudsql list --project my-project
gcpql cloudsql check my-project:prod-db --format table
gcpql cloudsql diagnose my-project:prod-db --query-insights
gcpql bigquery check my-project --format table
gcpql errorreporting list --project my-project --format table
gcpql pubsub check my-project --format table
gcpql pubsub diagnose my-project/my-sub --format table
```

## Commands

### `query` — raw PromQL

Run instant PromQL queries against GCP Cloud Monitoring.

```bash
gcpql query "cloudsql.googleapis.com/database/cpu/utilization" --project my-project
gcpql query '{__name__="cloudsql.googleapis.com/database/cpu/utilization", database_id="my-project:prod-db"}' --since 1h
gcpql query 'up' --project my-project --since 5m | jq .
```

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | gcloud config | GCP project ID |
| `--since` | metric default | Time window (5m, 1h, 7d) |

Project resolution: `--project` → `GCP_PROJECT` env → `gcloud config get-value project`

---

## Product extensions

Extensions combine PromQL metrics with Admin API data to provide list/check/diagnose flows.

### `cloudsql list`

Lists instances in a project with live CPU and memory utilization, sorted by CPU descending.

```bash
gcpql cloudsql list --project my-project
gcpql cloudsql list --project my-project --since 15m
gcpql cloudsql list --project my-project --format json
```

```
Project: my-project  (8 instances)

┌─────────────────────────────┬──────────┬─────────────┬─────────────┬──────────┬────────┬──────┬───────┐
│ INSTANCE                    │ STATE    │ VERSION     │ REGION      │ CPU      │ MEM    │ VCPU │ RAM   │
├─────────────────────────────┼──────────┼─────────────┼─────────────┼──────────┼────────┼──────┼───────┤
│ my-project:prod-db          │ RUNNABLE │ POSTGRES_14 │ us-central1 │ 🔴 100%  │ 🟢 44% │ 4    │ 15GB  │
│ my-project:app-db           │ RUNNABLE │ POSTGRES_16 │ us-central1 │ 🟢 37%   │ 🟢 47% │ 2    │ 16GB  │
│ my-project:replica-01       │ RUNNABLE │ POSTGRES_14 │ us-central1 │ 🟢 33%   │ 🟢 42% │ 4    │ 15GB  │
│ my-project:analytics-db     │ RUNNABLE │ POSTGRES_14 │ us-central1 │ 🟢 26%   │ 🟢 59% │ 8    │ 30GB  │
│ my-project:large-instance   │ RUNNABLE │ POSTGRES_14 │ us-central1 │ 🟢 15%   │ 🟢 40% │ 64   │ 128GB │
│ my-project:logs-db          │ RUNNABLE │ POSTGRES_16 │ us-central1 │ 🟢 14%   │ 🟢 41% │ 36   │ 45GB  │
│ my-project:history-db       │ RUNNABLE │ POSTGRES_16 │ us-central1 │ 🟢 13%   │ 🟡 70% │ 36   │ 36GB  │
│ my-project:primary-db       │ RUNNABLE │ POSTGRES_14 │ us-central1 │ 🟢  1%   │ 🟢 37% │ 32   │ 256GB │
└─────────────────────────────┴──────────┴─────────────┴─────────────┴──────────┴────────┴──────┴───────┘
```

🟢 <70% · 🟡 70–90% · 🔴 ≥90% · `-` no data

### `cloudsql check`

Deep health check covering CPU, memory, disk, connections, cache hit ratio, query latency (P50/P99), XID wraparound, Recommender suggestions, and more.

```bash
gcpql cloudsql check my-project:prod-db --format table
gcpql cloudsql check my-project:prod-db --since 7d --query-insights
gcpql cloudsql check my-project:prod-db | jq '.derived_insights.cache_hit_ratio'
gcpql cloudsql check my-project:prod-db | jq '.connections.count.current'
```

**Instance ID formats:** `my-instance` (needs `--project`) · `my-project:my-instance` · `my-project:region:my-instance`

**Output:** `json` (default, pipe to `jq`) · `table` (human-readable with status indicators)

### `cloudsql diagnose`

Analyzes metrics against known problem patterns. Returns prioritized findings with remediation steps.

```bash
gcpql cloudsql diagnose my-project:prod-db --query-insights --since 7d
gcpql cloudsql diagnose my-project:prod-db --format json
```

```
Instance:    my-project:prod-db
Region:      us-central1
Time Window: 7d

┌──────────────────────────────┐
│ LOAD SUMMARY                 │
├──────────────────────┬───────┤
│ METRIC               │ VALUE │
├──────────────────────┼───────┤
│ Avg Transactions/sec │ 69.5  │
└──────────────────────┴───────┘

4 issue(s) found:

🔴 CRITICAL  Disk Nearly Full
            91.0% disk used (auto-resize: true)
            → Auto-resize is on — check if the auto-resize limit has been reached in Cloud SQL settings
            → Increase or remove the auto-resize cap
            → Identify and purge large/bloated tables or indexes

🔴 CRITICAL  Critically Slow Query Pattern
            Top contributor 'app-user@my-project.iam'@'prod': avg 10485ms (10.5s) latency
            → Run EXPLAIN (ANALYZE, BUFFERS) on queries from this user/database
            → Check for missing indexes, sequential scans on large tables
            → Look for lock waits or I/O saturation causing query delays
            → Review pg_stat_statements for specific slow query hashes

🟡 WARNING   High CPU Pressure
            CPU P99: 99.8% — sustained load may cause query latency spikes
            → Identify expensive queries via Query Insights (enable if not already on)
            → Check for missing indexes causing full table scans
            → Review autovacuum/analyze frequency — excessive vacuum can spike CPU
            → Consider upgrading to a larger instance tier if CPU is consistently high

ℹ️  INFO      Query Load Highly Concentrated
            'app-user@my-project.iam'@'prod' accounts for 100% of total query time
            → Optimize this user/database's queries first for maximum impact
            → Review indexes, query plans, and connection patterns for this user
```

### `bigquery check`

Slot utilization, storage costs, bytes scanned, top expensive queries, and job summary (total, failed, cache hit rate).

```bash
gcpql bigquery check my-project
gcpql bigquery check my-project --format table
gcpql bigquery check my-project --dataset analytics --since 7d
gcpql bigquery check my-project | jq '.slots'
gcpql bigquery check my-project | jq '.top_queries[0]'
```

### `errorreporting list`

Top 50 error groups from GCP Error Reporting, ordered by count. Requires `roles/errorreporting.viewer`.

```bash
gcpql errorreporting list --project my-project
gcpql errorreporting list --project my-project --format table
gcpql errorreporting list --project my-project --since 1d --format table
gcpql errorreporting list --project my-project --service my-service
```

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | 7d | Look-back window; mapped to nearest API period: `≤1h`, `≤6h`, `≤24h`, `≤7d`, `>7d` (30d) |
| `--service` | all | Filter to a specific service name |
| `--format` | `json` | `json` or `table` |

### `pubsub check`

Project-wide health snapshot of all subscriptions. Surfaces backlog size, consumer lag, expired ack deadlines, and DLQ counts — with per-subscription status and a top offenders summary.

Requires `roles/pubsub.viewer` in addition to `roles/monitoring.viewer`.

```bash
gcpql pubsub check my-project
gcpql pubsub check my-project --format table
gcpql pubsub check my-project --since 30m --top 10
gcpql pubsub check my-project | jq '.subscriptions[] | select(.status == "CRITICAL")'
```

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | `1h` | Look-back window for rate metrics (5m, 1h, 7d) |
| `--top` | `5` | Number of worst subscriptions shown in top-offenders list |
| `--format` | `json` | `json` or `table` |

```
Project: my-project  (4 subscriptions)

┌──────────────────────────────────┬─────────┬──────────────┬──────────┬─────┬──────────┐
│ SUBSCRIPTION                     │ BACKLOG │ OLDEST UNACK │ EXP. ACK │ DLQ │ STATUS   │
├──────────────────────────────────┼─────────┼──────────────┼──────────┼─────┼──────────┤
│ orders-subscription              │  58,320 │ 2h15m        │      143 │   0 │ CRITICAL │
│ notifications-subscription       │  12,500 │ 12m30s       │        0 │   0 │ WARNING  │
│ analytics-subscription           │     420 │ 45s          │        0 │   0 │ OK       │
│ audit-subscription               │       0 │ -            │        0 │   0 │ OK       │
└──────────────────────────────────┴─────────┴──────────────┴──────────┴─────┴──────────┘

Top 2 offender(s):
  🔴 orders-subscription       oldest unacked: 2h15m
  🟡 notifications-subscription  backlog: 12,500 messages
```

### `pubsub diagnose`

Deep-dive diagnosis of a single subscription and its parent topic. Collects 10 metrics (subscription backlog, consumer lag, expired ack deadlines, DLQ, ack rate, pull/push error rates, topic publish rate, publish errors, message size) and runs them through a rule engine to produce prioritized findings with remediation steps.

Requires `roles/pubsub.viewer` in addition to `roles/monitoring.viewer`.

```bash
gcpql pubsub diagnose my-sub --project my-project
gcpql pubsub diagnose projects/my-project/subscriptions/my-sub
gcpql pubsub diagnose my-sub --project my-project --format table
gcpql pubsub diagnose my-sub --project my-project --since 6h | jq '.findings'
```

**Subscription ID formats:** `my-sub` (needs `--project`) · `projects/my-project/subscriptions/my-sub`

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | gcloud config | GCP project ID (required if not using full subscription path) |
| `--since` | `1h` | Look-back window (5m, 1h, 7d) |
| `--format` | `json` | `json` or `table` |

```
Subscription: my-project/orders-subscription
Topic:        my-project/orders-topic
Time Window:  1h

┌──────────────────────────────────────┐
│ LOAD SUMMARY                         │
├─────────────────────────┬────────────┤
│ METRIC                  │ VALUE      │
├─────────────────────────┼────────────┤
│ Backlog (current)       │ 58,320     │
│ Oldest Unacked          │ 2h15m      │
│ Ack Rate                │ 0 msg/s    │
│ Publish Rate            │ 142 msg/s  │
│ Avg Message Size        │ 1.2 KB     │
└─────────────────────────┴────────────┘

3 issue(s) found:

🔴 CRITICAL  Subscription Severely Backlogged
            Oldest unacked message: 2h15m (threshold: 1h)
            → Check if consumers are running and processing messages
            → Inspect consumer logs for processing errors or panics
            → Consider scaling out consumer replicas
            → Verify the subscription ack deadline is long enough for processing time

🔴 CRITICAL  No Active Consumer
            Backlog is 58,320 messages but ack rate is 0 msg/s
            → Check if consumer service is deployed and healthy
            → Verify subscription credentials and IAM permissions
            → Check if consumer is crashing before acknowledging

🟡 WARNING   Consumers Missing Ack Deadline
            143 expired ack deadlines in window
            → Increase the subscription ack deadline if processing takes longer
            → Reduce per-message processing time or process in smaller batches
            → Check for downstream bottlenecks causing slow processing
```

---

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | gcloud config | GCP project ID |
| `--since` | metric default | Time window (5m, 1h, 7d) |
| `--format` | `json` | Output format: `json` or `table` (where supported) |

Project resolution: `--project` flag → `GCP_PROJECT` env → `gcloud config get-value project`

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, code style, and PR guidelines.

AI agents: see [AGENTS.md](AGENTS.md) for project layout, patterns, and conventions.
