package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moshebe/gcpql/internal/config"
	"github.com/moshebe/gcpql/pkg/errorreporting"
	"github.com/moshebe/gcpql/pkg/monitoring"
	"github.com/moshebe/gcpql/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	erListFormat  string
	erListSince   string
	erListService string
)

var erListCmd = &cobra.Command{
	Use:   "list",
	Short: "List top error groups by count",
	Long: `Fetch top 50 error groups from GCP Error Reporting, ordered by count.

The --since value is mapped to the nearest supported API window:
  ≤1h → PERIOD_1_HOUR, ≤6h → PERIOD_6_HOURS, ≤24h → PERIOD_1_DAY,
  ≤7d → PERIOD_1_WEEK (default), >7d → PERIOD_30_DAYS

Examples:
  gcpql errorreporting list --project my-project
  gcpql errorreporting list --project my-project --since 1d --format table
  gcpql errorreporting list --project my-project --service my-service`,
	Args: cobra.NoArgs,
	RunE: runERList,
}

func init() {
	errorreportingCmd.AddCommand(erListCmd)
	erListCmd.Flags().StringVar(&erListFormat, "format", "json", "Output format: json or table")
	erListCmd.Flags().StringVar(&erListSince, "since", "", "Look-back window (e.g. 1h, 6h, 1d, 7d, 30d); default 7d")
	erListCmd.Flags().StringVar(&erListService, "service", "", "Filter to a specific service name")
}

func runERList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	project, err := config.ResolveProject(projectID)
	if err != nil {
		return fmt.Errorf("resolving project: %w", err)
	}

	var opts errorreporting.Options
	opts.Service = erListService
	if erListSince != "" {
		start, end, err := timerange.Parse(erListSince, "")
		if err != nil {
			return fmt.Errorf("invalid --since value: %w", err)
		}
		opts.Since = end.Sub(start)
	}

	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating monitoring client: %w", err)
	}

	groups, err := errorreporting.FetchGroups(ctx, monClient.HTTPClient(), project, opts)
	if err != nil {
		return fmt.Errorf("fetching error groups: %w", err)
	}

	result := &errorreporting.ListResult{
		Project: project,
		Groups:  groups,
		Total:   len(groups),
	}

	switch erListFormat {
	case "json":
		return errorreporting.FormatJSON(os.Stdout, result)
	case "table":
		return errorreporting.FormatTable(os.Stdout, result)
	default:
		return fmt.Errorf("unknown format: %s", erListFormat)
	}
}
