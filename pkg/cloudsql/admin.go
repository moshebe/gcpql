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

// InstanceInfo holds minimal metadata needed for the core check.
type InstanceInfo struct {
	Region          string
	DatabaseVersion string
	MaxConnections  int
}

// FetchInstanceInfo calls the Cloud SQL Admin API to get instance metadata and config.
// Returns an error if the instance does not exist (404).
func FetchInstanceInfo(ctx context.Context, httpClient *http.Client, project, instance string) (InstanceInfo, InstanceConfig, error) {
	url := fmt.Sprintf(
		"https://sqladmin.googleapis.com/v1/projects/%s/instances/%s",
		project, instance,
	)
	info, cfg, err := fetchInstanceInfoFromURL(ctx, httpClient, url)
	if err != nil {
		// Preserve project/instance context in the error message.
		return InstanceInfo{}, InstanceConfig{}, fmt.Errorf("instance %s/%s: %w", project, instance, err)
	}
	return info, cfg, nil
}

func fetchInstanceInfoFromURL(ctx context.Context, httpClient *http.Client, url string) (InstanceInfo, InstanceConfig, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return InstanceInfo{}, InstanceConfig{}, fmt.Errorf("failed to build admin API request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return InstanceInfo{}, InstanceConfig{}, fmt.Errorf("admin API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return InstanceInfo{}, InstanceConfig{}, fmt.Errorf("failed to read admin API response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return InstanceInfo{}, InstanceConfig{}, fmt.Errorf("instance not found")
	}
	if resp.StatusCode != http.StatusOK {
		return InstanceInfo{}, InstanceConfig{}, fmt.Errorf("admin API error (status %d): %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Region          string `json:"region"`
		DatabaseVersion string `json:"databaseVersion"`
		ConnectionName  string `json:"connectionName"`
		State           string `json:"state"`
		Settings        struct {
			Tier                      string            `json:"tier"`
			AvailabilityType          string            `json:"availabilityType"`
			StorageAutoResize         bool              `json:"storageAutoResize"`
			StorageAutoResizeLimit    string            `json:"storageAutoResizeLimit"`
			DataDiskType              string            `json:"dataDiskType"`
			DeletionProtectionEnabled bool              `json:"deletionProtectionEnabled"`
			UserLabels                map[string]string `json:"userLabels"`
			DatabaseFlags             []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"databaseFlags"`
			BackupConfiguration struct {
				Enabled                    bool   `json:"enabled"`
				StartTime                  string `json:"startTime"`
				PointInTimeRecoveryEnabled bool   `json:"pointInTimeRecoveryEnabled"`
			} `json:"backupConfiguration"`
			InsightsConfig struct {
				QueryInsightsEnabled bool `json:"queryInsightsEnabled"`
			} `json:"insightsConfig"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return InstanceInfo{}, InstanceConfig{}, fmt.Errorf("failed to parse admin API response: %w", err)
	}

	info := InstanceInfo{
		Region:          parsed.Region,
		DatabaseVersion: parsed.DatabaseVersion,
	}

	autoResizeGB, _ := strconv.ParseInt(parsed.Settings.StorageAutoResizeLimit, 10, 64)
	flags := make([]DBFlag, 0, len(parsed.Settings.DatabaseFlags))
	for _, f := range parsed.Settings.DatabaseFlags {
		flags = append(flags, DBFlag{Name: f.Name, Value: f.Value})
	}

	cfg := InstanceConfig{
		Labels:               parsed.Settings.UserLabels,
		AvailabilityType:     parsed.Settings.AvailabilityType,
		BackupEnabled:        parsed.Settings.BackupConfiguration.Enabled,
		BackupStartTime:      parsed.Settings.BackupConfiguration.StartTime,
		PITREnabled:          parsed.Settings.BackupConfiguration.PointInTimeRecoveryEnabled,
		StorageType:          parsed.Settings.DataDiskType,
		StorageAutoResize:    parsed.Settings.StorageAutoResize,
		StorageAutoResizeGB:  autoResizeGB,
		DatabaseFlags:        flags,
		QueryInsightsEnabled: parsed.Settings.InsightsConfig.QueryInsightsEnabled,
		DeletionProtection:   parsed.Settings.DeletionProtectionEnabled,
		State:                parsed.State,
		ConnectionName:       parsed.ConnectionName,
	}

	// Resolve max_connections: explicit flag wins, then tier-based fallback.
	for _, flag := range parsed.Settings.DatabaseFlags {
		if flag.Name == "max_connections" {
			if v, err := strconv.Atoi(flag.Value); err == nil {
				info.MaxConnections = v
				return info, cfg, nil
			}
		}
	}
	info.MaxConnections = maxConnectionsFromTier(parsed.Settings.Tier)
	return info, cfg, nil
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
