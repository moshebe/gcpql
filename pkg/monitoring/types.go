package monitoring

import (
	"time"
)

// QueryTimeSeriesRequest represents a query request
type QueryTimeSeriesRequest struct {
	Project   string
	Query     string
	StartTime time.Time
	EndTime   time.Time
}

// QueryTimeSeriesResponse wraps the API response
type QueryTimeSeriesResponse struct {
	TimeSeries []interface{} // Raw time series from API
	// We'll use interface{} for now to avoid mapping entire API structure
}
