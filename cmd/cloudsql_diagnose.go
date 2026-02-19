package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moshebe/gcpql/internal/config"
	"github.com/moshebe/gcpql/pkg/cloudsql"
	"github.com/moshebe/gcpql/pkg/monitoring"
	"github.com/moshebe/gcpql/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	diagnoseSince         string
	diagnoseFormat        string
	diagnoseQueryInsights bool
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose <instance-id>",
	Short: "Diagnose common CloudSQL/PostgreSQL issues with suggested actions",
	Long: `Collect CloudSQL metrics and analyze them against known problem patterns.
Reports critical issues, warnings, and informational findings with suggested
remediation steps.

Instance ID formats:
  - Short: my-instance (requires --project)
  - Full: my-project:my-instance
  - Database ID: my-project:us-central1:my-instance

Examples:
  gcpql cloudsql diagnose my-instance
  gcpql cloudsql diagnose my-instance --since 7d
  gcpql cloudsql diagnose my-instance --query-insights
  gcpql cloudsql diagnose my-instance --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runDiagnose,
}

func init() {
	cloudsqlCmd.AddCommand(diagnoseCmd)
	diagnoseCmd.Flags().StringVar(&diagnoseSince, "since", "24h", "Time window for metrics (e.g., 1h, 24h, 7d)")
	diagnoseCmd.Flags().StringVar(&diagnoseFormat, "format", "table", "Output format: table or json")
	diagnoseCmd.Flags().BoolVar(&diagnoseQueryInsights, "query-insights", false, "Include Query Insights data in diagnosis (opt-in, slower)")
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	instanceID := args[0]

	start, end, err := timerange.Parse(diagnoseSince, "")
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}
	sinceDuration := end.Sub(start)

	resolvedProject, err := config.ResolveProject(projectID)
	if err != nil && projectID == "" {
		resolvedProject = ""
	}

	project, instance, err := cloudsql.ParseInstanceID(instanceID, resolvedProject)
	if err != nil {
		return err
	}

	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating monitoring client: %w", err)
	}

	collector := cloudsql.NewCollector(monClient)
	result, err := collector.CollectMetrics(ctx, project, instance, sinceDuration, diagnoseQueryInsights)
	if err != nil {
		return fmt.Errorf("collecting metrics: %w", err)
	}

	dr := cloudsql.Diagnose(result, sinceDuration)

	switch diagnoseFormat {
	case "json":
		return cloudsql.FormatDiagnoseJSON(os.Stdout, dr)
	case "table":
		return cloudsql.FormatDiagnoseTable(os.Stdout, dr)
	default:
		return fmt.Errorf("unknown format: %s", diagnoseFormat)
	}
}
