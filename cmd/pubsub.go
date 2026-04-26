package cmd

import "github.com/spf13/cobra"

var pubsubCmd = &cobra.Command{
	Use:   "pubsub",
	Short: "PubSub diagnostics commands",
	Long:  `Diagnose PubSub subscription health, backlog, and delivery failures.`,
}

func init() {
	rootCmd.AddCommand(pubsubCmd)
}
