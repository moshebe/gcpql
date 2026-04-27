package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moshebe/gcpql/pkg/memorystore"
	"github.com/moshebe/gcpql/pkg/monitoring"
	"github.com/moshebe/gcpql/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	memorystoreCheckSince  string
	memorystoreCheckFormat string
	memorystoreCheckTop    int
)

var memorystoreCheckCmd = &cobra.Command{
	Use:   "check <project-id>",
	Short: "Check Memorystore Redis instance health across a project",
	Long: `Fetch a health snapshot of all Memorystore Redis instances in the project.
Shows memory usage, connected clients, cache hit ratio, evictions, and uptime.

Examples:
  gcpql memorystore check my-project
  gcpql memorystore check my-project --since 6h
  gcpql memorystore check my-project --format json
  gcpql memorystore check my-project --top 10`,
	Args: cobra.ExactArgs(1),
	RunE: runMemorystoreCheck,
}

func init() {
	memorystoreCmd.AddCommand(memorystoreCheckCmd)
	memorystoreCheckCmd.Flags().StringVar(&memorystoreCheckSince, "since", "1h", "Time window for metrics (e.g. 1h, 24h, 7d)")
	memorystoreCheckCmd.Flags().StringVar(&memorystoreCheckFormat, "format", "table", "Output format: table or json")
	memorystoreCheckCmd.Flags().IntVar(&memorystoreCheckTop, "top", 5, "Number of worst instances to highlight")
}

func runMemorystoreCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	project := args[0]

	start, end, err := timerange.Parse(memorystoreCheckSince, "")
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}
	since := end.Sub(start)

	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating monitoring client: %w", err)
	}

	opts := memorystore.CheckOptions{
		Project: project,
		Since:   since,
		Top:     memorystoreCheckTop,
	}

	result, err := memorystore.CollectCheckMetrics(ctx, monClient, opts)
	if err != nil {
		return fmt.Errorf("collecting metrics: %w", err)
	}

	switch memorystoreCheckFormat {
	case "json":
		return memorystore.FormatCheckJSON(os.Stdout, result)
	case "table":
		return memorystore.FormatCheckTable(os.Stdout, result, memorystoreCheckTop)
	default:
		return fmt.Errorf("unknown format: %s", memorystoreCheckFormat)
	}
}
