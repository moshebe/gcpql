package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moshebe/gcpql/pkg/monitoring"
	"github.com/moshebe/gcpql/pkg/pubsub"
	"github.com/moshebe/gcpql/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	pubsubCheckSince  string
	pubsubCheckFormat string
	pubsubCheckTop    int
)

var pubsubCheckCmd = &cobra.Command{
	Use:   "check <project-id>",
	Short: "Check PubSub subscription health across a project",
	Long: `Fetch a health snapshot of all PubSub subscriptions in the project.
Shows backlog size, oldest unacked message age, expired ack deadlines, and DLQ counts.

Examples:
  gcpql pubsub check my-project
  gcpql pubsub check my-project --since 6h
  gcpql pubsub check my-project --format json
  gcpql pubsub check my-project --top 10`,
	Args: cobra.ExactArgs(1),
	RunE: runPubSubCheck,
}

func init() {
	pubsubCmd.AddCommand(pubsubCheckCmd)
	pubsubCheckCmd.Flags().StringVar(&pubsubCheckSince, "since", "1h", "Time window for metrics (e.g. 1h, 24h, 7d)")
	pubsubCheckCmd.Flags().StringVar(&pubsubCheckFormat, "format", "table", "Output format: table or json")
	pubsubCheckCmd.Flags().IntVar(&pubsubCheckTop, "top", 5, "Number of worst subscriptions to highlight")
}

func runPubSubCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	project := args[0]

	start, end, err := timerange.Parse(pubsubCheckSince, "")
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}
	since := end.Sub(start)

	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating monitoring client: %w", err)
	}

	opts := pubsub.CheckOptions{
		Project: project,
		Since:   since,
		Top:     pubsubCheckTop,
	}

	result, err := pubsub.CollectCheckMetrics(ctx, monClient, opts)
	if err != nil {
		return fmt.Errorf("collecting metrics: %w", err)
	}

	switch pubsubCheckFormat {
	case "json":
		return pubsub.FormatCheckJSON(os.Stdout, result)
	case "table":
		return pubsub.FormatCheckTable(os.Stdout, result, pubsubCheckTop)
	default:
		return fmt.Errorf("unknown format: %s", pubsubCheckFormat)
	}
}
