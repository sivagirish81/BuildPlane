package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSchedulerPublishesOutboxEvents(t *testing.T) {
	repository := &fakeSchedulerRepository{
		events: []OutboxEvent{{
			ID:      1,
			Topic:   "node_execution.queued",
			Payload: json.RawMessage(`{"node_execution_id":"node-1"}`),
		}},
	}
	publisher := &fakePublisher{}
	scheduler := NewScheduler(repository, publisher)

	published, seen, err := scheduler.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run scheduler once: %v", err)
	}
	if published != 1 || seen != 1 {
		t.Fatalf("expected published=1 seen=1, got published=%d seen=%d", published, seen)
	}
	if repository.publishedID != 1 {
		t.Fatalf("expected outbox event 1 marked published, got %d", repository.publishedID)
	}
}

func TestSchedulerMarksPublishFailure(t *testing.T) {
	repository := &fakeSchedulerRepository{
		events: []OutboxEvent{{ID: 7, Topic: "node_execution.queued"}},
	}
	publisher := &fakePublisher{err: errors.New("redis unavailable")}
	scheduler := NewScheduler(repository, publisher)

	published, seen, err := scheduler.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run scheduler once: %v", err)
	}
	if published != 0 || seen != 1 {
		t.Fatalf("expected published=0 seen=1, got published=%d seen=%d", published, seen)
	}
	if repository.failedID != 7 {
		t.Fatalf("expected outbox event 7 marked failed, got %d", repository.failedID)
	}
}

func TestWorkerCompletesMessageWithLeaseAndAck(t *testing.T) {
	repository := &fakeWorkerRepository{}
	consumer := &fakeConsumer{
		message: QueueMessage{
			ID:              "redis-1",
			NodeExecutionID: "node-1",
			WorkflowRunID:   "run-1",
			NodeName:        "phase3.bootstrap",
		},
	}
	worker := NewWorker(repository, consumer, "worker-1")
	worker.Dependencies = NodeDependencies{AIClassifier: &fakeAIClassifier{}}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run worker once: %v", err)
	}
	if !processed {
		t.Fatal("expected worker to process message")
	}
	if !repository.heartbeatCalled {
		t.Fatal("expected heartbeat")
	}
	if !repository.completed {
		t.Fatal("expected completion")
	}
	if consumer.ackedMessageID != "redis-1" {
		t.Fatalf("expected redis-1 acked, got %q", consumer.ackedMessageID)
	}
}

func TestWorkerAcksMessageWhenLeaseUnavailable(t *testing.T) {
	repository := &fakeWorkerRepository{leaseErr: ErrLeaseUnavailable}
	consumer := &fakeConsumer{
		message: QueueMessage{ID: "redis-1", NodeExecutionID: "node-1"},
	}
	worker := NewWorker(repository, consumer, "worker-1")

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run worker once: %v", err)
	}
	if !processed {
		t.Fatal("expected worker to process duplicate message")
	}
	if consumer.ackedMessageID != "redis-1" {
		t.Fatalf("expected duplicate message acked, got %q", consumer.ackedMessageID)
	}
}

func TestWorkerReturnsFalseWhenNoMessage(t *testing.T) {
	worker := NewWorker(&fakeWorkerRepository{}, &fakeConsumer{err: ErrNoQueueMessage}, "worker-1")

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run worker once: %v", err)
	}
	if processed {
		t.Fatal("expected no message")
	}
}

type fakeSchedulerRepository struct {
	events      []OutboxEvent
	publishedID int64
	failedID    int64
}

func (r *fakeSchedulerRepository) ClaimSchedulableNodeExecutions(context.Context, int) ([]OutboxEvent, error) {
	return r.events, nil
}

func (r *fakeSchedulerRepository) ListPublishableOutboxEvents(context.Context, int) ([]OutboxEvent, error) {
	return r.events, nil
}

func (r *fakeSchedulerRepository) MarkOutboxPublished(_ context.Context, id int64) error {
	r.publishedID = id
	return nil
}

func (r *fakeSchedulerRepository) MarkOutboxFailed(_ context.Context, id int64, _ string) error {
	r.failedID = id
	return nil
}

type fakePublisher struct {
	err error
}

func (p *fakePublisher) PublishNodeExecution(context.Context, OutboxEvent) error {
	return p.err
}

type fakeWorkerRepository struct {
	leaseErr        error
	heartbeatCalled bool
	completed       bool
}

func (r *fakeWorkerRepository) AcquireNodeExecutionLease(_ context.Context, nodeExecutionID string, workerID string, leaseDuration time.Duration) (Lease, error) {
	if r.leaseErr != nil {
		return Lease{}, r.leaseErr
	}
	return Lease{
		NodeExecutionID: nodeExecutionID,
		WorkflowRunID:   "run-1",
		WorkflowName:    LocalDemoWorkflowName,
		NodeName:        "validate_input",
		Input:           json.RawMessage(`{"case_id":"synthetic-case-001"}`),
		WorkerID:        workerID,
		Attempt:         1,
		FencingToken:    1,
		LeaseExpiresAt:  time.Now().Add(leaseDuration),
	}, nil
}

func (r *fakeWorkerRepository) HeartbeatNodeExecution(context.Context, string, string, int64, time.Duration) error {
	r.heartbeatCalled = true
	return nil
}

func (r *fakeWorkerRepository) CompleteNodeExecution(context.Context, string, string, int64, json.RawMessage, string) error {
	r.completed = true
	return nil
}

func (r *fakeWorkerRepository) FailNodeExecution(context.Context, string, string, int64, string, int) error {
	return nil
}

type fakeConsumer struct {
	message        QueueMessage
	err            error
	ackedMessageID string
}

func (c *fakeConsumer) ReceiveNodeExecution(context.Context, string, time.Duration) (QueueMessage, error) {
	return c.message, c.err
}

func (c *fakeConsumer) AckNodeExecution(_ context.Context, messageID string) error {
	c.ackedMessageID = messageID
	return nil
}
