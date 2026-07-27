package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

type WorkflowRepository struct {
	db *sql.DB
}

func NewWorkflowRepository(db *sql.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) CreateRun(ctx context.Context, params workflows.CreateRunParams) (workflows.Run, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflows.Run{}, false, fmt.Errorf("begin create workflow run: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	run, err := insertRun(ctx, tx, params)
	if err == nil {
		if err := insertInitialNodeExecution(ctx, tx, run.ID, params.InitialNodeName); err != nil {
			return workflows.Run{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return workflows.Run{}, false, fmt.Errorf("commit create workflow run: %w", err)
		}
		return run, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return workflows.Run{}, false, err
	}

	run, err = getRunByIdempotencyKey(ctx, tx, params.IdempotencyKey)
	if err != nil {
		return workflows.Run{}, false, err
	}
	if run.RequestHash != params.RequestHash {
		return workflows.Run{}, false, workflows.ErrIdempotencyConflict
	}

	if err := tx.Commit(); err != nil {
		return workflows.Run{}, false, fmt.Errorf("commit replay workflow run: %w", err)
	}
	return run, false, nil
}

func (r *WorkflowRepository) GetRun(ctx context.Context, id string) (workflows.Run, error) {
	const query = `
SELECT id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, created_at, updated_at
FROM workflow_runs
WHERE id = $1`

	run, err := scanRun(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return workflows.Run{}, workflows.ErrNotFound
	}
	if err != nil {
		return workflows.Run{}, fmt.Errorf("get workflow run: %w", err)
	}
	return run, nil
}

func (r *WorkflowRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func insertRun(ctx context.Context, tx *sql.Tx, params workflows.CreateRunParams) (workflows.Run, error) {
	const query = `
INSERT INTO workflow_runs (
	id,
	workflow_name,
	status,
	input,
	idempotency_key,
	request_hash,
	correlation_id
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, created_at, updated_at`

	return scanRun(tx.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.WorkflowName,
		string(params.Status),
		[]byte(params.Input),
		params.IdempotencyKey,
		params.RequestHash,
		params.CorrelationID,
	))
}

func insertInitialNodeExecution(ctx context.Context, tx *sql.Tx, workflowRunID string, nodeName string) error {
	if nodeName == "" {
		nodeName = "phase3.bootstrap"
	}

	nodeID, err := newID()
	if err != nil {
		return fmt.Errorf("generate node execution id: %w", err)
	}

	const query = `
INSERT INTO node_executions (
	id,
	workflow_run_id,
	node_name,
	status
) VALUES ($1, $2, $3, $4)`

	if _, err := tx.ExecContext(ctx, query, nodeID, workflowRunID, nodeName, string(workflows.NodeStatusPending)); err != nil {
		return fmt.Errorf("insert initial node execution: %w", err)
	}
	return nil
}

func getRunByIdempotencyKey(ctx context.Context, tx *sql.Tx, idempotencyKey string) (workflows.Run, error) {
	const query = `
SELECT id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, created_at, updated_at
FROM workflow_runs
WHERE idempotency_key = $1`

	run, err := scanRun(tx.QueryRowContext(ctx, query, idempotencyKey))
	if err != nil {
		return workflows.Run{}, fmt.Errorf("get workflow run by idempotency key: %w", err)
	}
	return run, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (workflows.Run, error) {
	var run workflows.Run
	var status string
	var inputBytes []byte

	if err := row.Scan(
		&run.ID,
		&run.WorkflowName,
		&status,
		&inputBytes,
		&run.IdempotencyKey,
		&run.RequestHash,
		&run.CorrelationID,
		&run.CreatedAt,
		&run.UpdatedAt,
	); err != nil {
		return workflows.Run{}, err
	}

	run.Status = workflows.Status(status)
	run.Input = json.RawMessage(inputBytes)
	return run, nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullTime(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
