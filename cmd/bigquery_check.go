package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gcp-metrics/gcp-metrics/pkg/bigquery"
	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
	"github.com/gcp-metrics/gcp-metrics/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	bqCheckSince   string
	bqCheckFormat  string
	bqCheckDataset string
)

var bqCheckCmd = &cobra.Command{
	Use:   "check <project-id>",
	Short: "Check BigQuery health metrics",
	Long: `Fetch BigQuery slot utilization, cost indicators, and top expensive queries.

Examples:
  gcp-metrics bigquery check my-project
  gcp-metrics bigquery check my-project --since 7d
  gcp-metrics bigquery check my-project --dataset analytics --format table`,
	Args: cobra.ExactArgs(1),
	RunE: runBQCheck,
}

func init() {
	bigqueryCmd.AddCommand(bqCheckCmd)
	bqCheckCmd.Flags().StringVar(&bqCheckSince, "since", "24h", "Time window for metrics (e.g., 1h, 24h, 7d)")
	bqCheckCmd.Flags().StringVar(&bqCheckFormat, "format", "json", "Output format: json or table")
	bqCheckCmd.Flags().StringVar(&bqCheckDataset, "dataset", "", "Filter to specific dataset (optional)")
}

func runBQCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	project := args[0]

	// Parse time window
	start, end, err := timerange.Parse(bqCheckSince, "")
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}
	sinceDuration := end.Sub(start)

	// Create monitoring client
	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create monitoring client: %w", err)
	}

	// Create BigQuery client
	bqClient, err := bigquery.NewClient(ctx, project, monClient)
	if err != nil {
		return fmt.Errorf("failed to create bigquery client: %w", err)
	}
	defer bqClient.Close()

	// Collect metrics
	opts := bigquery.CheckOptions{
		Project: project,
		Dataset: bqCheckDataset,
		Since:   sinceDuration,
	}

	result, err := bigquery.CollectCheckMetrics(ctx, bqClient, opts)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Format output
	switch bqCheckFormat {
	case "json":
		return bigquery.FormatJSON(os.Stdout, result)
	case "table":
		return bigquery.FormatCheckTable(os.Stdout, result)
	default:
		return fmt.Errorf("unknown format: %s", bqCheckFormat)
	}
}
