# Cloud SQL System Insights Metrics Guide

## Overview
This guide documents Cloud SQL for PostgreSQL system insights metrics, their meanings, and how to use them for troubleshooting database performance issues.

## Metric Categories

### 1. CPU & Resource Utilization

#### CPU Utilization (`cloudsql.googleapis.com/database/cpu/utilization`)
**What it measures:** Current CPU usage as percentage of reserved CPU capacity

**Values:**
- Dashboard shows P99 and P50 percentiles
- 0-70%: Normal operating range
- 70-85%: Monitor closely, consider scaling
- 85-100%: Performance degradation, scale urgently

**Debugging tips:**
- Correlate with database load and query latency metrics
- Check connection count - high connections can drive CPU usage
- Enable Query Insights to identify expensive queries
- Consider vertical scaling (more vCPUs) or query optimization

#### Memory Utilization
**What it measures:** Available memory as percentage of total database memory

**Normal values:** 15-30% free memory is healthy

**Troubleshooting:**
- Low available memory: Increase instance size or tune shared_buffers
- Memory pressure causes disk swapping and performance degradation
- Check cache hit ratios to validate memory sizing

---

### 2. Connection Metrics

#### Peak Connections
**What it measures:** Ratio of peak connections to max_connections limit during period

**Normal values:**
- < 70%: Healthy headroom
- 70-90%: Monitor for growth
- > 90%: Risk of connection exhaustion

**Key insight:** Peak may briefly exceed max if instance was recently scaled

**Debugging tips:**
- High ratio: Implement connection pooling (PgBouncer, pgpool-II)
- Review application connection management
- Check for connection leaks (idle connections piling up)

#### Connections by Status
**What it measures:** Connection count grouped by state

**States:**
- `active`: Executing queries (normal)
- `idle`: Connected but not executing (normal between queries)
- `idle_in_transaction`: Transaction open but inactive (potential issue)
- `idle_in_transaction_aborted`: Transaction failed but not closed (leak)
- `disabled`, `fastpath_function_call`: Rare states

**Troubleshooting:**
- High `idle_in_transaction`: Application not closing transactions properly
- Growing `idle_in_transaction_aborted`: Error handling issues in application
- Many `idle`: Connection pooling would help
- Action: Query `pg_stat_activity` to identify culprit sessions and applications

#### New Connections Per Second (PostgreSQL 14+)
**What it measures:** Rate of new connection creation per database

**Normal values:** < 10/sec for most applications

**Troubleshooting:**
- High rate: Application reconnecting excessively
- Connection pooling missing or misconfigured
- Check application logs for connection errors
- Each new connection has overhead; sustained high rates hurt performance

#### Connection Count by Application Name
**What it measures:** Connections grouped by `application_name`

**Use case:** Identify which services/apps drive database load

**Debugging tip:** Cross-reference with CPU/query metrics to find problematic applications

---

### 3. Disk & Storage

#### Disk Utilization
**What it measures:** Latest disk usage as percentage of allocated storage

**Thresholds:**
- < 80%: Normal
- 80-90%: Plan capacity expansion
- > 90%: Critical - scale disk immediately

**Actions:**
- Increase disk size (automatic storage increase available)
- Identify large tables/databases consuming space
- Run VACUUM FULL on bloated tables

#### Disk Storage by Type
**What it measures:** Breakdown of disk usage

**Types:**
- `data`: Actual database data and indexes
- `binlog`: Binary logs for replication
- `tmp_data`: Temporary files (cleaned during maintenance)

**Notes:**
- PITR (point-in-time recovery) uses WAL logs stored in Cloud Storage, not counted here
- Temp data can exceed limits temporarily to prevent disk full

**Troubleshooting:**
- Large binlog: Adjust retention or promote replicas sooner
- High tmp_data: Complex queries with sorts/joins, tune work_mem

#### Disk Read/Write Operations
**What it measures:** Read and write operations per second

**Interpretation:**
- **Read ops:** Cache misses requiring disk access (lower is better)
- **Write ops:** ~1/sec even at idle due to system table updates

**Troubleshooting:**
- High read ops: Cache too small, consider larger instance with more memory
- Sustained high reads: Check block read count and cache hit ratio
- Compare with memory metrics to validate sizing decisions

---

### 4. Query Performance

#### Query Latency (requires Query Insights)
**What it measures:** Aggregated query execution time distribution (P99, P95, P50) by user/database

**Normal values:** Application-dependent; establish baseline

**Debugging tips:**
- Rising P99: Some queries degrading (bad execution plans, locks)
- Rising P50: Systemic issue (CPU, I/O, memory)
- Correlate with database load and CPU utilization
- Drill into Query Insights to find specific slow queries
- Check for missing indexes, table bloat, or lock contention

#### Database Load (Per Database/User/Client)
**What it measures:** Accumulated execution time including:
- CPU time
- I/O wait
- Lock wait
- Context switches

**Use case:** Reveals which database, user, or client consumes most resources

**Debugging approach:**
1. Identify high-load database/user
2. Cross-reference with query latency
3. Use Query Insights to find expensive queries
4. Examine execution plans and optimize

#### I/O Wait Breakdown (requires Query Insights)
**What it measures:** Separates read vs. write I/O wait time

**Troubleshooting:**
- High read wait: Undersized cache, consider scaling up
- High write wait: Disk throughput bottleneck, consider faster disk or batch writes
- Persistent I/O wait: Check for disk read/write ops correlation

#### Rows Processed Metrics
**What it measures:**
- `rows_fetched`: Result rows returned to client
- `rows_returned`: Rows scanned during query processing
- `rows_written`: INSERT/UPDATE/DELETE operations

**Key insight:** Large gap between returned vs. fetched indicates inefficient queries

**Example:** Query returns 10 rows but scans 1M rows = missing index

**Actions:**
- Analyze queries with high returned/fetched ratio
- Add indexes on filter/join columns
- Rewrite queries to be more selective

---

### 5. Transaction Health

#### Transaction ID Utilization
**What it measures:** Percentage of available transaction IDs consumed

**Critical thresholds:**
- < 80%: Normal
- 80-95%: Schedule VACUUM operations
- > 95%: Critical - risk of transaction ID wraparound

**What happens at 100%:** Database shuts down to prevent data corruption

**Troubleshooting:**
- Run VACUUM on all databases immediately
- Identify tables with old transaction IDs: `SELECT relname, age(relfrozenxid) FROM pg_class`
- Check for long-running transactions blocking VACUUM

#### Oldest Transaction by Age
**What it measures:** Age of oldest transaction preventing VACUUM from reclaiming space

**Normal values:** < 1 hour

**Troubleshooting:**
- Long-running transactions (hours/days) prevent cleanup
- Causes table bloat and transaction ID wraparound risk
- Query `pg_stat_activity` to find long transactions
- Terminate problematic transactions: `SELECT pg_terminate_backend(pid)`

#### Deadlock Count by Database
**What it measures:** Number of deadlocks per database

**Normal:** Occasional deadlocks are expected in concurrent systems

**Troubleshooting:**
- Frequent deadlocks: Application logic issue or lock contention
- Review PostgreSQL logs for deadlock details
- Redesign transaction ordering to acquire locks consistently
- Keep transactions short and acquire locks in same order

---

### 6. Data Access Patterns

#### Block Read Count
**What it measures:** Database blocks read from disk and cache per second

**Components:**
- Blocks from cache (heap hit)
- Blocks from disk (heap read)

**Key metric:** Cache hit ratio = cache reads / (cache + disk reads)

**Healthy cache hit ratio:** > 95%

**Troubleshooting:**
- Low hit ratio (< 90%): Undersized shared_buffers or insufficient memory
- Scale to larger instance or tune PostgreSQL memory settings
- Check if specific tables/indexes causing misses

#### Temp Data Metrics
**What it measures:**
- Temp bytes written per second
- Temp files created per second

**What creates temp data:**
- Large sorts exceeding work_mem
- Hash joins exceeding hash_mem
- Complex aggregations

**Troubleshooting:**
- High temp data: Queries need more memory for operations
- Increase work_mem (per-connection setting)
- Optimize queries to reduce sorting/hashing needs
- Consider larger instance for more total memory

#### Rows in Database by State (< 50 databases only)
**What it measures:** Live row count per database

**Use case:**
- Understand data growth trends
- Detect bloat (dead rows not vacuumed)
- Plan capacity

**Action:** Compare with disk usage to identify bloated databases

---

### 7. Replication & High Availability

#### Max Replica Byte Lag
**What it measures:** Maximum replication lag in bytes among external replicas

**Normal values:**
- < 10 MB: Minimal lag
- 10-100 MB: Some lag, monitor
- > 100 MB: Significant lag, investigate

**Troubleshooting:**
- High lag: Replica underpowered or network issues
- Check replica CPU and disk I/O
- Verify network bandwidth between primary and replica
- Consider scaling replica or optimizing write workload

#### WAL Archiving Success/Failure Count
**What it measures:** Write-ahead log archiving operations per minute

**Normal:** All archives succeed

**Troubleshooting:**
- Failures indicate backup/recovery issues
- Check Cloud Storage permissions
- Verify PITR configuration
- Review PostgreSQL logs for archiving errors

---

### 8. Network & Throughput

#### Ingress/Egress Bytes
**What it measures:** Network traffic in and out of instance

**Use case:** Bandwidth utilization and data transfer patterns

**Troubleshooting:**
- Sustained high ingress: Heavy write workload or bulk imports
- Sustained high egress: Large result sets returned to clients
- Optimize queries to return less data
- Use pagination or streaming for large datasets

---

## Dashboard Features

### Time Period Analysis
- Default: 1 day view
- Historical data: Available up to 30 days back
- Custom periods: Target specific incident timeframes
- Auto-refresh: Updates every minute

### Events Timeline
Tracks critical events and correlates them with metric changes:
- Instance restarts
- Failovers
- Maintenance windows
- Backups
- Configuration updates
- Replica operations
- Imports/exports

**Debugging use:** Correlate performance changes with system events

---

### 9. Advanced Performance Indicators

#### Sequential Scans vs Index Scans (Derived Metric)
**What it measures:** Ratio of table scans to index scans

**Available via:** `cloudsql.googleapis.com/database/postgresql/insights/aggregate/io_time`

**Interpretation:**
- High sequential scan rate relative to data size = missing indexes
- Sequential scans on large tables = performance killer
- Normal: Small tables (< 1000 rows) benefit from sequential scans

**Troubleshooting without DB access:**
- Correlate high disk read ops + high rows_returned/rows_fetched ratio
- Look for sudden increases in disk I/O coinciding with deployment
- Check if CPU spikes correlate with disk read spikes (scanning pattern)

#### Index Efficiency Indicators (Indirect Detection)

**Signal 1: High rows_returned vs rows_fetched ratio**
- Fetched 10 rows, returned 1M rows = full table scan likely
- Indicates missing or unused indexes
- **Metric:** Track ratio over time; sudden increases = new query pattern without index

**Signal 2: Disk read ops increasing without data growth**
- Baseline: Establish normal read ops rate
- Spike without corresponding data size increase = cache-inefficient queries
- Likely cause: Sequential scans bypassing indexes

**Signal 3: CPU + Disk I/O both high**
- CPU high alone = computation or many small queries
- Disk I/O high alone = large reads, possible index scans on large tables
- Both high = sequential scans processing large datasets

**Signal 4: Temp data generation**
- Large temp file creation = sorts/joins without indexes
- Query doing work that index could eliminate
- Look for spikes in temp bytes written coinciding with slow performance

#### Autovacuum Activity Metrics
**Metrics:**
- `cloudsql.googleapis.com/database/postgresql/vacuum_count`
- `cloudsql.googleapis.com/database/postgresql/autovacuum_count`

**What it measures:** VACUUM operations per minute

**Troubleshooting:**
- Frequent autovacuums on same table = high churn, possible bloat
- No autovacuum on growing table = bloat accumulating
- Autovacuum duration increasing = table bloat or blocking

**Correlation patterns:**
- High autovacuum + high disk writes = write-heavy workload
- High autovacuum + rising disk usage = bloat not being reclaimed (tune autovacuum)
- No autovacuum + high transaction ID utilization = autovacuum stuck or disabled

#### Lock Contention Metrics
**Metrics:**
- `cloudsql.googleapis.com/database/postgresql/insights/aggregate/lock_time`
- Deadlock count (already covered)

**What it measures:** Time spent waiting for locks

**Thresholds:**
- < 5% of database load: Normal
- 5-20%: Moderate contention, review query patterns
- > 20%: Severe contention, redesign needed

**Troubleshooting:**
- High lock time + normal CPU = queries blocking each other
- Increasing lock time over time = growing contention as data scales
- Lock time spikes + deadlocks = poorly ordered lock acquisition

#### Checkpoint and WAL Metrics
**Metrics:**
- `cloudsql.googleapis.com/database/postgresql/checkpoint_sync_latency`
- `cloudsql.googleapis.com/database/postgresql/transaction_count`

**What it measures:**
- Checkpoint sync latency: Time to sync dirty pages to disk
- Transaction rate: Commits per second

**Troubleshooting:**
- High checkpoint latency = I/O bottleneck or undersized disk
- Frequent checkpoints + high write rate = tune checkpoint settings
- Transaction rate drops during checkpoint = write stalls

#### Cache Efficiency Deep Dive
**Metrics:**
- Shared buffer hit ratio: `heap_blks_hit / (heap_blks_hit + heap_blks_read)`
- Index hit ratio: Similar calculation for index blocks

**Derived from:** Block read count metrics

**Targets:**
- Shared buffer hit ratio: > 99% for OLTP
- Index hit ratio: > 99% (indexes should stay in cache)
- Temp table hit ratio: Not applicable (temp tables always use work_mem)

**Poor cache patterns:**
- Hit ratio < 95%: Undersized instance or poor query patterns
- Decreasing hit ratio over time: Data outgrowing cache
- Hit ratio drops during specific hours: Batch jobs not using indexes

#### Buffer Pool Pressure
**Indicators (derived):**
- Rising disk read ops + stable working set size = cache thrashing
- High block reads from disk + low memory utilization = shared_buffers too small
- High temp data + available memory = work_mem too low per connection

**Troubleshooting:**
- Calculate: Read ops per second / total data size
- High ratio = inefficient data access (missing indexes or cache churn)

#### Write Amplification
**What it measures:** Ratio of actual disk writes to logical data written

**Calculated from:**
- Disk write ops / rows_written
- High ratio = excessive index maintenance, bloat, or WAL overhead

**Normal values:**
- OLTP: 2-5x (reasonable index overhead)
- Batch writes: 1-2x (efficient bulk operations)
- > 10x: Excessive indexes or bloat

**Troubleshooting:**
- High amplification + many indexes = remove unused indexes
- High amplification + low autovacuum = bloat accumulation

---

## Advanced Debugging Techniques (Metrics-Only)

### Technique 1: Query Pattern Change Detection

**Goal:** Identify when new query patterns cause performance issues

**Method:**
1. Establish baseline metrics (1 week):
   - Average CPU utilization
   - Average disk read ops
   - Average rows_returned/rows_fetched ratio
   - Average query latency P50/P99

2. Monitor for deviations:
   - > 50% increase in disk reads = new sequential scan likely
   - > 100% increase in rows_returned ratio = missing index
   - CPU stable but latency up = I/O bottleneck (cache/disk)

3. Correlate with deployments (Events Timeline)

**Output:** Identify exact time query pattern changed

### Technique 2: Index vs Sequential Scan Detection

**Goal:** Determine if queries are using indexes without DB access

**Indicators of missing indexes:**

✅ **Confirming pattern (all present = likely missing index):**
1. High disk read ops (> baseline)
2. High rows_returned/rows_fetched ratio (> 10:1)
3. High temp data generation
4. CPU + I/O both elevated
5. Query latency P99 increasing

✅ **Strong signal (3+ indicators):**
- Disk read spikes coincide with specific application activity
- Read ops increase linear with data growth (should be logarithmic with indexes)
- Cache hit ratio drops during specific query types

**How to validate:**
1. Enable Query Insights (if not already)
2. Correlate high-latency queries with disk I/O spikes
3. Check if problem queries appear after code deployments

### Technique 3: Table Bloat Detection

**Goal:** Identify bloated tables without connecting to database

**Indicators:**
1. **Disk usage grows but row count stable** (requires rows_in_database metric)
2. **Autovacuum frequency increases** on specific databases
3. **Disk read ops increase** without query pattern change
4. **Transaction ID utilization rising** (bloat prevents wraparound advancement)

**Calculation:**
- Expected size = rows × average_row_size
- Actual size from disk metrics
- Bloat ratio = actual / expected

**Action:**
- Schedule VACUUM FULL during maintenance window
- Tune autovacuum to be more aggressive
- Investigate high-churn tables

### Technique 4: Connection Pool Efficiency Analysis

**Goal:** Determine if connection pooling is working

**Good pattern:**
- Connection count stable
- New connections/sec low (< 1)
- Active connections fluctuate with load
- Idle connections stay within pool size

**Bad pattern:**
- Connection count grows steadily = leak
- High new connections/sec = no pooling or pool exhausted
- Many idle_in_transaction = app not releasing connections

**Metrics to track:**
- Connection count by status over time
- New connections per second trend
- Ratio of active/idle connections

### Technique 5: Cache Working Set Analysis

**Goal:** Determine if data fits in cache

**Method:**
1. Calculate cache size: Instance memory × 0.25 (typical shared_buffers)
2. Estimate working set: Data accessed in 1 hour
3. Compare: cache_size / working_set_size

**Ratios:**
- > 2x: Excellent, data fits in cache
- 1-2x: Good, hot data cached
- 0.5-1x: Marginal, frequent cache evictions
- < 0.5x: Poor, constant cache thrashing

**Validate with metrics:**
- Cache hit ratio (should be > 99% if working set fits)
- Disk read ops (should be low and stable)

### Technique 6: Query Latency Percentile Analysis

**Goal:** Distinguish between systemic issues vs outliers

**P50 rising:**
- Systemic degradation (all queries slower)
- Causes: CPU exhaustion, memory pressure, disk I/O saturation
- Action: Scale instance or optimize all queries

**P99 rising, P50 stable:**
- Specific queries degrading (outliers)
- Causes: Missing index, lock contention, bad execution plan
- Action: Identify and optimize specific queries

**P99 and P50 diverging:**
- Performance variance increasing
- Causes: Intermittent issues (locks, batch jobs, checkpoints)
- Action: Investigate time-based patterns

### Technique 7: Time-Based Pattern Recognition

**Goal:** Identify periodic performance issues

**Method:**
1. Plot metrics over 7 days with hourly granularity
2. Look for patterns:
   - Daily spikes at same time = scheduled job
   - Hourly patterns = periodic tasks
   - Random spikes = user-driven load
   - Gradual degradation = growing problem

**Examples:**
- CPU spike every day at 2 AM = backup or batch job
- Connections peak at 9 AM = user login storm
- Disk I/O high overnight = ETL or vacuum
- Memory pressure growing = leak or data growth

### Technique 8: Correlation Matrix Analysis

**Goal:** Find root cause by correlating metrics

**Common correlations:**

| Symptom | + | Correlate | = | Likely Cause |
|---------|---|-----------|---|--------------|
| High CPU | + | High connections | = | Connection storm |
| High CPU | + | High query latency | = | Slow queries |
| High CPU | + | Normal latency | = | Many fast queries |
| High disk reads | + | Low cache hit | = | Cache too small |
| High disk reads | + | High cache hit | = | Sequential scans |
| High latency | + | High lock time | = | Lock contention |
| High latency | + | High I/O wait | = | Disk bottleneck |
| High temp data | + | High disk reads | = | Missing indexes |
| High memory use | + | Low connections | = | Memory leak |
| High disk usage | + | Stable rows | = | Bloat |

### Technique 9: Capacity Headroom Analysis

**Goal:** Determine how close to limits

**For each resource, calculate headroom:**

```
CPU headroom = (100% - P99_CPU) / 100%
Memory headroom = Available_memory / Total_memory
Disk headroom = (Total_disk - Used_disk) / Total_disk
Connection headroom = (Max_connections - Peak_connections) / Max_connections
```

**Risk levels:**
- > 30% headroom: Comfortable
- 15-30%: Plan for scaling
- 5-15%: Scale soon
- < 5%: Scale immediately

**Action triggers:**
- < 20% headroom in 2+ resources = scale up
- < 30% headroom with growth trend = proactive scaling

### Technique 10: Saturation Point Detection

**Goal:** Identify which resource saturates first

**Method:**
1. Monitor all resources during load test or peak hours
2. Note which hits limit first:
   - CPU at 90%+ = CPU-bound
   - Memory available < 10% = Memory-bound
   - Disk IOPS maxed = I/O-bound
   - Connections near max = Connection-bound

**Optimization priority:**
1. Fix saturated resource first (biggest impact)
2. Scale that specific resource
3. Re-test to find next bottleneck

---

## Metric Collection Strategy

### Essential Metrics (Monitor Always)
- CPU Utilization (P50, P99)
- Memory Utilization
- Disk Utilization
- Connection Count
- Query Latency (P50, P99) [requires Query Insights]
- Disk Read/Write Ops

### High-Value Metrics (Monitor for Troubleshooting)
- Database Load by Database/User
- Connections by Status
- Rows Processed (fetched vs returned)
- Transaction ID Utilization
- Disk Storage by Type
- Block Read Count (for cache hit ratio)

### Specialized Metrics (Monitor for Specific Issues)
- Autovacuum Count (bloat investigation)
- Lock Time (contention analysis)
- Deadlock Count (concurrency issues)
- Temp Data Written (query efficiency)
- Checkpoint Latency (I/O health)
- Replication Lag (HA monitoring)

### Metric Retention
- 1-minute resolution: 30 days (Cloud Monitoring default)
- 1-hour aggregates: Store custom for trend analysis
- Baseline calculations: Retain 7-day rolling baseline

---

## Alerting Recommendations

### Critical Alerts (Page Immediately)
- Disk utilization > 90%
- CPU utilization P99 > 95% for > 10 min
- Transaction ID utilization > 95%
- Replication lag > 500 MB
- Connection count > 90% of max

### Warning Alerts (Investigate Soon)
- CPU utilization P99 > 85% for > 30 min
- Memory available < 15%
- Disk utilization > 80%
- Query latency P99 > 2x baseline
- Cache hit ratio < 95%
- Transaction ID utilization > 80%

### Trend Alerts (Plan Capacity)
- Disk usage growing > 5% per week
- Connection count increasing > 10% per week
- Query latency increasing > 20% over 2 weeks
- CPU utilization baseline rising

---

## Common Troubleshooting Workflows

### Workflow 1: Slow Query Performance
1. Check **Query Latency** - identify if P99 or P50 is elevated
2. Review **Database Load** - find which database/user consuming resources
3. Enable **Query Insights** if not already active
4. Check **CPU Utilization** - confirm if CPU-bound
5. Review **I/O Wait Breakdown** - identify if I/O-bound
6. Examine **Rows Processed** - find inefficient queries
7. **Action:** Optimize queries, add indexes, or scale instance

### Workflow 2: High CPU Utilization
1. Check **Connection Count** - too many connections?
2. Review **Database Load** - which component driving load?
3. Check **Query Latency** - slow queries causing CPU spike?
4. Review **Deadlock Count** - lock contention?
5. **Action:** Optimize queries, add connection pooling, or scale vertically

### Workflow 3: Storage Issues
1. Check **Disk Utilization** - capacity concern?
2. Review **Disk Storage by Type** - what's consuming space?
3. Check **Transaction ID Utilization** - bloat from unvacuumed tables?
4. Review **Oldest Transaction Age** - VACUUM being blocked?
5. **Action:** Scale disk, run VACUUM, or disable/re-enable PITR to clean WAL logs

### Workflow 4: Connection Exhaustion
1. Check **Peak Connections** - approaching limit?
2. Review **Connections by Status** - leaking connections?
3. Check **New Connections/Sec** - excessive reconnection?
4. Review **Connection Count by Application** - which app is culprit?
5. **Action:** Implement connection pooling, fix leaks, or increase max_connections

### Workflow 5: Memory Pressure
1. Check **Memory Utilization** - low available memory?
2. Review **Block Read Count** - high disk reads from cache misses?
3. Check **Temp Data Metrics** - queries spilling to disk?
4. Review **Disk Read Operations** - confirming I/O pressure?
5. **Action:** Scale to larger instance or tune shared_buffers/work_mem

### Workflow 6: Replication Lag
1. Check **Max Replica Byte Lag** - how far behind?
2. Review replica **CPU Utilization** - replica underpowered?
3. Check replica **Disk Read/Write Ops** - I/O bottleneck?
4. Review **WAL Archiving** - archiving failures?
5. **Action:** Scale replica, optimize writes, or investigate network

---

## Complete Metric API Reference

### Core Performance Metrics

| Metric Display Name | API Metric Type | Unit | Description |
|---------------------|-----------------|------|-------------|
| CPU Utilization | `cloudsql.googleapis.com/database/cpu/utilization` | % | CPU usage as percentage of reserved capacity |
| CPU Reserved Cores | `cloudsql.googleapis.com/database/cpu/reserved_cores` | cores | Number of vCPUs allocated |
| Memory Utilization | `cloudsql.googleapis.com/database/memory/utilization` | % | Memory usage percentage |
| Memory Total | `cloudsql.googleapis.com/database/memory/total_usage` | bytes | Total memory allocated |
| Memory Quota | `cloudsql.googleapis.com/database/memory/quota` | bytes | Memory limit for instance |

### Disk & Storage Metrics

| Metric | API Metric Type | Unit | Description |
|--------|-----------------|------|-------------|
| Disk Utilization | `cloudsql.googleapis.com/database/disk/utilization` | % | Disk usage percentage |
| Disk Quota | `cloudsql.googleapis.com/database/disk/quota` | bytes | Total disk size |
| Disk Bytes Used | `cloudsql.googleapis.com/database/disk/bytes_used` | bytes | Actual disk usage |
| Disk Read Ops | `cloudsql.googleapis.com/database/disk/read_ops_count` | ops/sec | Read operations per second |
| Disk Write Ops | `cloudsql.googleapis.com/database/disk/write_ops_count` | ops/sec | Write operations per second |
| Disk Read Bytes | `cloudsql.googleapis.com/database/disk/read_bytes_count` | bytes/sec | Bytes read per second |
| Disk Write Bytes | `cloudsql.googleapis.com/database/disk/write_bytes_count` | bytes/sec | Bytes written per second |

### Network Metrics

| Metric | API Metric Type | Unit | Description |
|--------|-----------------|------|-------------|
| Network Ingress | `cloudsql.googleapis.com/database/network/received_bytes_count` | bytes | Inbound network traffic |
| Network Egress | `cloudsql.googleapis.com/database/network/sent_bytes_count` | bytes | Outbound network traffic |
| Network Connections | `cloudsql.googleapis.com/database/network/connections` | count | Active network connections |

### PostgreSQL-Specific Metrics

| Metric | API Metric Type | Unit | Description |
|--------|-----------------|------|-------------|
| Connections | `cloudsql.googleapis.com/database/postgresql/num_backends` | count | Total active connections |
| Replication Lag | `cloudsql.googleapis.com/database/replication/replica_lag` | bytes | Replication byte lag |
| Transaction Count | `cloudsql.googleapis.com/database/postgresql/transaction_count` | count | Committed transactions |
| Transaction ID Utilization | `cloudsql.googleapis.com/database/postgresql/transaction_id_utilization` | % | Transaction ID usage |
| Insights Aggregate Execution Time | `cloudsql.googleapis.com/database/postgresql/insights/aggregate/execution_time` | μs | Query execution time |
| Insights Aggregate I/O Time | `cloudsql.googleapis.com/database/postgresql/insights/aggregate/io_time` | μs | I/O wait time |
| Insights Aggregate Lock Time | `cloudsql.googleapis.com/database/postgresql/insights/aggregate/lock_time` | μs | Lock wait time |
| Insights Aggregate Latencies | `cloudsql.googleapis.com/database/postgresql/insights/aggregate/latencies` | μs | Query latency distribution |
| Insights Rows Read | `cloudsql.googleapis.com/database/postgresql/insights/aggregate/row_count` | count | Rows read by queries |

### Vacuum & Maintenance Metrics

| Metric | API Metric Type | Unit | Description |
|--------|-----------------|------|-------------|
| Vacuum Count | `cloudsql.googleapis.com/database/postgresql/vacuum_count` | count | Manual VACUUM operations |
| Autovacuum Count | `cloudsql.googleapis.com/database/postgresql/autovacuum_count` | count | Autovacuum operations |

### Replication & HA Metrics

| Metric | API Metric Type | Unit | Description |
|--------|-----------------|------|-------------|
| Replica Lag | `cloudsql.googleapis.com/database/replication/replica_lag` | bytes | Bytes behind primary |
| Replica Lag Seconds | `cloudsql.googleapis.com/database/replication/replica_lag_seconds` | seconds | Time behind primary |
| Network Lag | `cloudsql.googleapis.com/database/replication/network_lag` | μs | Network delay to replica |

### Checkpoint & WAL Metrics

| Metric | API Metric Type | Unit | Description |
|--------|-----------------|------|-------------|
| Checkpoint Sync Latency | `cloudsql.googleapis.com/database/postgresql/checkpoint_sync_latency` | ms | Time to sync checkpoint |
| Checkpoint Write Latency | `cloudsql.googleapis.com/database/postgresql/checkpoint_write_latency` | ms | Time to write checkpoint |

### Instance State Metrics

| Metric | API Metric Type | Unit | Description |
|--------|-----------------|------|-------------|
| Instance State | `cloudsql.googleapis.com/database/state` | enum | Running, stopped, etc. |
| Up | `cloudsql.googleapis.com/database/up` | bool | Instance availability |
| Uptime | `cloudsql.googleapis.com/database/uptime` | seconds | Time since last restart |

### Query Insights Metrics (Requires Query Insights Enabled)

| Metric | API Metric Type | Labels | Description |
|--------|-----------------|--------|-------------|
| Query Execution Time | `cloudsql.googleapis.com/database/postgresql/insights/perquery/execution_time` | query_hash, user, database | Per-query execution time |
| Query Row Count | `cloudsql.googleapis.com/database/postgresql/insights/perquery/row_count` | query_hash, user, database | Rows processed per query |
| Query I/O Time | `cloudsql.googleapis.com/database/postgresql/insights/perquery/io_time` | query_hash, user, database | I/O wait per query |
| Query Lock Time | `cloudsql.googleapis.com/database/postgresql/insights/perquery/lock_time` | query_hash, user, database | Lock wait per query |
| Query Calls | `cloudsql.googleapis.com/database/postgresql/insights/perquery/calls` | query_hash, user, database | Query execution count |

### Useful Labels for Filtering

All metrics support these labels:
- `resource.project_id`: GCP project
- `resource.database_id`: Instance name (format: `project:instance`)
- `resource.region`: GCP region
- `metric.database`: Database name (PostgreSQL metrics only)

Query Insights metrics additional labels:
- `metric.user`: PostgreSQL user
- `metric.client_addr`: Client IP address
- `metric.query_hash`: Query fingerprint hash

---

## Programmatic Metric Collection

### Using Cloud Monitoring API (Python)

```python
from google.cloud import monitoring_v3
from datetime import datetime, timedelta

def get_metric(project, instance, metric_type, hours=24):
    """Fetch metric data for CloudSQL instance"""
    client = monitoring_v3.MetricServiceClient()

    interval = monitoring_v3.TimeInterval({
        "end_time": datetime.utcnow(),
        "start_time": datetime.utcnow() - timedelta(hours=hours)
    })

    # Format: project:instance
    database_id = f"{project}:{instance}"

    results = client.list_time_series(
        request={
            "name": f"projects/{project}",
            "filter": f'metric.type="{metric_type}" AND resource.labels.database_id="{database_id}"',
            "interval": interval,
            "view": monitoring_v3.ListTimeSeriesRequest.TimeSeriesView.FULL
        }
    )

    return results

# Example: Get CPU utilization
cpu_data = get_metric(
    "my-project",
    "my-instance",
    "cloudsql.googleapis.com/database/cpu/utilization",
    hours=24
)

for series in cpu_data:
    for point in series.points:
        print(f"{point.interval.end_time}: {point.value.double_value:.2%}")
```

### Aggregation Functions

```python
def get_metric_stats(time_series):
    """Calculate P50, P99, max, avg from time series"""
    values = [p.value.double_value for ts in time_series for p in ts.points]

    if not values:
        return None

    values.sort()
    n = len(values)

    return {
        "min": values[0],
        "max": values[-1],
        "avg": sum(values) / n,
        "p50": values[n // 2],
        "p95": values[int(n * 0.95)],
        "p99": values[int(n * 0.99)]
    }

# Get stats for last 24h
cpu = get_metric(project, instance, "cloudsql.googleapis.com/database/cpu/utilization")
stats = get_metric_stats(cpu)
print(f"CPU P99: {stats['p99']:.2%}, Avg: {stats['avg']:.2%}")
```

### Multi-Metric Collection for Diagnosis

```python
def diagnose_instance(project, instance, hours=24):
    """Collect all key metrics for diagnosis"""
    metrics = {
        "cpu": "cloudsql.googleapis.com/database/cpu/utilization",
        "memory": "cloudsql.googleapis.com/database/memory/utilization",
        "disk_util": "cloudsql.googleapis.com/database/disk/utilization",
        "disk_read_ops": "cloudsql.googleapis.com/database/disk/read_ops_count",
        "disk_write_ops": "cloudsql.googleapis.com/database/disk/write_ops_count",
        "connections": "cloudsql.googleapis.com/database/postgresql/num_backends",
        "txid_util": "cloudsql.googleapis.com/database/postgresql/transaction_id_utilization"
    }

    results = {}
    for name, metric_type in metrics.items():
        data = get_metric(project, instance, metric_type, hours)
        results[name] = get_metric_stats(data)

    return results

# Diagnose and suggest actions
diagnosis = diagnose_instance("my-project", "my-instance")

if diagnosis["cpu"]["p99"] > 0.85:
    print("⚠️  High CPU: P99 > 85%")
    if diagnosis["connections"]["max"] > 100:
        print("  → High connection count detected. Consider connection pooling.")
    if diagnosis["disk_read_ops"]["p99"] > 1000:
        print("  → High disk reads. Possible missing indexes or cache too small.")

if diagnosis["disk_util"]["max"] > 0.90:
    print("⚠️  Critical disk usage > 90%")
    print("  → Scale disk immediately or run VACUUM to reclaim space.")

if diagnosis["txid_util"]["max"] > 0.80:
    print("⚠️  High transaction ID utilization")
    print("  → Run VACUUM on all databases to prevent wraparound.")
```

### Query Insights API Access

```python
def get_top_queries(project, instance, database, hours=24):
    """Get top queries by execution time (requires Query Insights)"""
    metric_type = "cloudsql.googleapis.com/database/postgresql/insights/perquery/execution_time"

    client = monitoring_v3.MetricServiceClient()

    interval = monitoring_v3.TimeInterval({
        "end_time": datetime.utcnow(),
        "start_time": datetime.utcnow() - timedelta(hours=hours)
    })

    database_id = f"{project}:{instance}"

    results = client.list_time_series(
        request={
            "name": f"projects/{project}",
            "filter": f'metric.type="{metric_type}" '
                     f'AND resource.labels.database_id="{database_id}" '
                     f'AND metric.labels.database="{database}"',
            "interval": interval,
        }
    )

    queries = []
    for ts in results:
        query_hash = ts.metric.labels.get("query_hash", "unknown")
        user = ts.metric.labels.get("user", "unknown")
        total_time = sum(p.value.double_value for p in ts.points)

        queries.append({
            "query_hash": query_hash,
            "user": user,
            "total_execution_time_us": total_time
        })

    # Sort by execution time
    queries.sort(key=lambda q: q["total_execution_time_us"], reverse=True)
    return queries[:10]  # Top 10

# Find slow queries
slow_queries = get_top_queries("my-project", "my-instance", "mydb")
for q in slow_queries:
    print(f"Query {q['query_hash']} by {q['user']}: {q['total_execution_time_us']/1000:.2f}ms total")
```

### Correlation Analysis

```python
def correlation_analysis(project, instance, hours=24):
    """Identify correlations between metrics"""

    # Collect metrics
    cpu = get_metric(project, instance, "cloudsql.googleapis.com/database/cpu/utilization", hours)
    disk_reads = get_metric(project, instance, "cloudsql.googleapis.com/database/disk/read_ops_count", hours)
    connections = get_metric(project, instance, "cloudsql.googleapis.com/database/postgresql/num_backends", hours)

    cpu_stats = get_metric_stats(cpu)
    disk_stats = get_metric_stats(disk_reads)
    conn_stats = get_metric_stats(connections)

    # Pattern detection
    patterns = []

    if cpu_stats["p99"] > 0.80 and disk_stats["p99"] > 1000:
        patterns.append({
            "pattern": "High CPU + High Disk Reads",
            "likely_cause": "Sequential scans or missing indexes",
            "recommendation": "Enable Query Insights and identify queries with high I/O"
        })

    if cpu_stats["p99"] > 0.80 and conn_stats["max"] > 200:
        patterns.append({
            "pattern": "High CPU + Many Connections",
            "likely_cause": "Connection storm or missing connection pooling",
            "recommendation": "Implement PgBouncer or reduce connection count"
        })

    if disk_stats["max"] < 100 and cpu_stats["p99"] > 0.80:
        patterns.append({
            "pattern": "High CPU + Low Disk I/O",
            "likely_cause": "CPU-bound queries (complex computations)",
            "recommendation": "Optimize query logic or scale to larger instance"
        })

    return patterns

# Run analysis
patterns = correlation_analysis("my-project", "my-instance")
for p in patterns:
    print(f"\n🔍 Pattern: {p['pattern']}")
    print(f"   Cause: {p['likely_cause']}")
    print(f"   Action: {p['recommendation']}")
```

### Automated Alerting Logic

```python
def check_health(project, instance):
    """Automated health check with severity levels"""
    diagnosis = diagnose_instance(project, instance, hours=1)

    alerts = []

    # Critical alerts
    if diagnosis["disk_util"]["max"] > 0.90:
        alerts.append(("CRITICAL", "Disk usage > 90%", "Scale disk immediately"))

    if diagnosis["txid_util"]["max"] > 0.95:
        alerts.append(("CRITICAL", "Transaction ID > 95%", "Run VACUUM urgently"))

    if diagnosis["cpu"]["p99"] > 0.95:
        alerts.append(("CRITICAL", "CPU P99 > 95%", "Scale instance or reduce load"))

    # Warning alerts
    if diagnosis["cpu"]["p99"] > 0.85:
        alerts.append(("WARNING", "CPU P99 > 85%", "Investigate query performance"))

    if diagnosis["memory"]["avg"] > 0.85:
        alerts.append(("WARNING", "Memory > 85%", "Consider scaling instance"))

    if diagnosis["disk_util"]["max"] > 0.80:
        alerts.append(("WARNING", "Disk usage > 80%", "Plan capacity expansion"))

    # Info alerts
    if diagnosis["connections"]["p99"] > 0.70 * diagnosis["connections"]["max"]:
        alerts.append(("INFO", "Connections > 70% of max", "Consider connection pooling"))

    return alerts

# Monitor instance
alerts = check_health("my-project", "my-instance")
for severity, message, action in alerts:
    icon = "🔴" if severity == "CRITICAL" else "🟡" if severity == "WARNING" else "🔵"
    print(f"{icon} [{severity}] {message}")
    print(f"   → {action}\n")
```

---

## Prerequisites for Full Metrics

**Query Insights must be enabled** for these metrics:
- Query Latency
- Database Load breakdowns
- I/O Wait Breakdown
- Rows Processed metrics

**Enable via:**
```sql
-- Enable query insights
ALTER DATABASE mydb SET query_insights = on;
```

Or via gcloud:
```bash
gcloud sql instances patch INSTANCE_NAME \
  --database-flags=query_insights=on
```

---

## Next Steps

This guide provides the foundation for building an automated troubleshooting agent. The agent should:
1. Fetch relevant metrics based on symptoms
2. Apply troubleshooting workflows
3. Correlate multiple metrics to identify root cause
4. Suggest specific remediation actions

**Example agent workflow:**
- Input: "Database is slow"
- Fetch: CPU, Query Latency, Database Load, Connection Count
- Analyze: Identify bottleneck (CPU-bound, I/O-bound, connection exhaustion)
- Output: Root cause and specific remediation steps
