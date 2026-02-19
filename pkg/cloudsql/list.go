package cloudsql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
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
// Handles:
//   - db-custom-{vcpu}-{memMB}
//   - db-n1-standard-{n}   (3.75 GB/vCPU)
//   - db-n1-highmem-{n}    (6.5 GB/vCPU)
//   - db-perf-optimized-N-{n} (8 GB/vCPU)
//   - db-f1-micro, db-g1-small (shared-core)
//
// Returns (0, 0) for unrecognised tiers.
func parseTier(tier string) (vcpu int, memGB float64) {
	switch tier {
	case "db-f1-micro":
		return 1, 0.6
	case "db-g1-small":
		return 1, 1.7
	}

	parts := strings.Split(tier, "-")
	if len(parts) < 3 || parts[0] != "db" {
		return 0, 0
	}

	switch {
	case len(parts) == 4 && parts[1] == "custom":
		// db-custom-{vcpu}-{memMB}
		v, err1 := strconv.Atoi(parts[2])
		m, err2 := strconv.Atoi(parts[3])
		if err1 != nil || err2 != nil {
			return 0, 0
		}
		return v, float64(m) / 1024

	case len(parts) == 4 && parts[1] == "n1" && parts[2] == "standard":
		// db-n1-standard-{n}
		v, err := strconv.Atoi(parts[3])
		if err != nil {
			return 0, 0
		}
		return v, float64(v) * 3.75

	case len(parts) == 4 && parts[1] == "n1" && parts[2] == "highmem":
		// db-n1-highmem-{n}
		v, err := strconv.Atoi(parts[3])
		if err != nil {
			return 0, 0
		}
		return v, float64(v) * 6.5

	case len(parts) == 5 && parts[1] == "perf" && parts[2] == "optimized":
		// db-perf-optimized-N-{n}
		v, err := strconv.Atoi(parts[4])
		if err != nil {
			return 0, 0
		}
		return v, float64(v) * 8
	}

	return 0, 0
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
// Any other format returns empty string; callers should skip empty results.
func normalizeDBID(id string) string {
	parts := strings.Split(id, ":")
	switch len(parts) {
	case 2:
		return id // already "project:instance"
	case 3:
		return parts[0] + ":" + parts[2] // "project:region:instance" → "project:instance"
	default:
		return "" // unrecognized format; caller skips empty key
	}
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
		if dbID == "" {
			continue
		}

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

// ListInstances fetches all instances for a project with live CPU+mem metrics.
// Monitoring failures are non-fatal; affected instances show nil CPUPct/MemPct.
func ListInstances(ctx context.Context, httpClient *http.Client, monClient *monitoring.Client, project string, since time.Duration) (*ListResult, error) {
	adminURL := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances", project)
	return listInstancesWithURL(ctx, httpClient, adminURL, monClient, project, since)
}

// listInstancesWithURL is the testable inner implementation.
func listInstancesWithURL(ctx context.Context, httpClient *http.Client, adminURL string, monClient *monitoring.Client, project string, since time.Duration) (*ListResult, error) {
	var (
		adminRecords []instanceAdminRecord
		cpuMap       map[string]float64
		memMap       map[string]float64
		adminErr     error
	)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		adminRecords, adminErr = fetchAllInstancesFromURL(ctx, httpClient, adminURL)
	}()
	go func() {
		defer wg.Done()
		cpuMap, _ = fetchBulkUtilization(ctx, monClient, project, since,
			"cloudsql.googleapis.com/database/cpu/utilization")
	}()
	go func() {
		defer wg.Done()
		memMap, _ = fetchBulkUtilization(ctx, monClient, project, since,
			"cloudsql.googleapis.com/database/memory/utilization")
	}()

	wg.Wait()

	if adminErr != nil {
		return nil, fmt.Errorf("admin API: %w", adminErr)
	}

	items := make([]ListItem, 0, len(adminRecords))
	for _, r := range adminRecords {
		dbID := fmt.Sprintf("%s:%s", project, r.name)
		item := ListItem{
			Instance:  dbID,
			State:     r.state,
			DBVersion: r.dbVersion,
			Region:    r.region,
			VCPU:      r.vcpu,
			MemoryGB:  r.memoryGB,
		}
		if v, ok := cpuMap[dbID]; ok {
			pct := v * 100
			item.CPUPct = &pct
		}
		if v, ok := memMap[dbID]; ok {
			pct := v * 100
			item.MemPct = &pct
		}
		items = append(items, item)
	}

	// Sort by CPU% descending; nil (no data) sorts last.
	sort.Slice(items, func(i, j int) bool {
		if items[i].CPUPct == nil && items[j].CPUPct == nil {
			return false
		}
		if items[i].CPUPct == nil {
			return false
		}
		if items[j].CPUPct == nil {
			return true
		}
		return *items[i].CPUPct > *items[j].CPUPct
	})

	return &ListResult{
		Project:   project,
		Timestamp: time.Now(),
		Items:     items,
	}, nil
}
