package main

import (
	"github.com/gcp-metrics/gcp-metrics/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
