package cloudsql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ListItem represents one CloudSQL instance in the list output.
type ListItem struct {
	Instance  string   `json:"instance"`
	State     string   `json:"state"`
	DBVersion string   `json:"db_version"`
	Region    string   `json:"region"`
	VCPU      int      `json:"vcpu,omitempty"`
	MemoryGB  float64  `json:"memory_gb,omitempty"`
	CPUPct    *float64 `json:"cpu_pct,omitempty"`
	MemPct    *float64 `json:"mem_pct,omitempty"`
}

// ListResult holds all instances for a project.
type ListResult struct {
	Project   string     `json:"project"`
	Timestamp time.Time  `json:"timestamp"`
	Items     []ListItem `json:"items"`
}

// parseTier extracts vCPU count and memory GB from a Cloud SQL tier string.
// Handles "db-custom-{vcpu}-{memMB}" format. Returns (0, 0) for unknown tiers.
func parseTier(tier string) (vcpu int, memGB float64) {
	parts := strings.Split(tier, "-")
	if len(parts) != 4 || parts[0] != "db" || parts[1] != "custom" {
		return 0, 0
	}
	v, err1 := strconv.Atoi(parts[2])
	m, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return v, float64(m) / 1024
}

// instanceAdminRecord is a raw record from the Cloud SQL Admin API list response.
type instanceAdminRecord struct {
	name      string
	state     string
	dbVersion string
	region    string
	vcpu      int
	memoryGB  float64
}

// fetchAllInstances lists all Cloud SQL instances for a project via the Admin API.
func fetchAllInstances(ctx context.Context, httpClient *http.Client, project string) ([]instanceAdminRecord, error) {
	url := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances", project)
	return fetchAllInstancesFromURL(ctx, httpClient, url)
}

// fetchAllInstancesFromURL is the testable inner implementation.
func fetchAllInstancesFromURL(ctx context.Context, httpClient *http.Client, url string) ([]instanceAdminRecord, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("admin API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("admin API error (status %d): %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Items []struct {
			Name            string `json:"name"`
			State           string `json:"state"`
			DatabaseVersion string `json:"databaseVersion"`
			Region          string `json:"region"`
			Settings        struct {
				Tier string `json:"tier"`
			} `json:"settings"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	records := make([]instanceAdminRecord, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		vcpu, memGB := parseTier(item.Settings.Tier)
		records = append(records, instanceAdminRecord{
			name:      item.Name,
			state:     item.State,
			dbVersion: item.DatabaseVersion,
			region:    item.Region,
			vcpu:      vcpu,
			memoryGB:  memGB,
		})
	}
	return records, nil
}
