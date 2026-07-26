package autoscaling

import (
	"testing"
	"time"
)

func TestDecideScalesByQueueCompute(t *testing.T) {
	d := Decide(Inputs{
		QueueDepth:           10,
		EstimatedJobDuration: time.Minute,
		RequestedCPUUnits:    1,
		TargetQueueDrain:     2 * time.Minute,
		CurrentCapacity:      1,
		MinCapacity:          1,
		MaxCapacity:          20,
		MaxScaleUpStep:       3,
	})
	if d.DesiredCapacity != 4 {
		t.Fatalf("desired capacity = %d, want current+step 4 (%s)", d.DesiredCapacity, d.Reason)
	}
}

func TestOldestAgePreventsScaleDown(t *testing.T) {
	d := Decide(Inputs{
		QueueDepth:       1,
		OldestQueuedAge:  time.Minute,
		CurrentCapacity:  5,
		MinCapacity:      1,
		MaxCapacity:      10,
		MaxScaleDownStep: 2,
	})
	if d.DesiredCapacity != 5 {
		t.Fatalf("expected no scale down with queued work, got %d", d.DesiredCapacity)
	}
}

func TestCooldown(t *testing.T) {
	now := time.Now()
	d := Decide(Inputs{
		QueueDepth:      100,
		CurrentCapacity: 2,
		MinCapacity:     1,
		MaxCapacity:     10,
		ScaleUpCooldown: time.Minute,
		LastScaleTime:   now.Add(-10 * time.Second),
		Now:             now,
	})
	if d.DesiredCapacity != 2 {
		t.Fatalf("expected cooldown to hold capacity, got %d", d.DesiredCapacity)
	}
}
