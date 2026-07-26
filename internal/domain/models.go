package domain

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID                string
	Name              string
	SchedulingWeight  int
	MaxConcurrentJobs int
	CreatedAt         time.Time
}

type CacheSpec struct {
	Paths     []string `json:"paths" yaml:"paths"`
	KeyFiles  []string `json:"key_files" yaml:"key_files"`
	KeyPrefix string   `json:"key_prefix" yaml:"key_prefix"`
}

type JobDefinition struct {
	Name                  string    `json:"name" yaml:"name"`
	Commands              []string  `json:"commands" yaml:"commands"`
	RequestedCPUMillis    int       `json:"requested_cpu_millis" yaml:"requested_cpu_millis"`
	RequestedMemoryMB     int       `json:"requested_memory_mb" yaml:"requested_memory_mb"`
	MaxAttempts           int       `json:"max_attempts" yaml:"max_attempts"`
	Cache                 CacheSpec `json:"cache" yaml:"cache"`
	ActiveDeadlineSeconds int64     `json:"active_deadline_seconds" yaml:"active_deadline_seconds"`
}

type Workflow struct {
	ID         uuid.UUID
	TenantID   string
	Name       string
	Definition []JobDefinition
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type WorkflowRun struct {
	ID             uuid.UUID
	WorkflowID     uuid.UUID
	TenantID       string
	RepositoryURL  string
	CommitSHA      string
	Status         JobStatus
	Priority       Priority
	IdempotencyKey string
	CreatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Version        int
}

type Job struct {
	ID                 uuid.UUID
	WorkflowRunID      uuid.UUID
	TenantID           string
	Name               string
	Status             JobStatus
	Priority           Priority
	Commands           []string
	RequestedCPUMillis int
	RequestedMemoryMB  int
	MaxAttempts        int
	AttemptCount       int
	QueuedAt           *time.Time
	ScheduledAt        *time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
	Version            int
	Cache              CacheSpec
	RepositoryURL      string
	CommitSHA          string
}

type Attempt struct {
	ID                uuid.UUID
	JobID             uuid.UUID
	AttemptNumber     int
	Status            JobStatus
	KubernetesJobName string
	LeaseTokenHash    string
	LeaseExpiresAt    time.Time
	LastHeartbeatAt   *time.Time
	ExitCode          *int
	FailureReason     *string
	StartedAt         time.Time
	CompletedAt       *time.Time
}
