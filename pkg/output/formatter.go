package output

import (
	"encoding/json"
	"io"
	"time"
)

// QueryResult represents the formatted output
type QueryResult struct {
	Query      string        `json:"query"`
	Project    string        `json:"project"`
	TimeRange  TimeRange     `json:"timeRange"`
	TimeSeries []interface{} `json:"timeSeries"`
}

// TimeRange represents the query time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ErrorResult represents an error response
type ErrorResult struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Query   string `json:"query,omitempty"`
}

// FormatJSON formats the query result as pretty-printed JSON
func FormatJSON(w io.Writer, result *QueryResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// FormatError formats an error as JSON
func FormatError(w io.Writer, errResult *ErrorResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(errResult)
}
