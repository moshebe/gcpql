package timerange

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var durationRegex = regexp.MustCompile(`^(\d+)([mhd])$`)

// Parse converts a relative time string (e.g., "5m", "1h", "7d") into start and end times.
// If since is empty, defaults to 5 minutes.
// window parameter is reserved for future use (MVP ignores it).
// Returns start time (now - duration) and end time (now).
func Parse(since, window string) (time.Time, time.Time, error) {
	if since == "" {
		since = "5m"
	}

	matches := durationRegex.FindStringSubmatch(since)
	if matches == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid format: %q (expected: 5m, 1h, 7d)", since)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid number: %w", err)
	}

	if value <= 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("duration must be positive")
	}

	unit := matches[2]
	var duration time.Duration

	switch unit {
	case "m":
		duration = time.Duration(value) * time.Minute
	case "h":
		duration = time.Duration(value) * time.Hour
	case "d":
		duration = time.Duration(value) * 24 * time.Hour
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported unit: %q (expected: m, h, d)", unit)
	}

	end := time.Now()
	start := end.Add(-duration)

	return start, end, nil
}
