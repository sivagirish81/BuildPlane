package queue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/buildplane/buildplane/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Queue struct {
	client *redis.Client
}

func New(addr, password string) *Queue {
	return &Queue{client: redis.NewClient(&redis.Options{Addr: addr, Password: password})}
}

func (q *Queue) Close() error {
	return q.client.Close()
}

func (q *Queue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

func key(priority domain.Priority, tenantID string) string {
	return fmt.Sprintf("buildplane:ready:%s:%s", priority, tenantID)
}

func tenantsKey(priority domain.Priority) string {
	return fmt.Sprintf("buildplane:ready-tenants:%s", priority)
}

func (q *Queue) Enqueue(ctx context.Context, job domain.Job) error {
	score := float64(time.Now().UnixMilli())
	member := fmt.Sprintf("%s:%d", job.ID, job.Version)
	pipe := q.client.Pipeline()
	pipe.ZAdd(ctx, key(job.Priority, job.TenantID), redis.Z{Score: score, Member: member})
	pipe.SAdd(ctx, tenantsKey(job.Priority), job.TenantID)
	_, err := pipe.Exec(ctx)
	return err
}

func (q *Queue) Remove(ctx context.Context, priority domain.Priority, tenantID string, jobID uuid.UUID) error {
	members, err := q.client.ZRange(ctx, key(priority, tenantID), 0, -1).Result()
	if err != nil {
		return err
	}
	prefix := jobID.String() + ":"
	for _, member := range members {
		if strings.HasPrefix(member, prefix) {
			if err := q.client.ZRem(ctx, key(priority, tenantID), member).Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

type Candidate struct {
	JobID    uuid.UUID
	Version  int
	TenantID string
	Priority domain.Priority
	QueuedAt time.Time
}

func (q *Queue) Candidates(ctx context.Context, priorities []domain.Priority, perTenant int) ([]Candidate, error) {
	var candidates []Candidate
	for _, priority := range priorities {
		tenants, err := q.client.SMembers(ctx, tenantsKey(priority)).Result()
		if err != nil {
			return nil, err
		}
		for _, tenantID := range tenants {
			items, err := q.client.ZRangeWithScores(ctx, key(priority, tenantID), 0, int64(perTenant-1)).Result()
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				parts := strings.SplitN(fmt.Sprint(item.Member), ":", 2)
				if len(parts) != 2 {
					continue
				}
				id, err := uuid.Parse(parts[0])
				if err != nil {
					continue
				}
				version, _ := strconv.Atoi(parts[1])
				candidates = append(candidates, Candidate{
					JobID:    id,
					Version:  version,
					TenantID: tenantID,
					Priority: priority,
					QueuedAt: time.UnixMilli(int64(item.Score)),
				})
			}
		}
	}
	return candidates, nil
}

func (q *Queue) Depth(ctx context.Context, priority domain.Priority) (int64, error) {
	tenants, err := q.client.SMembers(ctx, tenantsKey(priority)).Result()
	if err != nil {
		return 0, err
	}
	var depth int64
	for _, tenantID := range tenants {
		n, err := q.client.ZCard(ctx, key(priority, tenantID)).Result()
		if err != nil {
			return 0, err
		}
		depth += n
	}
	return depth, nil
}
