package cmd

import (
	"github.com/spf13/cobra"
)

var bigqueryCmd = &cobra.Command{
	Use:   "bigquery",
	Short: "BigQuery diagnostics and optimization commands",
	Long:  `Query BigQuery health metrics, analyze tables, and track query performance.`,
}

func init() {
	rootCmd.AddCommand(bigqueryCmd)
}
