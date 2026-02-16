package main

import (
	"github.com/moshebeladev/gcp-metrics/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
