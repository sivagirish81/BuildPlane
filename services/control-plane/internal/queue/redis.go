package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

const (
	streamPrefix = "buildplane:node-executions"
	defaultGroup = "buildplane-workers"
)

type RedisQueue struct {
	client     *redis.Client
	workerPool string
	group      string
}

func NewRedisQueue(client *redis.Client) *RedisQueue {
	return NewRedisQueueForPool(client, workflows.WorkerPoolGeneral)
}

func NewRedisQueueForPool(client *redis.Client, workerPool string) *RedisQueue {
	return &RedisQueue{
		client:     client,
		workerPool: normalizeWorkerPool(workerPool),
		group:      defaultGroup,
	}
}

func OpenRedis(redisURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return redis.NewClient(options), nil
}

func (q *RedisQueue) EnsureConsumerGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, q.streamName(), q.group, "0").Err()
	if err == nil {
		return nil
	}
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if isBusyGroup(err) {
		return nil
	}
	return fmt.Errorf("create redis consumer group: %w", err)
}

func (q *RedisQueue) PublishNodeExecution(ctx context.Context, event workflows.OutboxEvent) error {
	var payload struct {
		NodeExecutionID string `json:"node_execution_id"`
		WorkflowRunID   string `json:"workflow_run_id"`
		NodeName        string `json:"node_name"`
		WorkerPool      string `json:"worker_pool"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode outbox payload: %w", err)
	}
	workerPool := normalizeWorkerPool(payload.WorkerPool)
	if workerPool == workflows.WorkerPoolGeneral && payload.NodeName != "" {
		workerPool = workflows.WorkerPoolForNode(payload.NodeName)
	}
	stream := StreamNameForPool(workerPool)

	if err := ensureConsumerGroup(ctx, q.client, stream, q.group); err != nil {
		return err
	}

	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{
			"outbox_event_id":   fmt.Sprintf("%d", event.ID),
			"node_execution_id": payload.NodeExecutionID,
			"workflow_run_id":   payload.WorkflowRunID,
			"node_name":         payload.NodeName,
			"worker_pool":       workerPool,
		},
	}).Err()
}

func (q *RedisQueue) ReceiveNodeExecution(ctx context.Context, workerID string, wait time.Duration) (workflows.QueueMessage, error) {
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: workerID,
		Streams:  []string{q.streamName(), ">"},
		Count:    1,
		Block:    wait,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return workflows.QueueMessage{}, workflows.ErrNoQueueMessage
	}
	if err != nil {
		return workflows.QueueMessage{}, fmt.Errorf("read redis stream: %w", err)
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return workflows.QueueMessage{}, workflows.ErrNoQueueMessage
	}

	message := streams[0].Messages[0]
	return workflows.QueueMessage{
		ID:              message.ID,
		NodeExecutionID: stringValue(message.Values["node_execution_id"]),
		WorkflowRunID:   stringValue(message.Values["workflow_run_id"]),
		NodeName:        stringValue(message.Values["node_name"]),
		WorkerPool:      normalizeWorkerPool(stringValue(message.Values["worker_pool"])),
	}, nil
}

func (q *RedisQueue) AckNodeExecution(ctx context.Context, messageID string) error {
	if err := q.client.XAck(ctx, q.streamName(), q.group, messageID).Err(); err != nil {
		return fmt.Errorf("ack redis stream message: %w", err)
	}
	return nil
}

func (q *RedisQueue) streamName() string {
	return StreamNameForPool(q.workerPool)
}

func StreamNameForPool(workerPool string) string {
	return fmt.Sprintf("%s:%s", streamPrefix, normalizeWorkerPool(workerPool))
}

func ensureConsumerGroup(ctx context.Context, client *redis.Client, stream string, group string) error {
	err := client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err == nil {
		return nil
	}
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if isBusyGroup(err) {
		return nil
	}
	return fmt.Errorf("create redis consumer group for %s: %w", stream, err)
}

func isBusyGroup(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "BUSYGROUP")
}

func normalizeWorkerPool(workerPool string) string {
	workerPool = strings.TrimSpace(strings.ToLower(workerPool))
	if workerPool == "" {
		return workflows.WorkerPoolGeneral
	}
	return workerPool
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(value)
	}
}
