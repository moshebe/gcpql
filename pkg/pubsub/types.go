package pubsub

import "time"

// Severity levels for diagnostic findings.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

// Finding is a single diagnostic issue with suggested remediation.
type Finding struct {
	Severity Severity `json:"severity"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail"`
	Actions  []string `json:"actions"`
}

// Stats holds basic statistics over a metric time-series.
type Stats struct {
	Current float64 `json:"current"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

// SubscriptionSnapshot is one row in the check output.
type SubscriptionSnapshot struct {
	Name             string   `json:"name"`
	Backlog          int64    `json:"backlog"`
	OldestUnackedSec float64  `json:"oldest_unacked_sec"`
	ExpiredAckCount  int64    `json:"expired_ack_count"`
	DLQCount         int64    `json:"dlq_count"`
	Status           Severity `json:"status"`
	StatusReason     string   `json:"status_reason"`
}

// CheckResult is the full output of `pubsub check`.
type CheckResult struct {
	Project       string                 `json:"project"`
	Timestamp     time.Time              `json:"timestamp"`
	Subscriptions []SubscriptionSnapshot `json:"subscriptions"`
	Metadata      Metadata               `json:"metadata"`
}

// SubMetrics holds collected time-series data for one subscription (diagnose).
type SubMetrics struct {
	Backlog          Stats
	OldestUnackedSec Stats
	ExpiredAckCount  int64   // sum over window
	DLQCount         int64   // last value
	AckRatePerSec    float64 // sum(ack_message_count) / window_seconds
	PullErrorRate    float64 // error pull ops / total pull ops (0–1)
	PushErrorRate    float64 // error push requests / total push requests (0–1)
}

// TopicMetrics holds collected time-series data for the parent topic (diagnose).
type TopicMetrics struct {
	PublishRatePerSec float64
	PublishErrorRate  float64 // error publish ops / total publish ops (0–1)
	AvgMessageSizeB   float64 // average message size in bytes (0 if unavailable)
	Available         bool    // false when topic metrics could not be fetched
}

// DiagnoseData is the raw collected data passed to the rule engine.
type DiagnoseData struct {
	Project      string
	Subscription string
	TopicName    string
	Since        time.Duration
	Sub          SubMetrics
	Topic        TopicMetrics
	Metadata     Metadata
}

// DiagnoseResult is the rule-engine output.
type DiagnoseResult struct {
	Project      string    `json:"project"`
	Subscription string    `json:"subscription"`
	TopicName    string    `json:"topic,omitempty"`
	TimeWindow   string    `json:"time_window"`
	Findings     []Finding `json:"findings"`
}

// Metadata tracks collection quality.
type Metadata struct {
	MetricsCollected     int      `json:"metrics_collected"`
	MetricsNoData        int      `json:"metrics_no_data"`
	MetricsUnavailable   []string `json:"metrics_unavailable,omitempty"`
	CollectionDurationMS int64    `json:"collection_duration_ms"`
}
