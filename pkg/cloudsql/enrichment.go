package cloudsql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// FetchRecommendations fetches Cloud Recommender suggestions for the given instance.
// Returns Recommendations{Available: false} (no error) on 403/404.
func FetchRecommendations(ctx context.Context, httpClient *http.Client, project, region string) (Recommendations, error) {
	url := fmt.Sprintf(
		"https://recommender.googleapis.com/v1/projects/%s/locations/%s/recommenders/google.cloudsql.instance.PerformanceRecommender/recommendations",
		project, region,
	)
	return fetchRecommendations(ctx, httpClient, project, region, url)
}

func fetchRecommendations(ctx context.Context, httpClient *http.Client, project, region, url string) (Recommendations, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Recommendations{}, fmt.Errorf("failed to build recommender request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return Recommendations{}, fmt.Errorf("recommender request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return Recommendations{Available: false}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Recommendations{}, fmt.Errorf("failed to read recommender response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Recommendations{}, fmt.Errorf("recommender API error (status %d): %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Recommendations []struct {
			Description   string `json:"description"`
			Priority      string `json:"priority"`
			PrimaryImpact struct {
				Category string `json:"category"`
			} `json:"primaryImpact"`
			StateInfo struct {
				State string `json:"state"`
			} `json:"stateInfo"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Recommendations{}, fmt.Errorf("failed to parse recommender response: %w", err)
	}

	items := make([]Recommendation, 0, len(parsed.Recommendations))
	for _, r := range parsed.Recommendations {
		items = append(items, Recommendation{
			Description: r.Description,
			Impact:      priorityToImpact(r.Priority),
			State:       r.StateInfo.State,
		})
	}

	return Recommendations{Available: true, Items: items}, nil
}

// priorityToImpact maps GCP Recommender priority strings to impact levels.
func priorityToImpact(priority string) string {
	switch priority {
	case "P1":
		return "HIGH"
	case "P2":
		return "MEDIUM"
	case "P3", "P4":
		return "LOW"
	default:
		return "UNKNOWN"
	}
}
