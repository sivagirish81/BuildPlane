package autoscaling

import (
	"math"
	"time"
)

type Inputs struct {
	QueueDepth            int
	OldestQueuedAge       time.Duration
	ActiveJobs            int
	PendingKubernetesJobs int
	EstimatedJobDuration  time.Duration
	RequestedCPUUnits     float64
	CurrentCapacity       int
	MinCapacity           int
	MaxCapacity           int
	TargetQueueDrain      time.Duration
	MaxScaleUpStep        int
	MaxScaleDownStep      int
	ScaleUpCooldown       time.Duration
	ScaleDownCooldown     time.Duration
	LastScaleTime         time.Time
	Now                   time.Time
}

type Decision struct {
	DesiredCapacity int
	Reason          string
}

func Decide(in Inputs) Decision {
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	if in.MinCapacity < 0 {
		in.MinCapacity = 0
	}
	if in.MaxCapacity < in.MinCapacity {
		in.MaxCapacity = in.MinCapacity
	}
	if in.TargetQueueDrain <= 0 {
		in.TargetQueueDrain = 2 * time.Minute
	}
	if in.EstimatedJobDuration <= 0 {
		in.EstimatedJobDuration = 5 * time.Minute
	}
	if in.RequestedCPUUnits <= 0 {
		in.RequestedCPUUnits = 1
	}
	requiredComputeSeconds := float64(in.QueueDepth) * in.EstimatedJobDuration.Seconds() * in.RequestedCPUUnits
	requiredCPU := requiredComputeSeconds / in.TargetQueueDrain.Seconds()
	desired := int(math.Ceil(requiredCPU)) + in.ActiveJobs + in.PendingKubernetesJobs
	if in.OldestQueuedAge > in.TargetQueueDrain && desired <= in.CurrentCapacity {
		desired = in.CurrentCapacity + 1
	}
	desired = clamp(desired, in.MinCapacity, in.MaxCapacity)

	if desired > in.CurrentCapacity {
		if since := in.Now.Sub(in.LastScaleTime); !in.LastScaleTime.IsZero() && since < in.ScaleUpCooldown {
			return Decision{DesiredCapacity: in.CurrentCapacity, Reason: "scale-up cooldown"}
		}
		step := in.MaxScaleUpStep
		if step <= 0 {
			step = desired - in.CurrentCapacity
		}
		if desired-in.CurrentCapacity > step {
			desired = in.CurrentCapacity + step
		}
		return Decision{DesiredCapacity: desired, Reason: "queue requires more capacity"}
	}
	if desired < in.CurrentCapacity {
		if in.QueueDepth > 0 && in.OldestQueuedAge > 0 {
			return Decision{DesiredCapacity: in.CurrentCapacity, Reason: "queued jobs prevent scale down"}
		}
		if since := in.Now.Sub(in.LastScaleTime); !in.LastScaleTime.IsZero() && since < in.ScaleDownCooldown {
			return Decision{DesiredCapacity: in.CurrentCapacity, Reason: "scale-down cooldown"}
		}
		step := in.MaxScaleDownStep
		if step <= 0 {
			step = in.CurrentCapacity - desired
		}
		if in.CurrentCapacity-desired > step {
			desired = in.CurrentCapacity - step
		}
		return Decision{DesiredCapacity: desired, Reason: "excess idle capacity"}
	}
	return Decision{DesiredCapacity: desired, Reason: "capacity unchanged"}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
