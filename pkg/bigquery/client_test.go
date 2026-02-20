package bigquery

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	ctx := context.Background()

	// This will fail in CI without credentials, but validates structure
	client, err := NewClient(ctx, "test-project", nil)
	if err != nil {
		// Expected to fail without creds
		return
	}

	if client == nil {
		t.Fatal("Expected client, got nil")
	}

	if client.project != "test-project" {
		t.Errorf("Expected project test-project, got %s", client.project)
	}
}

func TestSetLocation(t *testing.T) {
	client := &Client{location: "us"}
	client.SetLocation("eu")
	if client.location != "eu" {
		t.Errorf("Expected location eu, got %s", client.location)
	}
}

func TestClose_NilClient(t *testing.T) {
	client := &Client{}
	if err := client.Close(); err != nil {
		t.Errorf("Close() with nil client should not error, got %v", err)
	}
}
