package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moshebe/gcpql/internal/config"
	"github.com/moshebe/gcpql/pkg/monitoring"
	"github.com/moshebe/gcpql/pkg/pubsub"
	"github.com/moshebe/gcpql/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	pubsubDiagnoseSince  string
	pubsubDiagnoseFormat string
)

var pubsubDiagnoseCmd = &cobra.Command{
	Use:   "diagnose <subscription>",
	Short: "Diagnose a PubSub subscription with actionable findings",
	Long: `Fetch subscription and topic metrics and analyze them against known problem patterns.
Reports CRITICAL, WARNING, and INFO findings with suggested remediation steps.

Subscription ID formats:
  - Short: my-sub (requires --project)
  - Canonical: projects/my-project/subscriptions/my-sub

Examples:
  gcpql pubsub diagnose my-sub --project my-project
  gcpql pubsub diagnose projects/my-project/subscriptions/my-sub
  gcpql pubsub diagnose my-sub --project my-project --since 6h --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runPubSubDiagnose,
}

func init() {
	pubsubCmd.AddCommand(pubsubDiagnoseCmd)
	pubsubDiagnoseCmd.Flags().StringVar(&pubsubDiagnoseSince, "since", "1h", "Time window for metrics (e.g. 1h, 24h, 7d)")
	pubsubDiagnoseCmd.Flags().StringVar(&pubsubDiagnoseFormat, "format", "table", "Output format: table or json")
}

func runPubSubDiagnose(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	subscriptionArg := args[0]

	resolvedProject, err := config.ResolveProject(projectID)
	if err != nil && projectID == "" {
		resolvedProject = ""
	}

	project, subscription, err := pubsub.ParseSubscriptionID(subscriptionArg, resolvedProject)
	if err != nil {
		return err
	}

	start, end, err := timerange.Parse(pubsubDiagnoseSince, "")
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}
	since := end.Sub(start)

	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating monitoring client: %w", err)
	}

	opts := pubsub.DiagnoseOptions{
		Project:      project,
		Subscription: subscription,
		Since:        since,
	}

	data, err := pubsub.CollectDiagnoseMetrics(ctx, monClient, opts)
	if err != nil {
		return fmt.Errorf("collecting metrics: %w", err)
	}

	dr := pubsub.Diagnose(*data)

	switch pubsubDiagnoseFormat {
	case "json":
		return pubsub.FormatDiagnoseJSON(os.Stdout, dr)
	case "table":
		return pubsub.FormatDiagnoseTable(os.Stdout, dr)
	default:
		return fmt.Errorf("unknown format: %s", pubsubDiagnoseFormat)
	}
}
