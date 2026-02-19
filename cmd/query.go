package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gcp-metrics/gcp-metrics/internal/config"
	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
	"github.com/gcp-metrics/gcp-metrics/pkg/output"
	"github.com/gcp-metrics/gcp-metrics/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	since  string
	window string
)

var queryCmd = &cobra.Command{
	Use:   "query <PromQL>",
	Short: "Execute a PromQL query",
	Long:  `Execute a Prometheus Query Language (PromQL) query against GCP Cloud Monitoring.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runQuery,
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringVar(&since, "since", "", "Time range (e.g., 5m, 1h, 24h, 7d)")
	queryCmd.Flags().StringVar(&window, "window", "", "Aggregation window (future)")
}

func runQuery(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	query := args[0]

	// Resolve project
	project, err := config.ResolveProject(projectID)
	if err != nil {
		return formatAndPrintError("CONFIG_ERROR", err.Error(), query)
	}

	// Parse time range
	start, end, err := timerange.Parse(since, window)
	if err != nil {
		return formatAndPrintError("VALIDATION_ERROR", err.Error(), query)
	}

	// Create monitoring client
	client, err := monitoring.NewClient(ctx)
	if err != nil {
		return formatAndPrintError("AUTH_ERROR", fmt.Sprintf("Authentication failed: %v. Run 'gcloud auth application-default login'", err), query)
	}

	// Execute query
	resp, err := client.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
		Project:   project,
		Query:     query,
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		return formatAndPrintError("API_ERROR", err.Error(), query)
	}

	// Format and print result
	result := &output.QueryResult{
		Query:      query,
		Project:    project,
		TimeRange:  output.TimeRange{Start: start, End: end},
		TimeSeries: resp.TimeSeries,
	}

	if err := output.FormatJSON(os.Stdout, result); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	return nil
}

func formatAndPrintError(code, message, query string) error {
	errResult := &output.ErrorResult{
		Error:   code,
		Message: message,
		Query:   query,
	}

	// Print to stderr with helpful hints
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)

	// Add hints based on error type
	if code == "AUTH_ERROR" {
		fmt.Fprintln(os.Stderr, "\nRun: gcloud auth application-default login")
	} else if code == "CONFIG_ERROR" {
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fmt.Fprintln(os.Stderr, "  gcp-metrics query \"...\" --project PROJECT_ID")
		fmt.Fprintln(os.Stderr, "  export GCP_PROJECT=PROJECT_ID")
		fmt.Fprintln(os.Stderr, "  gcloud config set project PROJECT_ID")
	}

	// Print JSON to stdout
	output.FormatError(os.Stdout, errResult)

	return fmt.Errorf("%s", message)
}
