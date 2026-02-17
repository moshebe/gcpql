from google.cloud import monitoring_v3
from datetime import datetime, timedelta
import os

project = os.popen('gcloud config get-value project').read().strip()
client = monitoring_v3.MetricServiceClient()

# Get current (last 5 min)
current = client.list_time_series(name=f'projects/{project}',
      filter='metric.type="cloudsql.googleapis.com/database/cpu/utilization"',
      interval={'end_time': datetime.utcnow(), 'start_time': datetime.utcnow() - timedelta(minutes=5)}
  )

# Get 24h history
history = client.list_time_series(name=f'projects/{project}',
      filter='metric.type="cloudsql.googleapis.com/database/cpu/utilization"',
      interval={'end_time': datetime.utcnow(), 'start_time': datetime.utcnow() - timedelta(hours=24)}
  )

# Build max map
max_map = {}
for r in history:
    db_id = r.resource.labels['database_id']
    max_val = max(p.value.double_value for p in r.points)
    max_map[db_id] = max_val

for r in current:
    db_id = r.resource.labels['database_id']
    curr = r.points[0].value.double_value
    max_24h = max_map.get(db_id, curr)
    print(f"{db_id}: {curr:.2%} (24h max: {max_24h:.2%})")
