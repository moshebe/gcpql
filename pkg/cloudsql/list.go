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

	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
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
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("project not found")
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

// normalizeDBID converts a 3-part monitoring label "project:region:instance"
// to the 2-part format "project:instance". 2-part IDs are returned unchanged.
func normalizeDBID(id string) string {
	parts := strings.Split(id, ":")
	if len(parts) == 3 {
		return parts[0] + ":" + parts[2]
	}
	return id
}

// fetchBulkUtilization queries a single metric type for all instances in a project.
// Returns map[database_id → latest value] where database_id is "project:instance".
// Monitoring errors return nil map (non-fatal; list shows "-" for affected instances).
func fetchBulkUtilization(ctx context.Context, monClient *monitoring.Client, project string, since time.Duration, metricType string) (map[string]float64, error) {
	now := time.Now()
	query := fmt.Sprintf(`{__name__="%s"}`, metricType)
	resp, err := monClient.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
		Project:   project,
		Query:     query,
		StartTime: now.Add(-since),
		EndTime:   now,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64, len(resp.TimeSeries))
	for _, ts := range resp.TimeSeries {
		tsMap, ok := ts.(map[string]interface{})
		if !ok {
			continue
		}
		metricLabels, ok := tsMap["metric"].(map[string]interface{})
		if !ok {
			continue
		}
		dbID, ok := metricLabels["database_id"].(string)
		if !ok {
			continue
		}
		dbID = normalizeDBID(dbID)

		values, ok := tsMap["values"].([]interface{})
		if !ok || len(values) == 0 {
			continue
		}
		// Take the last (most recent) point.
		last, ok := values[len(values)-1].([]interface{})
		if !ok || len(last) < 2 {
			continue
		}
		valStr, ok := last[1].(string)
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		result[dbID] = v
	}
	return result, nil
}
