package domain

import "testing"

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		from JobStatus
		to   JobStatus
		ok   bool
	}{
		{StatusQueued, StatusScheduling, true},
		{StatusQueued, StatusQueued, true},
		{StatusQueued, StatusSucceeded, false},
		{StatusRunning, StatusSucceeded, true},
		{StatusSucceeded, StatusFailed, false},
		{StatusLost, StatusQueued, true},
	}

	for _, tt := range tests {
		err := ValidateTransition(tt.from, tt.to)
		if tt.ok && err != nil {
			t.Fatalf("expected %s -> %s to be valid: %v", tt.from, tt.to, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("expected %s -> %s to be invalid", tt.from, tt.to)
		}
	}
}

func TestPriorityWeightDefaultsNormal(t *testing.T) {
	if got := PriorityWeight(Priority("unknown")); got != 2 {
		t.Fatalf("unknown priority got weight %d", got)
	}
}
