package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moshebeladev/gcp-metrics/internal/config"
	"github.com/moshebeladev/gcp-metrics/pkg/monitoring"
	"github.com/moshebeladev/gcp-metrics/pkg/output"
	"github.com/moshebeladev/gcp-metrics/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	since  string
	window string
)

var queryCmd = &cobra.Command{
	Use:   "query <MQL>",
	Short: "Execute an MQL query",
	Long:  `Execute a Monitoring Query Language (MQL) query against GCP Cloud Monitoring.`,
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
	mql := args[0]

	// Resolve project
	project, err := config.ResolveProject(projectID)
	if err != nil {
		return formatAndPrintError("CONFIG_ERROR", err.Error(), mql)
	}

	// Parse time range
	start, end, err := timerange.Parse(since, window)
	if err != nil {
		return formatAndPrintError("VALIDATION_ERROR", err.Error(), mql)
	}

	// Create monitoring client
	client, err := monitoring.NewClient(ctx)
	if err != nil {
		return formatAndPrintError("AUTH_ERROR", fmt.Sprintf("Authentication failed: %v. Run 'gcloud auth application-default login'", err), mql)
	}

	// Execute query
	resp, err := client.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
		Project:   project,
		Query:     mql,
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		return formatAndPrintError("API_ERROR", err.Error(), mql)
	}

	// Format and print result
	result := &output.QueryResult{
		Query:      mql,
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

	// Print to stderr for human context
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)

	// Print JSON to stdout
	output.FormatError(os.Stdout, errResult)

	return fmt.Errorf(message)
}
