package pubsub

import (
	"errors"
	"os"
	"testing"

	"github.com/moshebe/gcpql/internal/config"
)

func TestParseSubscriptionID(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		projectFlag string
		wantProject string
		wantName    string
		wantErr     bool
		setupMock   bool // true if test needs to mock gcloud resolution failure
	}{
		{
			name:        "canonical format",
			id:          "projects/my-project/subscriptions/my-sub",
			wantProject: "my-project",
			wantName:    "my-sub",
		},
		{
			name:        "short name with project flag",
			id:          "my-sub",
			projectFlag: "my-project",
			wantProject: "my-project",
			wantName:    "my-sub",
		},
		{
			name:      "short name without project returns error",
			id:        "my-sub",
			wantErr:   true,
			setupMock: true,
		},
		{
			name:    "canonical wrong segment count",
			id:      "projects/my-project/topics/my-sub",
			wantErr: true,
		},
		{
			name:    "canonical too few segments",
			id:      "projects/my-project/subscriptions",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock if needed for this test case
			if tc.setupMock {
				originalFunc := config.GcloudCommandFunc
				defer func() { config.GcloudCommandFunc = originalFunc }()

				// Mock gcloud command to return error
				config.GcloudCommandFunc = func() (string, error) {
					return "", errors.New("command not found")
				}

				os.Unsetenv("GCP_PROJECT")
			}

			got, gotName, err := ParseSubscriptionID(tc.id, tc.projectFlag)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantProject {
				t.Errorf("project: got %q, want %q", got, tc.wantProject)
			}
			if gotName != tc.wantName {
				t.Errorf("name: got %q, want %q", gotName, tc.wantName)
			}
		})
	}
}
