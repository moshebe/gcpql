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

## Output

JSON format (pipe to `jq` for pretty printing):
```bash
gcp-metrics query "..." | jq .
```

## Exit Codes

- `0` - Success
- `1` - Error (config, auth, API, validation)
