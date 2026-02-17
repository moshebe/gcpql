from google.cloud import monitoring_v3
from datetime import datetime, timedelta
import os
import json
import subprocess
import re

project = os.popen('gcloud config get-value project').read().strip()
monitoring = monitoring_v3.MetricServiceClient()

def parse_tier(tier):
    """Parse tier string to extract vCPUs and memory"""
    # db-custom-4-15360 = 4 vCPUs, 15360 MB
    if match := re.match(r'db-custom-(\d+)-(\d+)', tier):
        return int(match.group(1)), int(match.group(2))
    # db-perf-optimized-N-2 = 2 vCPUs
    elif match := re.match(r'db-perf-optimized-N-(\d+)', tier):
        vcpus = int(match.group(1))
        # N-series: 4GB per vCPU
        return vcpus, vcpus * 4096
    return None, None

# Get instance configs via gcloud
result = subprocess.run(
    ['gcloud', 'sql', 'instances', 'list', '--format=json'],
    capture_output=True, text=True
)
instances = {}
for inst in json.loads(result.stdout):
    inst_name = inst['name']
    tier = inst['settings']['tier']
    vcpus, mem_mb = parse_tier(tier)
    instances[inst_name] = {
        'tier': tier,
        'vcpus': vcpus,
        'mem_gb': mem_mb / 1024 if mem_mb else None,
        'state': inst['state']
    }

# Get current CPU (last 5 min)
current = monitoring.list_time_series(name=f'projects/{project}',
      filter='metric.type="cloudsql.googleapis.com/database/cpu/utilization"',
      interval={'end_time': datetime.utcnow(), 'start_time': datetime.utcnow() - timedelta(minutes=5)}
  )

# Get 24h history
history_24h = monitoring.list_time_series(name=f'projects/{project}',
      filter='metric.type="cloudsql.googleapis.com/database/cpu/utilization"',
      interval={'end_time': datetime.utcnow(), 'start_time': datetime.utcnow() - timedelta(hours=24)}
  )

# Get 7d history
history_7d = monitoring.list_time_series(name=f'projects/{project}',
      filter='metric.type="cloudsql.googleapis.com/database/cpu/utilization"',
      interval={'end_time': datetime.utcnow(), 'start_time': datetime.utcnow() - timedelta(days=7)}
  )

# Build max maps
max_24h = {}
for r in history_24h:
    db_id = r.resource.labels['database_id']
    max_24h[db_id] = max(p.value.double_value for p in r.points)

max_7d = {}
for r in history_7d:
    db_id = r.resource.labels['database_id']
    max_7d[db_id] = max(p.value.double_value for p in r.points)

# Display results
print(f"{'Instance':<40} {'vCPU':<6} {'Mem(GB)':<8} {'Current':<9} {'24h Max':<9} {'7d Max':<9}")
print("-" * 90)

for r in current:
    db_id = r.resource.labels['database_id']
    inst_name = db_id.split(':')[1]
    curr = r.points[0].value.double_value
    m24h = max_24h.get(db_id, curr)
    m7d = max_7d.get(db_id, curr)

    inst_info = instances.get(inst_name, {})
    vcpus = inst_info.get('vcpus', '?')
    mem_gb = inst_info.get('mem_gb', '?')

    print(f"{inst_name:<40} {vcpus:<6} {mem_gb:<8.0f} {curr:>7.2%} {m24h:>7.2%} {m7d:>7.2%}")
