package domain

import (
	"errors"
	"fmt"
)

type JobStatus string

const (
	StatusQueued     JobStatus = "QUEUED"
	StatusScheduling JobStatus = "SCHEDULING"
	StatusScheduled  JobStatus = "SCHEDULED"
	StatusRunning    JobStatus = "RUNNING"
	StatusSucceeded  JobStatus = "SUCCEEDED"
	StatusFailed     JobStatus = "FAILED"
	StatusTimedOut   JobStatus = "TIMED_OUT"
	StatusCancelled  JobStatus = "CANCELLED"
	StatusLost       JobStatus = "LOST"
)

type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityNormal   Priority = "normal"
	PriorityLow      Priority = "low"
)

var priorityWeights = map[Priority]int{
	PriorityCritical: 8,
	PriorityHigh:     4,
	PriorityNormal:   2,
	PriorityLow:      1,
}

var allowedTransitions = map[JobStatus]map[JobStatus]bool{
	StatusQueued: {
		StatusScheduling: true,
		StatusCancelled:  true,
	},
	StatusScheduling: {
		StatusScheduled: true,
		StatusQueued:    true,
		StatusFailed:    true,
		StatusCancelled: true,
	},
	StatusScheduled: {
		StatusRunning:   true,
		StatusQueued:    true,
		StatusLost:      true,
		StatusCancelled: true,
	},
	StatusRunning: {
		StatusSucceeded: true,
		StatusFailed:    true,
		StatusTimedOut:  true,
		StatusLost:      true,
		StatusCancelled: true,
	},
	StatusLost: {
		StatusQueued: true,
		StatusFailed: true,
	},
}

func PriorityWeight(priority Priority) int {
	if weight, ok := priorityWeights[priority]; ok {
		return weight
	}
	return priorityWeights[PriorityNormal]
}

func ValidatePriority(priority Priority) error {
	switch priority {
	case PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow:
		return nil
	default:
		return fmt.Errorf("invalid priority %q", priority)
	}
}

func Terminal(status JobStatus) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusTimedOut, StatusCancelled:
		return true
	default:
		return false
	}
}

var ErrInvalidTransition = errors.New("invalid state transition")

func CanTransition(from, to JobStatus) bool {
	if from == to {
		return true
	}
	return allowedTransitions[from][to]
}

func ValidateTransition(from, to JobStatus) error {
	if CanTransition(from, to) {
		return nil
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}
