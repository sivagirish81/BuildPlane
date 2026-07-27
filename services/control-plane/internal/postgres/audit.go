package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type auditRecordParams struct {
	WorkflowRunID   string
	NodeExecutionID string
	EventType       string
	ActorType       string
	ActorID         string
	Details         map[string]any
}

func insertAuditRecord(ctx context.Context, tx *sql.Tx, params auditRecordParams) error {
	details := params.Details
	if details == nil {
		details = map[string]any{}
	}

	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encode audit details: %w", err)
	}

	var nodeExecutionID any
	if params.NodeExecutionID != "" {
		nodeExecutionID = params.NodeExecutionID
	}

	const query = `
INSERT INTO audit_records (
	workflow_run_id,
	node_execution_id,
	event_type,
	actor_type,
	actor_id,
	details
) VALUES ($1, $2, $3, $4, $5, $6)`

	if _, err := tx.ExecContext(
		ctx,
		query,
		params.WorkflowRunID,
		nodeExecutionID,
		params.EventType,
		params.ActorType,
		params.ActorID,
		detailsBytes,
	); err != nil {
		return fmt.Errorf("insert audit record %s: %w", params.EventType, err)
	}

	return nil
}
