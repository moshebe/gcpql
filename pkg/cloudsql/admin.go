package cloudsql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// InstanceInfo holds metadata from the Cloud SQL Admin API.
type InstanceInfo struct {
	Region          string
	DatabaseVersion string
	MaxConnections  int
}

// FetchInstanceInfo calls the Cloud SQL Admin API to get instance metadata.
// Returns an error if the instance does not exist (404).
func FetchInstanceInfo(ctx context.Context, httpClient *http.Client, project, instance string) (InstanceInfo, error) {
	url := fmt.Sprintf(
		"https://sqladmin.googleapis.com/v1/projects/%s/instances/%s",
		project, instance,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return InstanceInfo{}, fmt.Errorf("failed to build admin API request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return InstanceInfo{}, fmt.Errorf("admin API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return InstanceInfo{}, fmt.Errorf("failed to read admin API response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return InstanceInfo{}, fmt.Errorf("instance %s:%s not found", project, instance)
	}
	if resp.StatusCode != http.StatusOK {
		return InstanceInfo{}, fmt.Errorf("admin API error (status %d): %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Region          string `json:"region"`
		DatabaseVersion string `json:"databaseVersion"`
		Settings        struct {
			Tier          string `json:"tier"`
			DatabaseFlags []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"databaseFlags"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return InstanceInfo{}, fmt.Errorf("failed to parse admin API response: %w", err)
	}

	info := InstanceInfo{
		Region:          parsed.Region,
		DatabaseVersion: parsed.DatabaseVersion,
	}

	// Check for explicit max_connections flag first.
	for _, flag := range parsed.Settings.DatabaseFlags {
		if flag.Name == "max_connections" {
			if v, err := strconv.Atoi(flag.Value); err == nil {
				info.MaxConnections = v
				return info, nil
			}
		}
	}

	// Fallback: derive from tier string (db-custom-{vCPU}-{memMB}).
	info.MaxConnections = maxConnectionsFromTier(parsed.Settings.Tier)
	return info, nil
}

// maxConnectionsFromTier computes max_connections from a GCP tier string.
// Formula: min(1000, max(25, 25*vCPUs)) for custom tiers.
// Falls back to 100 for unrecognised tiers.
func maxConnectionsFromTier(tier string) int {
	// e.g. "db-custom-4-15360"
	parts := strings.Split(tier, "-")
	if len(parts) == 4 && parts[0] == "db" && parts[1] == "custom" {
		if vCPUs, err := strconv.Atoi(parts[2]); err == nil && vCPUs > 0 {
			v := 25 * vCPUs
			if v < 25 {
				v = 25
			}
			if v > 1000 {
				v = 1000
			}
			return v
		}
	}
	return 100
}
