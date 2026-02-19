package cloudsql

import (
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
