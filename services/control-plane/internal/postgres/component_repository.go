package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sivagirish/buildplane/services/control-plane/internal/releases"
)

func (r *WorkflowRepository) CreateComponentVersion(ctx context.Context, params releases.CreateComponentVersionParams) (releases.ComponentVersion, error) {
	const query = `
INSERT INTO component_versions (
	id,
	component_name,
	version,
	prompt_version,
	spec,
	status,
	change_summary,
	created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at`

	version, err := scanComponentVersion(r.db.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.ComponentName,
		params.Version,
		params.PromptVersion,
		[]byte(params.Spec),
		string(params.Status),
		params.ChangeSummary,
		params.CreatedBy,
	))
	if err != nil {
		return releases.ComponentVersion{}, fmt.Errorf("create component version: %w", err)
	}
	return version, nil
}

func (r *WorkflowRepository) GetComponentVersion(ctx context.Context, id string) (releases.ComponentVersion, error) {
	const query = `
SELECT id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at
FROM component_versions
WHERE id = $1`

	version, err := scanComponentVersion(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return releases.ComponentVersion{}, releases.ErrComponentVersionNotFound
	}
	if err != nil {
		return releases.ComponentVersion{}, fmt.Errorf("get component version: %w", err)
	}
	return version, nil
}

func (r *WorkflowRepository) LatestPromotedComponentVersion(ctx context.Context, componentName string) (releases.ComponentVersion, error) {
	const query = `
SELECT id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at
FROM component_versions
WHERE component_name = $1
	AND status = 'promoted'
ORDER BY updated_at DESC
LIMIT 1`

	version, err := scanComponentVersion(r.db.QueryRowContext(ctx, query, componentName))
	if errors.Is(err, sql.ErrNoRows) {
		return releases.ComponentVersion{}, releases.ErrComponentVersionNotFound
	}
	if err != nil {
		return releases.ComponentVersion{}, fmt.Errorf("latest promoted component version: %w", err)
	}
	return version, nil
}

func (r *WorkflowRepository) RecordEvaluation(ctx context.Context, params releases.RecordEvaluationParams) (releases.EvaluationRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return releases.EvaluationRun{}, fmt.Errorf("begin record evaluation: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	candidatePassedCases, candidateTotalCases, baselinePassedCases, baselineTotalCases := countEvaluationResults(params.Results)
	const insertRun = `
INSERT INTO component_evaluation_runs (
	id,
	component_version_id,
	component_name,
	baseline_version_id,
	dataset_name,
	status,
	candidate_passed,
	baseline_passed,
	candidate_passed_cases,
	candidate_total_cases,
	baseline_passed_cases,
	baseline_total_cases,
	summary
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, component_version_id, component_name, baseline_version_id, dataset_name, status, candidate_passed, baseline_passed, candidate_passed_cases, candidate_total_cases, baseline_passed_cases, baseline_total_cases, summary, created_at`

	run, err := scanEvaluationRun(tx.QueryRowContext(
		ctx,
		insertRun,
		params.ID,
		params.ComponentVersionID,
		params.ComponentName,
		params.BaselineVersionID,
		params.DatasetName,
		params.Status,
		params.CandidatePassed,
		params.BaselinePassed,
		candidatePassedCases,
		candidateTotalCases,
		baselinePassedCases,
		baselineTotalCases,
		[]byte(params.Summary),
	))
	if err != nil {
		return releases.EvaluationRun{}, fmt.Errorf("insert evaluation run: %w", err)
	}

	for _, result := range params.Results {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO component_evaluation_results (
	evaluation_run_id,
	case_name,
	workflow_name,
	expected_category,
	baseline_category,
	candidate_category,
	baseline_passed,
	candidate_passed,
	details
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			params.ID,
			result.CaseName,
			result.WorkflowName,
			result.ExpectedCategory,
			result.BaselineCategory,
			result.CandidateCategory,
			result.BaselinePassed,
			result.CandidatePassed,
			[]byte(result.Details),
		); err != nil {
			return releases.EvaluationRun{}, fmt.Errorf("insert evaluation result: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE component_versions
SET status = 'evaluated',
	evaluation_passed = $2,
	updated_at = now()
WHERE id = $1`, params.ComponentVersionID, params.CandidatePassed); err != nil {
		return releases.EvaluationRun{}, fmt.Errorf("mark component evaluated: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO component_release_events (component_name, component_version_id, event_type, actor_id, details)
VALUES ($1, $2, 'component_version.evaluated', 'system', $3)`,
		params.ComponentName,
		params.ComponentVersionID,
		[]byte(params.Summary),
	); err != nil {
		return releases.EvaluationRun{}, fmt.Errorf("insert evaluation release event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return releases.EvaluationRun{}, fmt.Errorf("commit record evaluation: %w", err)
	}
	return run, nil
}

func (r *WorkflowRepository) StartCanary(ctx context.Context, id string, percent int) (releases.ComponentVersion, error) {
	const query = `
UPDATE component_versions
SET status = 'canary',
	canary_percent = $2,
	updated_at = now()
WHERE id = $1
	AND status IN ('evaluated', 'canary')
	AND evaluation_passed = true
RETURNING id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at`

	version, err := scanComponentVersion(r.db.QueryRowContext(ctx, query, id, percent))
	if errors.Is(err, sql.ErrNoRows) {
		return releases.ComponentVersion{}, releases.ErrReleaseGate
	}
	if err != nil {
		return releases.ComponentVersion{}, fmt.Errorf("start canary: %w", err)
	}
	return version, nil
}

func (r *WorkflowRepository) Promote(ctx context.Context, id string) (releases.PromotionResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return releases.PromotionResult{}, fmt.Errorf("begin promote component: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	candidate, err := getComponentVersionForUpdate(ctx, tx, id)
	if err != nil {
		return releases.PromotionResult{}, err
	}
	if candidate.Status != releases.StatusCanary || !candidate.EvaluationPassed {
		return releases.PromotionResult{}, releases.ErrReleaseGate
	}

	previous, err := getPromotedComponentVersionForUpdate(ctx, tx, candidate.ComponentName)
	if err != nil {
		return releases.PromotionResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE component_versions
SET status = 'superseded',
	canary_percent = 0,
	updated_at = now()
WHERE id = $1`, previous.ID); err != nil {
		return releases.PromotionResult{}, fmt.Errorf("supersede previous component version: %w", err)
	}
	previous.Status = releases.StatusSuperseded
	previous.CanaryPercent = 0

	promoted, err := scanComponentVersion(tx.QueryRowContext(ctx, `
UPDATE component_versions
SET status = 'promoted',
	canary_percent = 0,
	previous_promoted_version_id = $2,
	updated_at = now()
WHERE id = $1
RETURNING id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at`, candidate.ID, previous.ID))
	if err != nil {
		return releases.PromotionResult{}, fmt.Errorf("promote component version: %w", err)
	}

	if err := insertComponentReleaseEvent(ctx, tx, promoted.ComponentName, promoted.ID, "component_version.promoted", promoted.CreatedBy, map[string]any{
		"previous_promoted_version_id": previous.ID,
	}); err != nil {
		return releases.PromotionResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return releases.PromotionResult{}, fmt.Errorf("commit promote component: %w", err)
	}
	return releases.PromotionResult{Promoted: promoted, Previous: previous}, nil
}

func (r *WorkflowRepository) Rollback(ctx context.Context, req releases.RollbackRequest) (releases.PromotionResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return releases.PromotionResult{}, fmt.Errorf("begin rollback component: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := getPromotedComponentVersionForUpdate(ctx, tx, req.ComponentName)
	if err != nil {
		return releases.PromotionResult{}, err
	}
	if current.PreviousPromotedVersionID == "" {
		return releases.PromotionResult{}, releases.ErrRollbackUnavailable
	}

	previous, err := getComponentVersionForUpdate(ctx, tx, current.PreviousPromotedVersionID)
	if err != nil {
		return releases.PromotionResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE component_versions
SET status = 'rolled_back',
	canary_percent = 0,
	updated_at = now()
WHERE id = $1`, current.ID); err != nil {
		return releases.PromotionResult{}, fmt.Errorf("mark current component rolled back: %w", err)
	}
	current.Status = releases.StatusRolledBack
	current.CanaryPercent = 0

	restored, err := scanComponentVersion(tx.QueryRowContext(ctx, `
UPDATE component_versions
SET status = 'promoted',
	canary_percent = 0,
	previous_promoted_version_id = NULL,
	updated_at = now()
WHERE id = $1
RETURNING id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at`, previous.ID))
	if err != nil {
		return releases.PromotionResult{}, fmt.Errorf("restore previous component version: %w", err)
	}

	if err := insertComponentReleaseEvent(ctx, tx, restored.ComponentName, restored.ID, "component_version.rollback", req.ActorID, map[string]any{
		"rolled_back_version_id": current.ID,
		"reason":                 req.Reason,
	}); err != nil {
		return releases.PromotionResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return releases.PromotionResult{}, fmt.Errorf("commit rollback component: %w", err)
	}
	return releases.PromotionResult{Promoted: restored, Previous: current}, nil
}

func countEvaluationResults(results []releases.EvaluationResult) (int, int, int, int) {
	candidatePassed := 0
	baselinePassed := 0
	for _, result := range results {
		if result.CandidatePassed {
			candidatePassed++
		}
		if result.BaselinePassed {
			baselinePassed++
		}
	}
	return candidatePassed, len(results), baselinePassed, len(results)
}

func getComponentVersionForUpdate(ctx context.Context, tx *sql.Tx, id string) (releases.ComponentVersion, error) {
	const query = `
SELECT id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at
FROM component_versions
WHERE id = $1
FOR UPDATE`

	version, err := scanComponentVersion(tx.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return releases.ComponentVersion{}, releases.ErrComponentVersionNotFound
	}
	if err != nil {
		return releases.ComponentVersion{}, fmt.Errorf("get component version for update: %w", err)
	}
	return version, nil
}

func getPromotedComponentVersionForUpdate(ctx context.Context, tx *sql.Tx, componentName string) (releases.ComponentVersion, error) {
	const query = `
SELECT id, component_name, version, prompt_version, spec, status, change_summary, created_by, canary_percent, evaluation_passed, previous_promoted_version_id, created_at, updated_at
FROM component_versions
WHERE component_name = $1
	AND status = 'promoted'
ORDER BY updated_at DESC
LIMIT 1
FOR UPDATE`

	version, err := scanComponentVersion(tx.QueryRowContext(ctx, query, componentName))
	if errors.Is(err, sql.ErrNoRows) {
		return releases.ComponentVersion{}, releases.ErrRollbackUnavailable
	}
	if err != nil {
		return releases.ComponentVersion{}, fmt.Errorf("get promoted component version for update: %w", err)
	}
	return version, nil
}

func insertComponentReleaseEvent(ctx context.Context, tx *sql.Tx, componentName string, componentVersionID string, eventType string, actorID string, details map[string]any) error {
	payload, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encode component release event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO component_release_events (component_name, component_version_id, event_type, actor_id, details)
VALUES ($1, $2, $3, $4, $5)`, componentName, componentVersionID, eventType, actorID, []byte(payload)); err != nil {
		return fmt.Errorf("insert component release event: %w", err)
	}
	return nil
}

func scanComponentVersion(row rowScanner) (releases.ComponentVersion, error) {
	var version releases.ComponentVersion
	var spec []byte
	var status string
	var previous sql.NullString
	if err := row.Scan(
		&version.ID,
		&version.ComponentName,
		&version.Version,
		&version.PromptVersion,
		&spec,
		&status,
		&version.ChangeSummary,
		&version.CreatedBy,
		&version.CanaryPercent,
		&version.EvaluationPassed,
		&previous,
		&version.CreatedAt,
		&version.UpdatedAt,
	); err != nil {
		return releases.ComponentVersion{}, err
	}
	version.Spec = json.RawMessage(spec)
	version.Status = releases.ComponentStatus(status)
	version.PreviousPromotedVersionID = nullString(previous)
	return version, nil
}

func scanEvaluationRun(row rowScanner) (releases.EvaluationRun, error) {
	var run releases.EvaluationRun
	var summary []byte
	if err := row.Scan(
		&run.ID,
		&run.ComponentVersionID,
		&run.ComponentName,
		&run.BaselineVersionID,
		&run.DatasetName,
		&run.Status,
		&run.CandidatePassed,
		&run.BaselinePassed,
		&run.CandidatePassedCases,
		&run.CandidateTotalCases,
		&run.BaselinePassedCases,
		&run.BaselineTotalCases,
		&summary,
		&run.CreatedAt,
	); err != nil {
		return releases.EvaluationRun{}, err
	}
	run.Summary = json.RawMessage(summary)
	return run, nil
}
