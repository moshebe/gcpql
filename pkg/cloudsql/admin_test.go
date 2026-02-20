package cloudsql

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchInstanceInfo_ParsesInstanceConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"region": "us-central1",
			"databaseVersion": "POSTGRES_15",
			"connectionName": "myproject:us-central1:myinstance",
			"state": "RUNNABLE",
			"settings": {
				"tier": "db-custom-4-15360",
				"availabilityType": "REGIONAL",
				"storageAutoResize": true,
				"storageAutoResizeLimit": "500",
				"dataDiskType": "PD_SSD",
				"deletionProtectionEnabled": true,
				"userLabels": {"env": "prod", "team": "platform"},
				"databaseFlags": [
					{"name": "log_min_duration_statement", "value": "1000"},
					{"name": "max_connections", "value": "200"}
				],
				"backupConfiguration": {
					"enabled": true,
					"startTime": "03:00",
					"pointInTimeRecoveryEnabled": true
				},
				"insightsConfig": {
					"queryInsightsEnabled": true
				}
			}
		}`))
	}))
	defer srv.Close()

	info, cfg, err := fetchInstanceInfoFromURL(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// InstanceInfo
	if info.Region != "us-central1" {
		t.Errorf("region: got %q want %q", info.Region, "us-central1")
	}
	if info.MaxConnections != 200 {
		t.Errorf("max_connections: got %d want 200", info.MaxConnections)
	}

	// InstanceConfig
	if cfg.AvailabilityType != "REGIONAL" {
		t.Errorf("availability_type: got %q want REGIONAL", cfg.AvailabilityType)
	}
	if !cfg.BackupEnabled {
		t.Error("backup_enabled: want true")
	}
	if cfg.BackupStartTime != "03:00" {
		t.Errorf("backup_start_time: got %q want 03:00", cfg.BackupStartTime)
	}
	if !cfg.PITREnabled {
		t.Error("pitr_enabled: want true")
	}
	if cfg.StorageType != "PD_SSD" {
		t.Errorf("storage_type: got %q want PD_SSD", cfg.StorageType)
	}
	if !cfg.StorageAutoResize {
		t.Error("storage_auto_resize: want true")
	}
	if cfg.StorageAutoResizeGB != 500 {
		t.Errorf("storage_auto_resize_limit_gb: got %d want 500", cfg.StorageAutoResizeGB)
	}
	if !cfg.DeletionProtection {
		t.Error("deletion_protection: want true")
	}
	if cfg.State != "RUNNABLE" {
		t.Errorf("state: got %q want RUNNABLE", cfg.State)
	}
	if cfg.ConnectionName != "myproject:us-central1:myinstance" {
		t.Errorf("connection_name: got %q", cfg.ConnectionName)
	}
	if cfg.Labels["env"] != "prod" {
		t.Errorf("labels[env]: got %q want prod", cfg.Labels["env"])
	}
	if len(cfg.DatabaseFlags) != 2 {
		t.Errorf("database_flags: got %d want 2", len(cfg.DatabaseFlags))
	}
	if !cfg.QueryInsightsEnabled {
		t.Error("query_insights_enabled: want true")
	}
}
