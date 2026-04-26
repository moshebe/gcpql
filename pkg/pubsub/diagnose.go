package pubsub

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
)

// Diagnose runs all rules against DiagnoseData and returns a DiagnoseResult with
// findings sorted CRITICAL → WARNING → INFO.
func Diagnose(data DiagnoseData) DiagnoseResult {
	dr := DiagnoseResult{
		Project:      data.Project,
		Subscription: data.Subscription,
		TopicName:    data.TopicName,
		TimeWindow:   formatDuration(data.Since),
	}

	var findings []Finding
	add := func(f Finding) { findings = append(findings, f) }

	diagOldestUnacked(data, add)
	diagDLQ(data, add)
	diagExpiredAck(data, add)
	diagBacklog(data, add)
	diagPushErrors(data, add)
	diagPullErrors(data, add)
	diagPublishErrors(data, add)
	diagNoConsumer(data, add)
	diagLargeMessages(data, add)

	sortFindings(findings)
	dr.Findings = findings
	return dr
}

func diagOldestUnacked(data DiagnoseData, add func(Finding)) {
	age := data.Sub.OldestUnackedSec.Current
	if age > 3600 {
		add(Finding{
			Severity: SeverityCritical,
			Title:    "Subscription Severely Backlogged",
			Detail:   fmt.Sprintf("Oldest unacked message: %s (threshold: 1h)", fmtAge(age)),
			Actions: []string{
				"Check consumer logs for crashes or processing failures",
				"Scale out consumer instances to increase throughput",
				"Verify consumer can connect to the subscription endpoint",
				"Check if ack deadline is too short for processing time — consider extending it",
			},
		})
		return
	}
	if age > 600 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Consumer Falling Behind",
			Detail:   fmt.Sprintf("Oldest unacked message: %s (threshold: 10m)", fmtAge(age)),
			Actions: []string{
				"Monitor trend — if growing, consider scaling out consumers",
				"Check consumer processing latency for bottlenecks",
				"Review --since window to see if this is sustained or a spike",
			},
		})
	}
}

func diagDLQ(data DiagnoseData, add func(Finding)) {
	if data.Sub.DLQCount <= 0 {
		return
	}
	add(Finding{
		Severity: SeverityCritical,
		Title:    "Dead Letter Queue Has Messages",
		Detail:   fmt.Sprintf("%d messages in DLQ — consumers are failing to process them", data.Sub.DLQCount),
		Actions: []string{
			"Inspect DLQ messages to identify the failure pattern",
			"Check consumer logs for errors around message processing",
			"Fix the root cause in the consumer, then replay DLQ messages",
			"Review max delivery attempts setting on the subscription",
		},
	})
}

func diagExpiredAck(data DiagnoseData, add func(Finding)) {
	count := data.Sub.ExpiredAckCount
	if count <= 0 {
		return
	}
	add(Finding{
		Severity: SeverityWarning,
		Title:    "Consumers Missing Ack Deadline",
		Detail:   fmt.Sprintf("%d ack deadline expirations in window — consumers received but did not ack in time", count),
		Actions: []string{
			"Increase the subscription ack deadline if processing takes longer than the current setting",
			"Use modifyAckDeadline to extend the deadline while processing long messages",
			"Optimize consumer processing path to reduce per-message latency",
			"Check for consumer thread starvation or GC pauses",
		},
	})
}

func diagBacklog(data DiagnoseData, add func(Finding)) {
	backlog := data.Sub.Backlog.Current
	if backlog > 10000 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Large Message Backlog",
			Detail:   fmt.Sprintf("%.0f undelivered messages in backlog", backlog),
			Actions: []string{
				"Scale out consumers to increase throughput",
				"Check if oldest_unacked_message_age is growing — sustained growth means consumers are falling behind",
				"Verify there are no processing errors causing redelivery loops",
			},
		})
	} else if backlog > 1000 {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Elevated Message Backlog",
			Detail:   fmt.Sprintf("%.0f undelivered messages — above 1k threshold", backlog),
			Actions: []string{
				"Monitor trend with --since 6h to determine if backlog is growing",
				"Consider adding consumers if sustained above this level",
			},
		})
	}
}

func diagPushErrors(data DiagnoseData, add func(Finding)) {
	rate := data.Sub.PushErrorRate
	if rate <= 0 {
		return
	}
	if rate > 0.01 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Push Delivery Errors",
			Detail:   fmt.Sprintf("%.1f%% of push requests are failing", rate*100),
			Actions: []string{
				"Check the push endpoint URL is correct and reachable",
				"Verify the push endpoint is returning 2xx for successful processing",
				"Check endpoint authentication (OIDC token or service account)",
				"Review push endpoint logs for error details",
			},
		})
	}
}

func diagPullErrors(data DiagnoseData, add func(Finding)) {
	rate := data.Sub.PullErrorRate
	if rate > 0.01 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Pull Errors Detected",
			Detail:   fmt.Sprintf("%.1f%% of pull operations are returning errors", rate*100),
			Actions: []string{
				"Check consumer IAM permissions (roles/pubsub.subscriber required)",
				"Verify network connectivity between consumer and PubSub endpoint",
				"Check for quota exhaustion in Cloud Console",
			},
		})
	}
}

func diagPublishErrors(data DiagnoseData, add func(Finding)) {
	if !data.Topic.Available {
		return
	}
	rate := data.Topic.PublishErrorRate
	if rate > 0.01 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Topic Publish Errors",
			Detail:   fmt.Sprintf("%.1f%% of publish operations to topic %q are failing", rate*100, data.TopicName),
			Actions: []string{
				"Check publisher IAM permissions (roles/pubsub.publisher required)",
				"Verify topic exists and has not been deleted",
				"Check for publisher quota exhaustion",
				"Review publisher logs for error codes",
			},
		})
	}
}

func diagNoConsumer(data DiagnoseData, add func(Finding)) {
	backlog := data.Sub.Backlog.Current
	ackRate := data.Sub.AckRatePerSec
	if backlog > 0 && ackRate == 0 {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "No Active Consumer",
			Detail:   fmt.Sprintf("%.0f messages in backlog but ack rate is zero in this window", backlog),
			Actions: []string{
				"Verify consumer service is running and connected to this subscription",
				"Check if consumer deployment was scaled to zero or crashed",
				"For push subscriptions, verify the push endpoint is configured",
			},
		})
	}
}

func diagLargeMessages(data DiagnoseData, add func(Finding)) {
	if !data.Topic.Available {
		return
	}
	avgBytes := data.Topic.AvgMessageSizeB
	if avgBytes > 524288 { // 512 KB
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Large Messages Detected",
			Detail:   fmt.Sprintf("Average message size: %.0f KB — approaching PubSub 10 MB limit", avgBytes/1024),
			Actions: []string{
				"Consider storing large payloads in GCS and publishing only a reference URL",
				"Compress message payloads before publishing",
				"Review if the full payload is necessary in the message vs. the downstream storage",
			},
		})
	}
}

func sortFindings(fs []Finding) {
	order := map[Severity]int{SeverityCritical: 0, SeverityWarning: 1, SeverityInfo: 2}
	sort.SliceStable(fs, func(i, j int) bool {
		return order[fs[i].Severity] < order[fs[j].Severity]
	})
}

// FormatDiagnoseJSON writes a DiagnoseResult as indented JSON.
func FormatDiagnoseJSON(w io.Writer, dr DiagnoseResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(dr)
}

// FormatDiagnoseTable writes a DiagnoseResult as human-readable text.
func FormatDiagnoseTable(w io.Writer, dr DiagnoseResult) error {
	fmt.Fprintf(w, "Subscription: %s\n", dr.Subscription)
	if dr.TopicName != "" {
		fmt.Fprintf(w, "Topic:        %s\n", dr.TopicName)
	}
	fmt.Fprintf(w, "Project:      %s\n", dr.Project)
	fmt.Fprintf(w, "Time Window:  %s\n\n", dr.TimeWindow)

	if len(dr.Findings) == 0 {
		fmt.Fprintln(w, "✓  No issues detected.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.SetTitle(fmt.Sprintf("FINDINGS (%d)", len(dr.Findings)))
	t.AppendHeader(table.Row{"Severity", "Title", "Detail"})
	for _, f := range dr.Findings {
		t.AppendRow(table.Row{string(f.Severity), f.Title, f.Detail})
	}
	t.Render()
	fmt.Fprintln(w)

	for _, f := range dr.Findings {
		fmt.Fprintf(w, "%s %s\n", severityIcon(f.Severity), f.Title)
		fmt.Fprintf(w, "   %s\n", f.Detail)
		for _, a := range f.Actions {
			fmt.Fprintf(w, "   → %s\n", a)
		}
		fmt.Fprintln(w)
	}
	return nil
}

func severityIcon(s Severity) string {
	switch s {
	case SeverityCritical:
		return "🔴"
	case SeverityWarning:
		return "🟡"
	default:
		return "ℹ️ "
	}
}
