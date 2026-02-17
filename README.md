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
- Resources: CPU, memory, disk utilization and I/O
- Connections: Count, status breakdown, limits
- Query Performance: Latency (P50/P99), I/O wait, lock time (requires Query Insights)
- Database Health: Transaction IDs, deadlocks, vacuum activity
- Replication: Lag in bytes and seconds
- Network: Ingress/egress throughput
- Checkpoints: Sync and write latencies
- Temp Data: Bytes written, files created

**Output formats:**
- `json` - Machine-readable, suitable for piping to `jq` or other tools
- `table` - Human-readable tables with sections

**Example: Pipe to jq**
```bash
gcp-metrics cloudsql check my-instance | jq '.resources.cpu'
```

## Output

JSON format (pipe to `jq` for pretty printing):
```bash
gcp-metrics query "..." | jq .
```

## Exit Codes

- `0` - Success
- `1` - Error (config, auth, API, validation)
