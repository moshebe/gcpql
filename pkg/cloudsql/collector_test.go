package cloudsql

import (
	"testing"
)

func TestParseInstanceID(t *testing.T) {
	tests := []struct {
		name         string
		instanceID   string
		project      string
		wantProject  string
		wantInstance string
		wantErr      bool
	}{
		{
			name:         "short form with project",
			instanceID:   "my-instance",
			project:      "my-project",
			wantProject:  "my-project",
			wantInstance: "my-instance",
			wantErr:      false,
		},
		{
			name:         "full form project:instance",
			instanceID:   "my-project:my-instance",
			project:      "",
			wantProject:  "my-project",
			wantInstance: "my-instance",
			wantErr:      false,
		},
		{
			name:         "database ID format project:region:instance",
			instanceID:   "my-project:us-central1:my-instance",
			project:      "",
			wantProject:  "my-project",
			wantInstance: "my-instance",
			wantErr:      false,
		},
		{
			name:         "short form without project",
			instanceID:   "my-instance",
			project:      "",
			wantProject:  "",
			wantInstance: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProject, gotInstance, err := ParseInstanceID(tt.instanceID, tt.project)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInstanceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotProject != tt.wantProject {
				t.Errorf("ParseInstanceID() project = %v, want %v", gotProject, tt.wantProject)
			}

			if gotInstance != tt.wantInstance {
				t.Errorf("ParseInstanceID() instance = %v, want %v", gotInstance, tt.wantInstance)
			}
		})
	}
}
