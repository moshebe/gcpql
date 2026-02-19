# gcp-metrics

A CLI tool for querying GCP Cloud Monitoring metrics using PromQL.

## Installation

```bash
go build -o gcp-metrics
```

## Prerequisites

- GCP project with Cloud Monitoring API enabled
- Authentication configured:
  ```bash
  gcloud auth application-default login
  ```
- IAM role: `roles/monitoring.viewer`

## Usage

### Basic query
```bash
# Simple metric name (auto-wrapped in __name__ format)
gcp-metrics query "cloudsql.googleapis.com/database/cpu/utilization"

# Explicit __name__ format also works
gcp-metrics query '{__name__="cloudsql.googleapis.com/database/cpu/utilization"}'
```

**Note:** GCP requires metric names with dots/slashes to use the `__name__` label selector format. The tool automatically wraps simple metric names for you.

### With time range
```bash
gcp-metrics query "cloudsql.googleapis.com/database/cpu/utilization" --since 5m
gcp-metrics query "cloudsql.googleapis.com/database/cpu/utilization" --since 1h
```

The `--since` flag automatically appends a PromQL range selector `[duration]` to your query. If your query already contains a range selector like `[5m]`, the `--since` flag is ignored. Default time range is 5 minutes.

### With label selectors
```bash
# Filter by labels
gcp-metrics query '{__name__="cloudsql.googleapis.com/database/cpu/utilization",database_id="my-instance"}'

# Multiple labels
gcp-metrics query '{__name__="cloudsql.googleapis.com/database/cpu/utilization",database_id="my-db",region="us-central1"}'
```

### Custom project
```bash
gcp-metrics query "..." --project my-project
```

### Via environment variable
```bash
export GCP_PROJECT=my-project
gcp-metrics query "..."
```

## CloudSQL Commands

### List instances

List all CloudSQL instances in a project with live CPU and memory utilization:

```bash
# Table output (default)
gcp-metrics cloudsql list --project my-project

# JSON output
gcp-metrics cloudsql list --project my-project --format json

# Custom time window for metrics
gcp-metrics cloudsql list --project my-project --since 15m
```

**Columns:** INSTANCE, STATE, VERSION, REGION, CPU, MEM, vCPU, RAM

**Example output:**
```
Project: my-project  (2 instances)

┌──────────────────────────┬─────────┬─────────────┬─────────────┬──────────┬──────────┬──────┬──────┐
│ INSTANCE                 │ STATE   │ VERSION     │ REGION      │ CPU      │ MEM      │ vCPU │ RAM  │
├──────────────────────────┼─────────┼─────────────┼─────────────┼──────────┼──────────┼──────┼──────┤
│ my-project:prod-db       │ RUNNABLE│ POSTGRES_15 │ us-central1 │ 🟢 42%  │ 🟡 71%  │ 4    │ 15GB │
│ my-project:staging-db    │ RUNNABLE│ POSTGRES_14 │ us-east1    │ 🟢  8%  │ 🟢 34%  │ 2    │ 8GB  │
└──────────────────────────┴─────────┴─────────────┴─────────────┴──────────┴──────────┴──────┴──────┘
```

**Status indicators:**
- 🟢 Good: CPU/Memory < 70%
- 🟡 Warning: CPU/Memory 70–90%
- 🔴 Critical: CPU/Memory ≥ 90%
- `-` No monitoring data (instance stopped or metrics delayed)

**Note:** Requires `roles/monitoring.viewer` + `roles/cloudsql.viewer` (or `roles/cloudsql.admin`).

### Check instance health

Get comprehensive CloudSQL instance metrics:

```bash
# JSON output (default)
gcp-metrics cloudsql check my-instance

# Human-readable table output
gcp-metrics cloudsql check my-instance --format table

# Custom time window
gcp-metrics cloudsql check my-instance --since 7d

# Specify project
gcp-metrics cloudsql check my-instance --project my-project
```

**Instance ID formats:**
- Short form: `my-instance` (requires `--project` flag)
- Full form: `my-project:my-instance`
- Database ID: `my-project:us-central1:my-instance`

**Metrics included:**
- **Derived Insights:** Cache hit ratio, connection utilization, disk/memory usage percentages
- **Resources:** CPU, memory, disk utilization and I/O
- **Cache Performance:** Query cache hit/miss ratios, block hit ratios
- **Query Performance:** Latency (P50/P99), I/O wait, lock time (requires Query Insights)
- **Connections:** Count, status breakdown, limits, utilization
- **Database Health:** Transaction IDs, deadlocks, vacuum activity
- **Throughput:** Queries per second, statements per second
- **Replication:** Lag in bytes and seconds
- **Network:** Ingress/egress throughput
- **Checkpoints:** Sync and write latencies
- **Temp Data:** Bytes written, files created

**Output formats:**
- `json` - Machine-readable, suitable for piping to `jq` or other tools
- `table` - Human-readable tables with sections and status indicators (🟢🟡🔴)

**Status indicators** (table format only):
- 🟢 Good: CPU < 70%, Memory < 80%, Disk < 80%, Cache Hit > 90%, Connections < 80%
- 🟡 Warning: Moderate resource usage
- 🔴 Critical: High resource usage requiring attention

**Example: Pipe to jq**
```bash
gcp-metrics cloudsql check my-instance | jq '.resources.cpu'
gcp-metrics cloudsql check my-instance | jq '.derived_insights.cache_hit_ratio'
```

**Note on metric availability:**
Some metrics depend on specific configurations:
- Query performance metrics require Query Insights enabled
- Replication metrics only available for read replicas
- PostgreSQL-specific metrics (vacuum, temp files) not available on MySQL
- Recent metrics (< 2min) may be delayed due to GCP API propagation

## BigQuery Commands

### Check health metrics

Get comprehensive BigQuery health metrics including slots, costs, and top queries:

```bash
# JSON output (default)
gcp-metrics bigquery check my-project

# Human-readable table output
gcp-metrics bigquery check my-project --format table

# Custom time window
gcp-metrics bigquery check my-project --since 7d

# Filter by dataset
gcp-metrics bigquery check my-project --dataset analytics
```

**Metrics included:**
- **Slot Utilization:** Allocated, current, peak, queries in flight
- **Cost Indicators:** Storage costs, bytes scanned, estimated query costs
- **Top Expensive Queries:** Most costly queries by bytes processed

**Output formats:**
- `json` - Machine-readable, suitable for piping to `jq`
- `table` - Human-readable with status indicators ([OK] [WARN] [CRIT])

**Status indicators** (table format only):
- [OK] Good: Slots < 70%, Daily cost < $100
- [WARN] Warning: Moderate usage
- [CRIT] Critical: Slots ≥ 90%, Daily cost > $500

**Example: Pipe to jq**
```bash
gcp-metrics bigquery check my-project | jq '.slots'
gcp-metrics bigquery check my-project | jq '.top_queries[0]'
```

**Note on data sources:**
- Real-time metrics from Cloud Monitoring API
- Query history from INFORMATION_SCHEMA (requires BigQuery Data Viewer role)
- Minimal query costs (<10MB typically)

## Output

JSON format (pipe to `jq` for pretty printing):
```bash
gcp-metrics query "..." | jq .
```

## Exit Codes

- `0` - Success
- `1` - Error (config, auth, API, validation)
