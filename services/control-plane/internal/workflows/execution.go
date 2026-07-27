package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/observability"
)

type NodeStatus string

const (
	NodeStatusPending   NodeStatus = "pending"
	NodeStatusQueued    NodeStatus = "queued"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusSucceeded NodeStatus = "succeeded"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusCanceled  NodeStatus = "canceled"
)

var (
	ErrNodeExecutionNotFound = errors.New("node execution not found")
	ErrLeaseUnavailable      = errors.New("node execution lease unavailable")
	ErrStaleLease            = errors.New("node execution lease is stale")
)

type NodeExecution struct {
	ID             string
	WorkflowRunID  string
	NodeName       string
	Status         NodeStatus
	Attempt        int
	LeaseWorkerID  string
	LeaseExpiresAt time.Time
	FencingToken   int64
	Result         json.RawMessage
	Error          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OutboxEvent struct {
	ID          int64
	Topic       string
	Payload     json.RawMessage
	Status      string
	Attempts    int
	PublishedAt time.Time
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type QueueMessage struct {
	ID              string
	NodeExecutionID string
	WorkflowRunID   string
	NodeName        string
	WorkerPool      string
}

type Lease struct {
	NodeExecutionID string
	WorkflowRunID   string
	WorkflowName    string
	NodeName        string
	WorkerPool      string
	CorrelationID   string
	TraceParent     string
	Input           json.RawMessage
	WorkerID        string
	Attempt         int
	FencingToken    int64
	LeaseExpiresAt  time.Time
}

type SchedulerRepository interface {
	ClaimSchedulableNodeExecutions(ctx context.Context, limit int) ([]OutboxEvent, error)
	ListPublishableOutboxEvents(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, id int64) error
	MarkOutboxFailed(ctx context.Context, id int64, errText string) error
}

type WorkerRepository interface {
	AcquireNodeExecutionLease(ctx context.Context, nodeExecutionID string, workerID string, leaseDuration time.Duration) (Lease, error)
	HeartbeatNodeExecution(ctx context.Context, nodeExecutionID string, workerID string, fencingToken int64, leaseDuration time.Duration) error
	CompleteNodeExecution(ctx context.Context, nodeExecutionID string, workerID string, fencingToken int64, result json.RawMessage, nextNodeName string) error
	FailNodeExecution(ctx context.Context, nodeExecutionID string, workerID string, fencingToken int64, errText string, maxAttempts int) error
}

type QueuePublisher interface {
	PublishNodeExecution(ctx context.Context, event OutboxEvent) error
}

type QueueConsumer interface {
	ReceiveNodeExecution(ctx context.Context, workerID string, wait time.Duration) (QueueMessage, error)
	AckNodeExecution(ctx context.Context, messageID string) error
}

var ErrNoQueueMessage = errors.New("no queue message available")

type Scheduler struct {
	repository SchedulerRepository
	publisher  QueuePublisher
	BatchSize  int
}

func NewScheduler(repository SchedulerRepository, publisher QueuePublisher) *Scheduler {
	return &Scheduler{
		repository: repository,
		publisher:  publisher,
		BatchSize:  10,
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) (int, int, error) {
	claimLimit := s.BatchSize
	if claimLimit <= 0 {
		claimLimit = 10
	}

	if _, err := s.repository.ClaimSchedulableNodeExecutions(ctx, claimLimit); err != nil {
		return 0, 0, err
	}

	events, err := s.repository.ListPublishableOutboxEvents(ctx, claimLimit)
	if err != nil {
		return 0, 0, err
	}

	published := 0
	for _, event := range events {
		if err := s.publisher.PublishNodeExecution(ctx, event); err != nil {
			if markErr := s.repository.MarkOutboxFailed(ctx, event.ID, err.Error()); markErr != nil {
				return published, len(events), fmt.Errorf("publish outbox %d: %w; mark failed: %v", event.ID, err, markErr)
			}
			continue
		}
		if err := s.repository.MarkOutboxPublished(ctx, event.ID); err != nil {
			return published, len(events), err
		}
		published++
	}

	return published, len(events), nil
}

type Worker struct {
	repository    WorkerRepository
	consumer      QueueConsumer
	WorkerID      string
	LeaseDuration time.Duration
	Dependencies  NodeDependencies
	Metrics       *observability.Registry
}

func NewWorker(repository WorkerRepository, consumer QueueConsumer, workerID string) *Worker {
	return &Worker{
		repository:    repository,
		consumer:      consumer,
		WorkerID:      workerID,
		LeaseDuration: 30 * time.Second,
	}
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	wait := time.Second
	message, err := w.consumer.ReceiveNodeExecution(ctx, w.WorkerID, wait)
	if errors.Is(err, ErrNoQueueMessage) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	leaseDuration := w.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}

	lease, err := w.repository.AcquireNodeExecutionLease(ctx, message.NodeExecutionID, w.WorkerID, leaseDuration)
	if errors.Is(err, ErrLeaseUnavailable) {
		return true, w.consumer.AckNodeExecution(ctx, message.ID)
	}
	if err != nil {
		return true, err
	}

	if err := w.repository.HeartbeatNodeExecution(ctx, lease.NodeExecutionID, lease.WorkerID, lease.FencingToken, leaseDuration); err != nil {
		return true, err
	}

	start := time.Now()
	output, maxAttempts, err := ExecuteNode(ctx, NodeInput{
		WorkflowRunID: lease.WorkflowRunID,
		WorkflowName:  lease.WorkflowName,
		NodeName:      lease.NodeName,
		CorrelationID: lease.CorrelationID,
		TraceParent:   lease.TraceParent,
		Input:         lease.Input,
	}, w.Dependencies)
	if err != nil {
		w.recordNodeExecution(lease, "failed", time.Since(start))
		if failErr := w.repository.FailNodeExecution(ctx, lease.NodeExecutionID, lease.WorkerID, lease.FencingToken, err.Error(), maxAttempts); failErr != nil {
			return true, failErr
		}
		if ackErr := w.consumer.AckNodeExecution(ctx, message.ID); ackErr != nil {
			return true, ackErr
		}
		return true, nil
	}

	if err := w.repository.CompleteNodeExecution(ctx, lease.NodeExecutionID, lease.WorkerID, lease.FencingToken, output.Result, output.NextNodeName); err != nil {
		return true, err
	}
	w.recordNodeExecution(lease, "succeeded", time.Since(start))

	if err := w.consumer.AckNodeExecution(ctx, message.ID); err != nil {
		return true, err
	}

	return true, nil
}

func (w *Worker) recordNodeExecution(lease Lease, result string, duration time.Duration) {
	if w.Metrics == nil {
		return
	}
	labels := map[string]string{
		"worker_pool": lease.WorkerPool,
		"node_name":   lease.NodeName,
		"result":      result,
	}
	w.Metrics.Inc("buildplane_worker_node_executions_total", labels)
	w.Metrics.Inc("buildplane_worker_node_execution_duration_seconds_count", labels)
	w.Metrics.Add("buildplane_worker_node_execution_duration_seconds_sum", labels, duration.Seconds())
}
