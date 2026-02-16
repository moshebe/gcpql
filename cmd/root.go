package cmd

import (
	"github.com/spf13/cobra"
)

var (
	projectID string
	format    string
)

var rootCmd = &cobra.Command{
	Use:   "gcp-metrics",
	Short: "Query GCP Monitoring metrics",
	Long:  `A CLI tool for querying GCP Cloud Monitoring metrics using MQL.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&projectID, "project", "", "GCP project ID (default: from gcloud config)")
	rootCmd.PersistentFlags().StringVar(&format, "format", "json", "Output format (json)")
}
