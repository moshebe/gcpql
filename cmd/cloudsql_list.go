package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gcp-metrics/gcp-metrics/internal/config"
	"github.com/gcp-metrics/gcp-metrics/pkg/cloudsql"
	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
	"github.com/gcp-metrics/gcp-metrics/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	listSince  string
	listFormat string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List CloudSQL instances with live CPU/mem utilization",
	Long: `List all CloudSQL instances in a project with state, version, size,
and live CPU/memory utilization from Cloud Monitoring.

Examples:
  gcp-metrics cloudsql list --project my-project
  gcp-metrics cloudsql list --project my-project --format json
  gcp-metrics cloudsql list --project my-project --since 15m`,
	RunE: runList,
}

func init() {
	cloudsqlCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listSince, "since", "5m", "Time window for metrics (e.g., 5m, 1h)")
	listCmd.Flags().StringVar(&listFormat, "format", "table", "Output format: table or json")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	project, err := config.ResolveProject(projectID)
	if err != nil {
		return fmt.Errorf("project required: use --project flag or set GCP_PROJECT env var")
	}

	start, end, err := timerange.Parse(listSince, "")
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}
	sinceDuration := end.Sub(start)

	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create monitoring client: %w", err)
	}

	result, err := cloudsql.ListInstances(ctx, monClient.HTTPClient(), monClient, project, sinceDuration)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	switch listFormat {
	case "json":
		return cloudsql.FormatListJSON(os.Stdout, result)
	case "table":
		return cloudsql.FormatListTable(os.Stdout, result)
	default:
		return fmt.Errorf("unknown format: %s (use table or json)", listFormat)
	}
}
