package errorreporting

import "time"

// ErrorGroup represents a GCP Error Reporting error group
type ErrorGroup struct {
	GroupID          string    `json:"group_id"`
	Count            int64     `json:"count"`
	FirstSeen        time.Time `json:"first_seen,omitempty"`
	LastSeen         time.Time `json:"last_seen,omitempty"`
	Message          string    `json:"message"`
	AffectedServices []string  `json:"affected_services,omitempty"`
}

// ListResult contains the result of listing error groups
type ListResult struct {
	Project string       `json:"project"`
	Groups  []ErrorGroup `json:"groups"`
	Total   int          `json:"total"`
}
