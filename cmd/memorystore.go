package cmd

import "github.com/spf13/cobra"

var memorystoreCmd = &cobra.Command{
	Use:   "memorystore",
	Short: "Memorystore (Redis) diagnostics commands",
	Long:  `Check Memorystore Redis instance health: memory, connections, hit rate, evictions.`,
}

func init() {
	rootCmd.AddCommand(memorystoreCmd)
}
