# PubSub Diagnosis — Design Spec

**Date:** 2026-04-24
**Status:** Approved

## Overview

Add `gcpql pubsub` with two subcommands:

- `gcpql pubsub check <project>` — project-wide health snapshot of all subscriptions
- `gcpql pubsub diagnose <subscription>` — deep-dive diagnosis of one subscription + its parent topic

Both commands use the existing `monitoring.Client` (PromQL over Cloud Monitoring) with no new auth scopes required.

---

## Command Interface

```
gcpql pubsub check <project-id> [--since 1h] [--format table|json] [--top 5]
gcpql pubsub diagnose <subscription> [--project <id>] [--since 1h] [--format table|json]
```

### `pubsub check <project>`

- Fetches a snapshot of all subscriptions in the project via Cloud Monitoring
- Output: summary table (one row per subscription) + "Top offenders" section
- Summary table columns: `Subscription`, `Backlog`, `Oldest Unacked`, `Exp. Ack`, `DLQ`, `Status`
- Status computed as:
  - CRIT: `oldest_unacked_age > 60m` OR `dlq_count > 0 and growing`
  - WARN: `oldest_unacked_age > 10m` OR `backlog > 10,000` OR `expired_ack_deadlines > 0`
  - OK: otherwise
- Top offenders: up to `--top N` (default 5) worst subscriptions, one-line reason each

### `pubsub diagnose <subscription>`

Subscription ID formats accepted:
- `my-sub` — requires `--project` or `config.ResolveProject()`
- `projects/my-project/subscriptions/my-sub` — GCP canonical; project extracted directly

Output: load summary table + findings list (CRITICAL/WARNING/INFO with Actions), matching `cloudsql diagnose` style.

---

## Package Layout

```
cmd/
  pubsub.go              # parent cobra command, registers subcommands
  pubsub_check.go        # check flags, runPubSubCheck()
  pubsub_diagnose.go     # diagnose flags, runPubSubDiagnose()

pkg/pubsub/
  types.go               # all shared types (see Types section)
  subscription.go        # ParseSubscriptionID() — ID parsing + project resolution
  check_collector.go     # CollectCheckMetrics() — project-wide snapshot
  diagnose_collector.go  # CollectDiagnoseMetrics() — single sub + topic time-series
  diagnose.go            # Diagnose() — rule engine → []Finding + FormatDiagnose*
  formatter.go           # FormatCheckTable, FormatCheckJSON
```

---

## Types

```go
// pkg/pubsub/types.go

type Severity string
const (
    SeverityCritical Severity = "CRITICAL"
    SeverityWarning  Severity = "WARNING"
    SeverityInfo     Severity = "INFO"
)

type Finding struct {
    Severity Severity
    Title    string
    Detail   string
    Actions  []string
}

// SubscriptionSnapshot is one row in the check output.
type SubscriptionSnapshot struct {
    Name              string
    Backlog           int64
    OldestUnackedSec  float64
    ExpiredAckCount   int64
    DLQCount          int64
    Status            Severity
    StatusReason      string
}

// CheckResult is the full output of `pubsub check`.
type CheckResult struct {
    Project       string
    Timestamp     time.Time
    Subscriptions []SubscriptionSnapshot
    Metadata      Metadata
}

// SubMetrics holds time-series data for one subscription (diagnose).
type SubMetrics struct {
    Backlog          Stats   // num_undelivered_messages
    OldestUnackedSec Stats   // oldest_unacked_message_age
    ExpiredAckCount  int64   // expired_ack_deadlines_count (sum over window)
    DLQCount         int64   // dead_letter_message_count (last value)
    AckRate          Stats   // ack_message_count rate
    PullErrorRate    float64 // pull errors / total pull ops
    PushErrorRate    float64 // push errors / total push requests
}

// TopicMetrics holds time-series data for the parent topic (diagnose).
type TopicMetrics struct {
    PublishRate      Stats   // send_message_operation_count
    PublishErrorRate float64 // error send ops / total send ops
    MessageSizeP99   float64 // message_sizes histogram P99 (bytes)
    Available        bool    // false if topic metrics not found
}

// DiagnoseData is the raw collected data for one subscription.
type DiagnoseData struct {
    Project      string
    Subscription string
    Topic        string   // derived from metric label
    Since        time.Duration
    Sub          SubMetrics
    TopicMetrics TopicMetrics
    Metadata     Metadata
}

// DiagnoseResult is the rule-engine output.
type DiagnoseResult struct {
    Project      string
    Subscription string
    Topic        string
    TimeWindow   string
    Findings     []Finding
}

// Stats holds basic statistics over a metric time-series.
type Stats struct {
    Current float64
    Min     float64
    Max     float64
    P50     float64
    P99     float64
}

// Metadata tracks collection quality.
type Metadata struct {
    MetricsCollected     int
    MetricsNoData        int
    CollectionDurationMS int64
}
```

---

## Metrics Collected

### Subscription metrics (both check and diagnose)

| Metric | Use |
|---|---|
| `pubsub.googleapis.com/subscription/num_undelivered_messages` | backlog count |
| `pubsub.googleapis.com/subscription/oldest_unacked_message_age` | consumer lag (seconds) |
| `pubsub.googleapis.com/subscription/expired_ack_deadlines_count` | consumer deadline misses |
| `pubsub.googleapis.com/subscription/dead_letter_message_count` | DLQ size |
| `pubsub.googleapis.com/subscription/ack_message_count` | ack throughput |
| `pubsub.googleapis.com/subscription/pull_message_operation_count` | pull ops (by response_code) |
| `pubsub.googleapis.com/subscription/push_request_count` | push requests (by response_code) |

### Topic metrics (diagnose only)

| Metric | Use |
|---|---|
| `pubsub.googleapis.com/topic/send_message_operation_count` | publish rate |
| `pubsub.googleapis.com/topic/send_request_count` | publish errors |
| `pubsub.googleapis.com/topic/message_sizes` | message size distribution (P99) |

---

## Data Flow

### check

```
runPubSubCheck()
  → monitoring.Client.QueryTimeSeries() × N metrics (parallel goroutines, one per metric)
  → each query returns all series for the project (one series per subscription label)
  → merge by subscription_id label → []SubscriptionSnapshot
  → compute Status per snapshot
  → sort by Status (CRIT first), then by OldestUnackedSec desc
  → FormatCheckTable / FormatCheckJSON
```

### diagnose

```
runPubSubDiagnose()
  → ParseSubscriptionID() → (project, subscriptionName)
  → CollectDiagnoseMetrics()
      goroutine group A: subscription metrics (7 queries, parallel)
      goroutine group B: topic metrics (3 queries, parallel)
      → both complete → DiagnoseData
  → Diagnose(data) → DiagnoseResult with []Finding
  → FormatDiagnoseTable / FormatDiagnoseJSON
```

---

## Diagnose Rules

| Severity | Condition | Title |
|---|---|---|
| CRITICAL | `oldest_unacked_age > 3600s (1h)` | Subscription Severely Backlogged |
| CRITICAL | `dlq_count > 0 and delta > 0 in window` | Dead Letter Queue Growing |
| WARNING | `oldest_unacked_age > 600s (10m)` | Consumer Falling Behind |
| WARNING | `backlog > 10,000 messages` | Large Message Backlog |
| WARNING | `expired_ack_deadlines rate > 10/min` | Consumers Missing Ack Deadline (sustained) |
| WARNING | `expired_ack_deadlines > 0` | Consumers Missing Ack Deadline |
| WARNING | `push_error_rate > 1%` | Push Delivery Errors |
| WARNING | `pull_error_rate > 1%` | Pull Errors Detected |
| WARNING | `publish_error_rate > 1%` | Topic Publish Errors |
| INFO | `backlog > 1,000 messages` | Elevated Message Backlog |
| INFO | `message_size_p99 > 524288 (512 KB)` | Large Messages Detected |
| INFO | `ack_rate == 0 and backlog > 0` | No Active Consumer |

---

## Error Handling

- Gerund form: `"fetching subscription metrics: %w"` — never `"failed to fetch ..."`
- Per-metric failures: log warning, set `Metadata.MetricsNoData++`, continue (graceful degradation)
- No metric data for subscription: emit INFO finding — "No metric data — subscription may be new or inactive"
- Auth: reuse `monitoring.Client` (existing `monitoring.read` scope covers PubSub metrics)
- Project resolution: `internal/config.ResolveProject()` (same as `cloudsql diagnose`)

---

## Testing

- Unit tests for `ParseSubscriptionID()` — all three ID formats + error cases
- Table-driven tests for each diagnose rule in `diagnose_test.go` (construct `DiagnoseData`, assert `Finding` produced)
- Tests for `FormatCheckTable` output structure
- Mock `monitoring.Client` via `NewClientForTesting()` (existing pattern)
- No integration tests

---

## Out of Scope

- `gcpql pubsub list` — not planned for this iteration
- Snapshot seek metrics
- Per-region breakdown in check output
- Topic-only diagnosis (no `pubsub diagnose --type topic`)
