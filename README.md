# gcp-metrics

A CLI tool for querying GCP Cloud Monitoring metrics using MQL.

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
gcp-metrics query "fetch cloudsql_database | metric 'cloudsql.googleapis.com/database/cpu/utilization'"
```

### With time range
```bash
gcp-metrics query "..." --since 1h
gcp-metrics query "..." --since 24h
```

The `--since` flag automatically appends `| within <duration>` to your MQL query. If your query already contains a `within` clause, the `--since` flag is ignored. Default time range is 5 minutes.

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
