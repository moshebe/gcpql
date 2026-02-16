from google.cloud import monitoring_v3
from datetime import datetime, timedelta                                                             
import os                                                

project = os.popen('gcloud config get-value project').read().strip()
client = monitoring_v3.MetricServiceClient()

results = client.list_time_series(name=f'projects/{project}',
      filter='metric.type="cloudsql.googleapis.com/database/cpu/utilization"',
      interval={'end_time': datetime.utcnow(), 'start_time': datetime.utcnow() - timedelta(minutes=5)}
  )

for r in results:
    print(f"{r.resource.labels['database_id']}: {r.points[0].value.double_value:.2%}")
