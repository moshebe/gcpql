package errorreporting

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://clouderrorreporting.googleapis.com/v1beta1"

// Options controls what FetchGroups fetches.
type Options struct {
	// Service filters results to a specific service name (empty = all services).
	Service string
	// Since is the look-back window. Mapped to the closest supported API period.
	// Zero value defaults to 1 week.
	Since time.Duration
}

// durationToPeriod maps a duration to the GCP Error Reporting timeRange.period enum value.
// Supported periods: PERIOD_1_HOUR, PERIOD_6_HOURS, PERIOD_1_DAY, PERIOD_1_WEEK, PERIOD_30_DAYS.
func durationToPeriod(d time.Duration) string {
	switch {
	case d <= time.Hour:
		return "PERIOD_1_HOUR"
	case d <= 6*time.Hour:
		return "PERIOD_6_HOURS"
	case d <= 24*time.Hour:
		return "PERIOD_1_DAY"
	case d <= 7*24*time.Hour:
		return "PERIOD_1_WEEK"
	default:
		return "PERIOD_30_DAYS"
	}
}

// FetchGroups returns top error groups for the given project, ordered by count descending.
func FetchGroups(ctx context.Context, httpClient *http.Client, project string, opts Options) ([]ErrorGroup, error) {
	u, err := url.Parse(fmt.Sprintf("%s/projects/%s/groupStats", apiBase, project))
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}
	period := "PERIOD_1_WEEK"
	if opts.Since > 0 {
		period = durationToPeriod(opts.Since)
	}
	q := u.Query()
	q.Set("timeRange.period", period)
	q.Set("pageSize", "50")
	q.Set("order", "COUNT_DESC")
	if opts.Service != "" {
		q.Set("serviceFilter.service", opts.Service)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching error groups: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		ErrorGroupStats []struct {
			Group struct {
				GroupID string `json:"groupId"`
			} `json:"group"`
			Count         string `json:"count"`
			FirstSeenTime string `json:"firstSeenTime"`
			LastSeenTime  string `json:"lastSeenTime"`

			Representative struct {
				Message string `json:"message"`
			} `json:"representative"`
			AffectedServices []struct {
				Service string `json:"service"`
			} `json:"affectedServices"`
		} `json:"errorGroupStats"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	groups := make([]ErrorGroup, 0, len(apiResp.ErrorGroupStats))
	for _, g := range apiResp.ErrorGroupStats {
		eg := ErrorGroup{
			GroupID: g.Group.GroupID,
			Message: firstLine(g.Representative.Message, 200),
		}
		eg.Count, _ = strconv.ParseInt(g.Count, 10, 64)
		eg.FirstSeen, _ = time.Parse(time.RFC3339, g.FirstSeenTime)
		eg.LastSeen, _ = time.Parse(time.RFC3339, g.LastSeenTime)
		seen := make(map[string]struct{})
		for _, s := range g.AffectedServices {
			svc := stripServicePrefix(s.Service)
			if svc == "" {
				continue
			}
			if _, ok := seen[svc]; !ok {
				seen[svc] = struct{}{}
				eg.AffectedServices = append(eg.AffectedServices, svc)
			}
		}
		groups = append(groups, eg)
	}

	return groups, nil
}

// stripServicePrefix removes the "projects/{proj}/services/" prefix from a service resource name.
func stripServicePrefix(s string) string {
	if idx := strings.LastIndex(s, "/services/"); idx >= 0 {
		return s[idx+len("/services/"):]
	}
	return s
}

// firstLine returns the first line of s, truncated to max characters.
func firstLine(s string, max int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max]
	}
	return s
}
