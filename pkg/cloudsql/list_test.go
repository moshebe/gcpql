package cloudsql

import (
	"testing"
)

func TestParseTier(t *testing.T) {
	tests := []struct {
		tier      string
		wantVCPU  int
		wantMemGB float64
	}{
		{"db-custom-4-15360", 4, 15.0},
		{"db-custom-2-7680", 2, 7.5},
		{"db-custom-1-3840", 1, 3.75},
		{"db-n1-standard-4", 0, 0},
		{"", 0, 0},
		{"unknown", 0, 0},
	}
	for _, tc := range tests {
		vcpu, memGB := parseTier(tc.tier)
		if vcpu != tc.wantVCPU {
			t.Errorf("parseTier(%q) vcpu = %d, want %d", tc.tier, vcpu, tc.wantVCPU)
		}
		if memGB != tc.wantMemGB {
			t.Errorf("parseTier(%q) memGB = %f, want %f", tc.tier, memGB, tc.wantMemGB)
		}
	}
}
