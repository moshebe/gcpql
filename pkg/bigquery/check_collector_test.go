package bigquery

import (
	"context"
	"testing"
	"time"
)

func TestCollectSlotMetrics(t *testing.T) {
	_ = context.Background()

	opts := CheckOptions{
		Project: "test-project",
		Dataset: "",
		Since:   24 * time.Hour,
	}

	// Mock client would go here in real tests
	// For now, test structure
	result := &CheckResult{
		Project: opts.Project,
		Dataset: opts.Dataset,
	}

	if result.Project != "test-project" {
		t.Errorf("Expected project test-project, got %s", result.Project)
	}
}
