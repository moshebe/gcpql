package config

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// GcloudCommandFunc is the function used to get gcloud project (mockable for tests)
var GcloudCommandFunc = func() (string, error) {
	cmd := exec.Command("gcloud", "config", "get-value", "project")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	project := strings.TrimSpace(string(output))
	if project == "(unset)" {
		return "", errors.New("project is unset")
	}
	return project, nil
}

// ResolveProject resolves GCP project from flag, env, or gcloud config
// Precedence: flag > GCP_PROJECT env > gcloud config
func ResolveProject(flagValue string) (string, error) {
	// 1. Flag takes precedence
	if flagValue != "" {
		return flagValue, nil
	}

	// 2. Environment variable
	if project := os.Getenv("GCP_PROJECT"); project != "" {
		return project, nil
	}

	// 3. gcloud config
	project, err := GcloudCommandFunc()
	if err == nil && project != "" {
		return project, nil
	}

	return "", errors.New("GCP project not configured. Use --project, set GCP_PROJECT, or run 'gcloud config set project PROJECT_ID'")
}
