# GCP Monitoring Notes

## Key Learnings

### gcloud CLI Limitations
- Older gcloud versions lack `gcloud monitoring` commands (time-series, metric-descriptors, query)
- Only available: dashboards, policies, snoozes, uptime
- Direct API calls or Python client library are more reliable

### Working Approach: Python + google-cloud-monitoring

**Installation:**
```bash
uv run --with google-cloud-monitoring python3 script.py
```

**Basic Query Pattern:**
```python
from google.cloud import monitoring_v3
from datetime import datetime, timedelta

project = "projects/YOUR_PROJECT"
client = monitoring_v3.MetricServiceClient()

results = client.list_time_series(
    name=project,
    filter='metric.type="METRIC_TYPE"',
    interval={
        'end_time': datetime.utcnow(),
        'start_time': datetime.utcnow() - timedelta(minutes=5)
    }
)

for r in results:
    instance = r.resource.labels['database_id']  # or other label
    value = r.points[0].value.double_value
    print(f"{instance}: {value:.2%}")
```

## CloudSQL Metrics Reference

### Core Health Metrics
```
cloudsql.googleapis.com/database/cpu/utilization
cloudsql.googleapis.com/database/memory/utilization
cloudsql.googleapis.com/database/disk/utilization
```

### Additional Metrics
```
# Disk
cloudsql.googleapis.com/database/disk/bytes_used
cloudsql.googleapis.com/database/disk/read_ops_count
cloudsql.googleapis.com/database/disk/write_ops_count

# Connections (PostgreSQL)
cloudsql.googleapis.com/database/postgresql/num_backends

# Connections (MySQL)
cloudsql.googleapis.com/database/mysql/connections

# Replication
cloudsql.googleapis.com/database/replication/replica_lag
```

### Health Thresholds
- CPU: >80% sustained = upgrade needed
- Memory: >85% = upgrade needed
- Disk: >80% = expand needed
- Connections: >80% of max = scale up

## Verified Test
```bash
# Working test script
uv run --with google-cloud-monitoring python3 check_cloudsql.py
```

Successfully queries CloudSQL CPU utilization for all instances in project.
