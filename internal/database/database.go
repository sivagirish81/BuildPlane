package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/buildplane/buildplane/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	migration, err := os.ReadFile("migrations/0001_init.sql")
	if err != nil {
		return fmt.Errorf("read migrations/0001_init.sql: %w", err)
	}
	_, err = s.pool.Exec(ctx, string(migration))
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

type WorkflowInput struct {
	TenantID string                 `json:"tenant_id"`
	Name     string                 `json:"name"`
	Jobs     []domain.JobDefinition `json:"jobs"`
}

type CreateRunInput struct {
	WorkflowID     uuid.UUID
	RepositoryURL  string
	CommitSHA      string
	Priority       domain.Priority
	IdempotencyKey string
}

func (s *Store) CreateWorkflow(ctx context.Context, input WorkflowInput) (domain.Workflow, error) {
	if input.TenantID == "" || input.Name == "" || len(input.Jobs) == 0 {
		return domain.Workflow{}, errors.New("tenant_id, name, and at least one job are required")
	}
	for i := range input.Jobs {
		if input.Jobs[i].RequestedCPUMillis <= 0 {
			input.Jobs[i].RequestedCPUMillis = 1000
		}
		if input.Jobs[i].RequestedMemoryMB <= 0 {
			input.Jobs[i].RequestedMemoryMB = 512
		}
		if input.Jobs[i].MaxAttempts <= 0 {
			input.Jobs[i].MaxAttempts = 1
		}
		if input.Jobs[i].ActiveDeadlineSeconds <= 0 {
			input.Jobs[i].ActiveDeadlineSeconds = 1800
		}
	}
	definition, err := json.Marshal(input.Jobs)
	if err != nil {
		return domain.Workflow{}, fmt.Errorf("marshal workflow definition: %w", err)
	}
	var workflow domain.Workflow
	err = s.pool.QueryRow(ctx, `
INSERT INTO workflows(tenant_id, name, definition)
VALUES ($1, $2, $3)
RETURNING id, tenant_id, name, definition, created_at, updated_at`,
		input.TenantID, input.Name, definition).Scan(&workflow.ID, &workflow.TenantID, &workflow.Name, &definition, &workflow.CreatedAt, &workflow.UpdatedAt)
	if err != nil {
		return domain.Workflow{}, fmt.Errorf("create workflow: %w", err)
	}
	if err := json.Unmarshal(definition, &workflow.Definition); err != nil {
		return domain.Workflow{}, fmt.Errorf("decode workflow definition: %w", err)
	}
	return workflow, nil
}

func (s *Store) CreateRun(ctx context.Context, input CreateRunInput) (domain.WorkflowRun, []domain.Job, bool, error) {
	if input.WorkflowID == uuid.Nil || input.RepositoryURL == "" || input.CommitSHA == "" || input.IdempotencyKey == "" {
		return domain.WorkflowRun{}, nil, false, errors.New("workflow_id, repository_url, commit_sha, and idempotency key are required")
	}
	if err := domain.ValidatePriority(input.Priority); err != nil {
		return domain.WorkflowRun{}, nil, false, err
	}
	if err := validateRepositoryURL(input.RepositoryURL); err != nil {
		return domain.WorkflowRun{}, nil, false, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.WorkflowRun{}, nil, false, fmt.Errorf("begin create run tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var existingID uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT id FROM workflow_runs WHERE workflow_id=$1 AND idempotency_key=$2`,
		input.WorkflowID, input.IdempotencyKey).Scan(&existingID)
	if err == nil {
		run, jobs, getErr := s.getRunTx(ctx, tx, existingID)
		if getErr != nil {
			return domain.WorkflowRun{}, nil, false, getErr
		}
		return run, jobs, true, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowRun{}, nil, false, fmt.Errorf("check idempotency: %w", err)
	}

	var workflow domain.Workflow
	var definition []byte
	err = tx.QueryRow(ctx, `SELECT id, tenant_id, name, definition, created_at, updated_at FROM workflows WHERE id=$1`,
		input.WorkflowID).Scan(&workflow.ID, &workflow.TenantID, &workflow.Name, &definition, &workflow.CreatedAt, &workflow.UpdatedAt)
	if err != nil {
		return domain.WorkflowRun{}, nil, false, fmt.Errorf("load workflow: %w", err)
	}
	if err := json.Unmarshal(definition, &workflow.Definition); err != nil {
		return domain.WorkflowRun{}, nil, false, fmt.Errorf("decode workflow: %w", err)
	}

	var run domain.WorkflowRun
	err = tx.QueryRow(ctx, `
INSERT INTO workflow_runs(workflow_id, tenant_id, repository_url, commit_sha, status, priority, idempotency_key)
VALUES($1, $2, $3, $4, $5, $6, $7)
RETURNING id, workflow_id, tenant_id, repository_url, commit_sha, status, priority, idempotency_key, created_at, started_at, completed_at, version`,
		workflow.ID, workflow.TenantID, input.RepositoryURL, input.CommitSHA, domain.StatusQueued, input.Priority, input.IdempotencyKey).
		Scan(&run.ID, &run.WorkflowID, &run.TenantID, &run.RepositoryURL, &run.CommitSHA, &run.Status, &run.Priority,
			&run.IdempotencyKey, &run.CreatedAt, &run.StartedAt, &run.CompletedAt, &run.Version)
	if err != nil {
		return domain.WorkflowRun{}, nil, false, fmt.Errorf("insert run: %w", err)
	}

	jobs := make([]domain.Job, 0, len(workflow.Definition))
	for _, def := range workflow.Definition {
		commands, _ := json.Marshal(def.Commands)
		cacheSpec, _ := json.Marshal(def.Cache)
		var job domain.Job
		err := tx.QueryRow(ctx, `
INSERT INTO jobs(workflow_run_id, tenant_id, name, status, priority, commands, cache, requested_cpu_millis, requested_memory_mb, max_attempts, queued_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
RETURNING id, workflow_run_id, tenant_id, name, status, priority, commands, cache, requested_cpu_millis, requested_memory_mb, max_attempts, attempt_count, queued_at, scheduled_at, started_at, completed_at, version`,
			run.ID, run.TenantID, def.Name, domain.StatusQueued, input.Priority, commands, cacheSpec, def.RequestedCPUMillis, def.RequestedMemoryMB, def.MaxAttempts).
			Scan(&job.ID, &job.WorkflowRunID, &job.TenantID, &job.Name, &job.Status, &job.Priority, &commands, &cacheSpec,
				&job.RequestedCPUMillis, &job.RequestedMemoryMB, &job.MaxAttempts, &job.AttemptCount, &job.QueuedAt,
				&job.ScheduledAt, &job.StartedAt, &job.CompletedAt, &job.Version)
		if err != nil {
			return domain.WorkflowRun{}, nil, false, fmt.Errorf("insert job: %w", err)
		}
		_ = json.Unmarshal(commands, &job.Commands)
		_ = json.Unmarshal(cacheSpec, &job.Cache)
		job.RepositoryURL = run.RepositoryURL
		job.CommitSHA = run.CommitSHA
		jobs = append(jobs, job)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WorkflowRun{}, nil, false, fmt.Errorf("commit create run: %w", err)
	}
	return run, jobs, false, nil
}

func validateRepositoryURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid repository_url: %w", err)
	}
	if parsed.Scheme == "https" && parsed.Host != "" {
		return nil
	}
	if parsed.Scheme == "fixture" && parsed.Host != "" {
		return nil
	}
	if parsed.Scheme == "file" || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "/") {
		return nil
	}
	return errors.New("repository_url must be https, fixture, file, or local path for the MVP")
}

func (s *Store) GetRun(ctx context.Context, id uuid.UUID) (domain.WorkflowRun, []domain.Job, []domain.Attempt, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.WorkflowRun{}, nil, nil, err
	}
	defer tx.Rollback(ctx)
	run, jobs, err := s.getRunTx(ctx, tx, id)
	if err != nil {
		return domain.WorkflowRun{}, nil, nil, err
	}
	attempts, err := s.ListAttemptsForRun(ctx, id)
	if err != nil {
		return domain.WorkflowRun{}, nil, nil, err
	}
	return run, jobs, attempts, tx.Commit(ctx)
}

func (s *Store) getRunTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (domain.WorkflowRun, []domain.Job, error) {
	var run domain.WorkflowRun
	err := tx.QueryRow(ctx, `
SELECT id, workflow_id, tenant_id, repository_url, commit_sha, status, priority, idempotency_key, created_at, started_at, completed_at, version
FROM workflow_runs WHERE id=$1`, id).
		Scan(&run.ID, &run.WorkflowID, &run.TenantID, &run.RepositoryURL, &run.CommitSHA, &run.Status, &run.Priority,
			&run.IdempotencyKey, &run.CreatedAt, &run.StartedAt, &run.CompletedAt, &run.Version)
	if err != nil {
		return domain.WorkflowRun{}, nil, fmt.Errorf("get run: %w", err)
	}
	rows, err := tx.Query(ctx, `
SELECT j.id, j.workflow_run_id, j.tenant_id, j.name, j.status, j.priority, j.commands, j.cache, j.requested_cpu_millis, j.requested_memory_mb,
       j.max_attempts, j.attempt_count, j.queued_at, j.scheduled_at, j.started_at, j.completed_at, j.version,
       r.repository_url, r.commit_sha
FROM jobs j JOIN workflow_runs r ON r.id=j.workflow_run_id
WHERE workflow_run_id=$1 ORDER BY name`, id)
	if err != nil {
		return domain.WorkflowRun{}, nil, err
	}
	defer rows.Close()
	jobs, err := scanJobs(rows)
	return run, jobs, err
}

func scanJobs(rows pgx.Rows) ([]domain.Job, error) {
	var jobs []domain.Job
	for rows.Next() {
		var job domain.Job
		var commands []byte
		var cacheSpec []byte
		err := rows.Scan(&job.ID, &job.WorkflowRunID, &job.TenantID, &job.Name, &job.Status, &job.Priority, &commands, &cacheSpec,
			&job.RequestedCPUMillis, &job.RequestedMemoryMB, &job.MaxAttempts, &job.AttemptCount,
			&job.QueuedAt, &job.ScheduledAt, &job.StartedAt, &job.CompletedAt, &job.Version, &job.RepositoryURL, &job.CommitSHA)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal(commands, &job.Commands)
		_ = json.Unmarshal(cacheSpec, &job.Cache)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) ListQueuedJobs(ctx context.Context, limit int) ([]domain.Job, error) {
	rows, err := s.pool.Query(ctx, `
SELECT j.id, j.workflow_run_id, j.tenant_id, j.name, j.status, j.priority, j.commands, j.cache, j.requested_cpu_millis, j.requested_memory_mb,
       j.max_attempts, j.attempt_count, j.queued_at, j.scheduled_at, j.started_at, j.completed_at, j.version,
       r.repository_url, r.commit_sha
FROM jobs j JOIN workflow_runs r ON r.id=j.workflow_run_id
WHERE j.status=$1 AND j.queued_at <= now()
ORDER BY j.priority, j.queued_at
LIMIT $2`, domain.StatusQueued, limit)
	if err != nil {
		return nil, fmt.Errorf("list queued jobs: %w", err)
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *Store) GetJobs(ctx context.Context, ids []uuid.UUID) ([]domain.Job, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
SELECT j.id, j.workflow_run_id, j.tenant_id, j.name, j.status, j.priority, j.commands, j.cache, j.requested_cpu_millis, j.requested_memory_mb,
       j.max_attempts, j.attempt_count, j.queued_at, j.scheduled_at, j.started_at, j.completed_at, j.version,
       r.repository_url, r.commit_sha
FROM jobs j JOIN workflow_runs r ON r.id=j.workflow_run_id
WHERE j.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *Store) Tenants(ctx context.Context) (map[string]domain.Tenant, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, scheduling_weight, max_concurrent_jobs, created_at FROM tenants`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tenants := map[string]domain.Tenant{}
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.SchedulingWeight, &t.MaxConcurrentJobs, &t.CreatedAt); err != nil {
			return nil, err
		}
		tenants[t.ID] = t
	}
	return tenants, rows.Err()
}

func (s *Store) CountQueued(ctx context.Context, tenantID string) (int, error) {
	var count int
	var err error
	if tenantID == "" {
		err = s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE status=$1`, domain.StatusQueued).Scan(&count)
	} else {
		err = s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE status=$1 AND tenant_id=$2`, domain.StatusQueued, tenantID).Scan(&count)
	}
	return count, err
}

func (s *Store) ActiveCounts(ctx context.Context) (int, map[string]int, int, int, error) {
	rows, err := s.pool.Query(ctx, `
SELECT tenant_id, count(*), COALESCE(sum(requested_cpu_millis),0), COALESCE(sum(requested_memory_mb),0)
FROM jobs WHERE status IN ($1, $2, $3)
GROUP BY tenant_id`, domain.StatusScheduling, domain.StatusScheduled, domain.StatusRunning)
	if err != nil {
		return 0, nil, 0, 0, err
	}
	defer rows.Close()
	perTenant := map[string]int{}
	total, cpu, mem := 0, 0, 0
	for rows.Next() {
		var tenant string
		var count, tenantCPU, tenantMem int
		if err := rows.Scan(&tenant, &count, &tenantCPU, &tenantMem); err != nil {
			return 0, nil, 0, 0, err
		}
		perTenant[tenant] = count
		total += count
		cpu += tenantCPU
		mem += tenantMem
	}
	return total, perTenant, cpu, mem, rows.Err()
}

func (s *Store) ClaimJob(ctx context.Context, jobID uuid.UUID, expectedVersion int) (domain.Job, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Job{}, false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
UPDATE jobs SET status=$1, scheduled_at=now(), version=version+1
WHERE id=$2 AND status=$3 AND version=$4`, domain.StatusScheduling, jobID, domain.StatusQueued, expectedVersion)
	if err != nil {
		return domain.Job{}, false, fmt.Errorf("claim job: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.Job{}, false, tx.Commit(ctx)
	}
	run, jobs, err := s.getRunForJobTx(ctx, tx, jobID)
	if err != nil {
		return domain.Job{}, false, err
	}
	_ = run
	return jobs[0], true, tx.Commit(ctx)
}

func (s *Store) getRunForJobTx(ctx context.Context, tx pgx.Tx, jobID uuid.UUID) (domain.WorkflowRun, []domain.Job, error) {
	var runID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT workflow_run_id FROM jobs WHERE id=$1`, jobID).Scan(&runID); err != nil {
		return domain.WorkflowRun{}, nil, err
	}
	run, jobs, err := s.getRunTx(ctx, tx, runID)
	if err != nil {
		return domain.WorkflowRun{}, nil, err
	}
	for _, job := range jobs {
		if job.ID == jobID {
			return run, []domain.Job{job}, nil
		}
	}
	return domain.WorkflowRun{}, nil, errors.New("claimed job disappeared")
}

func (s *Store) CreateAttempt(ctx context.Context, job domain.Job, kubeJobName, leaseHash string, leaseExpires time.Time) (domain.Attempt, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Attempt{}, err
	}
	defer tx.Rollback(ctx)
	var attempt domain.Attempt
	err = tx.QueryRow(ctx, `
UPDATE jobs SET attempt_count=attempt_count+1, status=$1, version=version+1
WHERE id=$2 AND status=$3
RETURNING attempt_count`, domain.StatusScheduled, job.ID, domain.StatusScheduling).Scan(&attempt.AttemptNumber)
	if err != nil {
		return domain.Attempt{}, fmt.Errorf("advance job scheduled: %w", err)
	}
	err = tx.QueryRow(ctx, `
INSERT INTO job_attempts(job_id, attempt_number, status, kubernetes_job_name, lease_token_hash, lease_expires_at)
VALUES($1, $2, $3, $4, $5, $6)
RETURNING id, job_id, attempt_number, status, kubernetes_job_name, lease_token_hash, lease_expires_at, last_heartbeat_at, exit_code, failure_reason, started_at, completed_at`,
		job.ID, attempt.AttemptNumber, domain.StatusScheduled, kubeJobName, leaseHash, leaseExpires).
		Scan(&attempt.ID, &attempt.JobID, &attempt.AttemptNumber, &attempt.Status, &attempt.KubernetesJobName, &attempt.LeaseTokenHash,
			&attempt.LeaseExpiresAt, &attempt.LastHeartbeatAt, &attempt.ExitCode, &attempt.FailureReason, &attempt.StartedAt, &attempt.CompletedAt)
	if err != nil {
		return domain.Attempt{}, fmt.Errorf("create attempt: %w", err)
	}
	return attempt, tx.Commit(ctx)
}

func (s *Store) MarkAttemptRunning(ctx context.Context, attemptID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
UPDATE job_attempts SET status=$1, last_heartbeat_at=now() WHERE id=$2 AND status=$3;
UPDATE jobs SET status=$1, started_at=COALESCE(started_at, now()), version=version+1 WHERE id=(SELECT job_id FROM job_attempts WHERE id=$2) AND status=$3;
UPDATE workflow_runs SET status=$1, started_at=COALESCE(started_at, now()), version=version+1 WHERE id=(SELECT workflow_run_id FROM jobs WHERE id=(SELECT job_id FROM job_attempts WHERE id=$2)) AND status=$4`,
		domain.StatusRunning, attemptID, domain.StatusScheduled, domain.StatusQueued)
	return err
}

func (s *Store) RenewLease(ctx context.Context, attemptID uuid.UUID, nextExpiry time.Time) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE job_attempts SET lease_expires_at=$1, last_heartbeat_at=now()
WHERE id=$2 AND status IN ($3, $4) AND lease_expires_at > now()`,
		nextExpiry, attemptID, domain.StatusScheduled, domain.StatusRunning)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("lease renewal rejected")
	}
	return nil
}

func (s *Store) AttemptByID(ctx context.Context, attemptID uuid.UUID) (domain.Attempt, error) {
	var attempt domain.Attempt
	err := s.pool.QueryRow(ctx, `
SELECT id, job_id, attempt_number, status, kubernetes_job_name, lease_token_hash, lease_expires_at, last_heartbeat_at, exit_code, failure_reason, started_at, completed_at
FROM job_attempts WHERE id=$1`, attemptID).
		Scan(&attempt.ID, &attempt.JobID, &attempt.AttemptNumber, &attempt.Status, &attempt.KubernetesJobName, &attempt.LeaseTokenHash,
			&attempt.LeaseExpiresAt, &attempt.LastHeartbeatAt, &attempt.ExitCode, &attempt.FailureReason, &attempt.StartedAt, &attempt.CompletedAt)
	return attempt, err
}

func (s *Store) LatestAttemptNumber(ctx context.Context, jobID uuid.UUID) (int, error) {
	var number int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(max(attempt_number),0) FROM job_attempts WHERE job_id=$1`, jobID).Scan(&number)
	return number, err
}

func (s *Store) AppendLogs(ctx context.Context, attemptID uuid.UUID, chunk string) error {
	if chunk == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO job_logs(job_attempt_id, chunk) VALUES($1, $2)`, attemptID, chunk)
	return err
}

func (s *Store) JobLogs(ctx context.Context, jobID uuid.UUID) (string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT l.chunk FROM job_logs l
JOIN job_attempts a ON a.id=l.job_attempt_id
WHERE a.job_id=$1 ORDER BY l.id`, jobID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var chunk string
		if err := rows.Scan(&chunk); err != nil {
			return "", err
		}
		b.WriteString(chunk)
	}
	return b.String(), rows.Err()
}

func (s *Store) CompleteAttempt(ctx context.Context, attemptID uuid.UUID, status domain.JobStatus, exitCode int, failureReason string) error {
	if !domain.Terminal(status) {
		return errors.New("completion status must be terminal")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var jobID uuid.UUID
	var startedAt time.Time
	tag, err := tx.Exec(ctx, `
UPDATE job_attempts SET status=$1, exit_code=$2, failure_reason=$3, completed_at=now()
WHERE id=$4 AND status IN ($5, $6) AND lease_expires_at > now()`,
		status, exitCode, nullableFailure(failureReason), attemptID, domain.StatusScheduled, domain.StatusRunning)
	if err != nil {
		return fmt.Errorf("complete attempt: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("stale or expired attempt completion rejected")
	}
	if err := tx.QueryRow(ctx, `SELECT job_id, started_at FROM job_attempts WHERE id=$1`, attemptID).Scan(&jobID, &startedAt); err != nil {
		return err
	}
	var maxAttempts, attemptCount int
	if err := tx.QueryRow(ctx, `SELECT max_attempts, attempt_count FROM jobs WHERE id=$1`, jobID).Scan(&maxAttempts, &attemptCount); err != nil {
		return err
	}
	nextStatus := status
	if status == domain.StatusFailed && failureReason == "infrastructure" && attemptCount < maxAttempts {
		nextStatus = domain.StatusQueued
		_, err = tx.Exec(ctx, `UPDATE jobs SET status=$1, queued_at=now() + interval '2 seconds', version=version+1 WHERE id=$2`,
			nextStatus, jobID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE jobs SET status=$1, completed_at=now(), version=version+1 WHERE id=$2`, nextStatus, jobID)
	}
	if err != nil {
		return err
	}
	var runID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT workflow_run_id FROM jobs WHERE id=$1`, jobID).Scan(&runID); err != nil {
		return err
	}
	if nextStatus != domain.StatusQueued {
		if err := finalizeRunIfDone(ctx, tx, runID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func nullableFailure(reason string) *string {
	if reason == "" {
		return nil
	}
	return &reason
}

func finalizeRunIfDone(ctx context.Context, tx pgx.Tx, runID uuid.UUID) error {
	rows, err := tx.Query(ctx, `SELECT status FROM jobs WHERE workflow_run_id=$1`, runID)
	if err != nil {
		return err
	}
	defer rows.Close()
	allDone := true
	anyFailed := false
	for rows.Next() {
		var status domain.JobStatus
		if err := rows.Scan(&status); err != nil {
			return err
		}
		if !domain.Terminal(status) {
			allDone = false
		}
		if status != domain.StatusSucceeded {
			anyFailed = true
		}
	}
	if err := rows.Err(); err != nil || !allDone {
		return err
	}
	status := domain.StatusSucceeded
	if anyFailed {
		status = domain.StatusFailed
	}
	_, err = tx.Exec(ctx, `UPDATE workflow_runs SET status=$1, completed_at=now(), version=version+1 WHERE id=$2`, status, runID)
	return err
}

func (s *Store) ListAttemptsForRun(ctx context.Context, runID uuid.UUID) ([]domain.Attempt, error) {
	rows, err := s.pool.Query(ctx, `
SELECT a.id, a.job_id, a.attempt_number, a.status, a.kubernetes_job_name, a.lease_token_hash, a.lease_expires_at, a.last_heartbeat_at,
       a.exit_code, a.failure_reason, a.started_at, a.completed_at
FROM job_attempts a JOIN jobs j ON j.id=a.job_id
WHERE j.workflow_run_id=$1 ORDER BY j.name, a.attempt_number`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attempts []domain.Attempt
	for rows.Next() {
		var a domain.Attempt
		if err := rows.Scan(&a.ID, &a.JobID, &a.AttemptNumber, &a.Status, &a.KubernetesJobName, &a.LeaseTokenHash,
			&a.LeaseExpiresAt, &a.LastHeartbeatAt, &a.ExitCode, &a.FailureReason, &a.StartedAt, &a.CompletedAt); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

func (s *Store) ExpireLeases(ctx context.Context) ([]domain.Job, error) {
	rows, err := s.pool.Query(ctx, `
WITH expired AS (
  UPDATE job_attempts SET status=$1, failure_reason='lease expired', completed_at=now()
  WHERE status IN ($2, $3) AND lease_expires_at <= now()
  RETURNING job_id
), requeued AS (
  UPDATE jobs j SET status=CASE WHEN j.attempt_count < j.max_attempts THEN $4 ELSE $5 END,
                    queued_at=CASE WHEN j.attempt_count < j.max_attempts THEN now() ELSE j.queued_at END,
                    completed_at=CASE WHEN j.attempt_count < j.max_attempts THEN NULL ELSE now() END,
                    version=version+1
  FROM expired e WHERE j.id=e.job_id
  RETURNING j.id, j.workflow_run_id, j.tenant_id, j.name, j.status, j.priority, j.commands, j.cache, j.requested_cpu_millis, j.requested_memory_mb,
            j.max_attempts, j.attempt_count, j.queued_at, j.scheduled_at, j.started_at, j.completed_at, j.version,
            (SELECT repository_url FROM workflow_runs WHERE id=j.workflow_run_id),
            (SELECT commit_sha FROM workflow_runs WHERE id=j.workflow_run_id)
)
SELECT * FROM requeued WHERE status=$4`,
		domain.StatusLost, domain.StatusScheduled, domain.StatusRunning, domain.StatusQueued, domain.StatusFailed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx, `
UPDATE workflow_runs r SET status=CASE
    WHEN EXISTS (SELECT 1 FROM jobs j WHERE j.workflow_run_id=r.id AND j.status <> $1) THEN $2
    ELSE $1
  END,
  completed_at=now(),
  version=version+1
WHERE r.status NOT IN ($1, $2, $3, $4)
  AND NOT EXISTS (
    SELECT 1 FROM jobs j
    WHERE j.workflow_run_id=r.id AND j.status NOT IN ($1, $2, $3, $4)
  )`,
		domain.StatusSucceeded, domain.StatusFailed, domain.StatusTimedOut, domain.StatusCancelled)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *Store) CancelRun(ctx context.Context, runID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
WITH affected_jobs AS (
  UPDATE jobs SET status=$1, completed_at=now(), version=version+1
  WHERE workflow_run_id=$2 AND status NOT IN ($1, $3, $4, $5)
  RETURNING id
), affected_attempts AS (
  UPDATE job_attempts SET status=$1, failure_reason='cancelled', completed_at=now()
  WHERE job_id IN (SELECT id FROM affected_jobs) AND status IN ($6, $7, $8)
  RETURNING kubernetes_job_name
)
SELECT kubernetes_job_name FROM affected_attempts`,
		domain.StatusCancelled, runID, domain.StatusSucceeded, domain.StatusFailed, domain.StatusTimedOut,
		domain.StatusScheduling, domain.StatusScheduled, domain.StatusRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	_, err = s.pool.Exec(ctx, `UPDATE workflow_runs SET status=$1, completed_at=now(), version=version+1 WHERE id=$2`, domain.StatusCancelled, runID)
	return names, err
}

func (s *Store) UpsertCacheEntry(ctx context.Context, tenantID, cacheKey, objectKey, checksum string, size int64, expiresAt *time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO cache_entries(cache_key, tenant_id, object_key, checksum, size_bytes, expires_at)
VALUES($1, $2, $3, $4, $5, $6)
ON CONFLICT(cache_key) DO UPDATE SET object_key=EXCLUDED.object_key, checksum=EXCLUDED.checksum, size_bytes=EXCLUDED.size_bytes, last_accessed_at=now(), expires_at=EXCLUDED.expires_at`,
		cacheKey, tenantID, objectKey, checksum, size, expiresAt)
	return err
}

func (s *Store) CacheEntry(ctx context.Context, tenantID, cacheKey string) (string, string, int64, bool, error) {
	var objectKey, checksum string
	var size int64
	err := s.pool.QueryRow(ctx, `
UPDATE cache_entries SET last_accessed_at=now()
WHERE tenant_id=$1 AND cache_key=$2 AND (expires_at IS NULL OR expires_at > now())
RETURNING object_key, checksum, size_bytes`, tenantID, cacheKey).Scan(&objectKey, &checksum, &size)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", 0, false, nil
	}
	return objectKey, checksum, size, err == nil, err
}
