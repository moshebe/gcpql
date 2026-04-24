package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/moshebe/gcpql/internal/config"
)

// ParseSubscriptionID parses a subscription identifier and returns (project, name, error).
// Accepted formats:
//   - "my-sub" — requires --project flag or resolvable gcloud config
//   - "projects/my-project/subscriptions/my-sub" — GCP canonical path
func ParseSubscriptionID(id string, projectFlag string) (project, name string, err error) {
	if strings.HasPrefix(id, "projects/") {
		parts := strings.Split(id, "/")
		if len(parts) != 4 || parts[2] != "subscriptions" || parts[3] == "" {
			return "", "", fmt.Errorf("invalid subscription path %q: expected projects/<project>/subscriptions/<name>", id)
		}
		return parts[1], parts[3], nil
	}

	resolvedProject, err := config.ResolveProject(projectFlag)
	if err != nil {
		return "", "", fmt.Errorf("resolving project: %w", err)
	}
	return resolvedProject, id, nil
}

// subscriptionConfig is the partial PubSub Admin API response for a subscription.
type subscriptionConfig struct {
	Topic string `json:"topic"` // full path: projects/<project>/topics/<name>
}

// getSubscriptionTopic fetches the parent topic name (short, not full path) for a subscription
// via the PubSub Admin REST API. Returns "" if not found.
func getSubscriptionTopic(ctx context.Context, httpClient *http.Client, project, subscription string) (string, error) {
	url := fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/subscriptions/%s", project, subscription)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching subscription config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("subscription %q not found in project %q", subscription, project)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PubSub API error (status %d)", resp.StatusCode)
	}

	var cfg subscriptionConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return "", fmt.Errorf("decoding subscription config: %w", err)
	}

	// Extract short name from full path: projects/<project>/topics/<name>
	parts := strings.Split(cfg.Topic, "/")
	return parts[len(parts)-1], nil
}
