package cmd

import (
	"github.com/spf13/cobra"
)

var cloudsqlCmd = &cobra.Command{
	Use:   "cloudsql",
	Short: "CloudSQL instance monitoring commands",
	Long:  `Query and monitor CloudSQL instance metrics for health checking and diagnostics.`,
}

func init() {
	rootCmd.AddCommand(cloudsqlCmd)
}
