package cmd

import "github.com/spf13/cobra"

var errorreportingCmd = &cobra.Command{
	Use:   "errorreporting",
	Short: "GCP Error Reporting commands",
	Long:  `Fetch and display error groups from GCP Error Reporting.`,
}

func init() {
	rootCmd.AddCommand(errorreportingCmd)
}
