package config

import (
	"errors"
	"os"
	"testing"
)

func TestResolveProject_FromFlag(t *testing.T) {
	// Clear environment
	os.Unsetenv("GCP_PROJECT")

	project, err := ResolveProject("flag-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project != "flag-project" {
		t.Errorf("expected 'flag-project', got '%s'", project)
	}
}

func TestResolveProject_FromEnv(t *testing.T) {
	os.Setenv("GCP_PROJECT", "env-project")
	defer os.Unsetenv("GCP_PROJECT")

	project, err := ResolveProject("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project != "env-project" {
		t.Errorf("expected 'env-project', got '%s'", project)
	}
}

func TestResolveProject_NoProjectFound(t *testing.T) {
	// Save and restore original gcloudCommandFunc
	originalFunc := gcloudCommandFunc
	defer func() { gcloudCommandFunc = originalFunc }()

	// Mock gcloud command to return error
	gcloudCommandFunc = func() (string, error) {
		return "", errors.New("command not found")
	}

	os.Unsetenv("GCP_PROJECT")

	_, err := ResolveProject("")
	if err == nil {
		t.Error("expected error when no project found")
	}
}
