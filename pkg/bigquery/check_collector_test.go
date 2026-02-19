package bigquery

import (
	"testing"
)

func TestCalculateSlotUtilization(t *testing.T) {
	allocated := int64(1000)
	current := int64(850)

	utilization := float64(current) / float64(allocated) * 100

	if utilization != 85.0 {
		t.Errorf("Expected 85.0%%, got %.1f%%", utilization)
	}
}
