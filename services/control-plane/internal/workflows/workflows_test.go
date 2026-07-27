package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCreateRunStoresCanonicalInputAndHash(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)

	run, created, err := service.CreateRun(context.Background(), CreateRunRequest{
		WorkflowName:   " phase4.local-demo ",
		Input:          json.RawMessage(`{"b":2,"a":1}`),
		IdempotencyKey: " demo-key ",
		CorrelationID:  "correlation-1",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if !created {
		t.Fatal("expected created run")
	}
	if run.WorkflowName != "phase4.local-demo" {
		t.Fatalf("expected trimmed workflow name, got %q", run.WorkflowName)
	}
	if string(run.Input) != `{"a":1,"b":2}` {
		t.Fatalf("expected canonical input, got %s", run.Input)
	}
	if run.RequestHash == "" {
		t.Fatal("expected request hash")
	}
}

func TestCreateRunRejectsMissingWorkflowName(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, _, err := service.CreateRun(context.Background(), CreateRunRequest{
		IdempotencyKey: "demo-key",
	})
	if !errors.Is(err, ErrInvalidWorkflowName) {
		t.Fatalf("expected ErrInvalidWorkflowName, got %v", err)
	}
}

func TestCreateRunRejectsMissingIdempotencyKey(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, _, err := service.CreateRun(context.Background(), CreateRunRequest{
		WorkflowName: "phase4.local-demo",
	})
	if !errors.Is(err, ErrMissingIdempotencyKey) {
		t.Fatalf("expected ErrMissingIdempotencyKey, got %v", err)
	}
}

func TestCreateRunRejectsNonObjectInput(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, _, err := service.CreateRun(context.Background(), CreateRunRequest{
		WorkflowName:   "phase4.local-demo",
		Input:          json.RawMessage(`[]`),
		IdempotencyKey: "demo-key",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRunRejectsMissingID(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, err := service.GetRun(context.Background(), " ")
	if !errors.Is(err, ErrMissingID) {
		t.Fatalf("expected ErrMissingID, got %v", err)
	}
}

func TestSubmitHumanDecisionValidatesDecision(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, _, err := service.SubmitHumanDecision(context.Background(), SubmitHumanDecisionRequest{
		WorkflowRunID: "run-1",
		DecisionKey:   "decision-1",
		Decision:      "maybe",
		ActorID:       "operator-1",
	})
	if !errors.Is(err, ErrInvalidHumanDecision) {
		t.Fatalf("expected ErrInvalidHumanDecision, got %v", err)
	}
}

func TestSubmitHumanDecisionRequiresActor(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, _, err := service.SubmitHumanDecision(context.Background(), SubmitHumanDecisionRequest{
		WorkflowRunID: "run-1",
		DecisionKey:   "decision-1",
		Decision:      "approved",
	})
	if !errors.Is(err, ErrInvalidHumanDecision) {
		t.Fatalf("expected ErrInvalidHumanDecision, got %v", err)
	}
}

type fakeRepository struct {
	run Run
}

func (r *fakeRepository) CreateRun(_ context.Context, params CreateRunParams) (Run, bool, error) {
	r.run = Run{
		ID:             params.ID,
		WorkflowName:   params.WorkflowName,
		Status:         params.Status,
		Input:          params.Input,
		IdempotencyKey: params.IdempotencyKey,
		RequestHash:    params.RequestHash,
		CorrelationID:  params.CorrelationID,
	}
	return r.run, true, nil
}

func (r *fakeRepository) GetRun(_ context.Context, id string) (Run, error) {
	if r.run.ID == id {
		return r.run, nil
	}
	return Run{}, ErrNotFound
}

func (r *fakeRepository) ListAuditRecords(context.Context, string) ([]AuditRecord, error) {
	return nil, nil
}

func (r *fakeRepository) SubmitHumanDecision(_ context.Context, params SubmitHumanDecisionParams) (HumanDecisionResult, bool, error) {
	decision := HumanDecision{
		ID:            params.ID,
		WorkflowRunID: params.WorkflowRunID,
		DecisionKey:   params.DecisionKey,
		NodeName:      "await_human_approval",
		Decision:      params.Decision,
		ActorID:       params.ActorID,
		Reason:        params.Reason,
	}
	return HumanDecisionResult{
		Run: Run{
			ID:           params.WorkflowRunID,
			WorkflowName: InvoiceExceptionWorkflowName,
			Status:       StatusQueued,
		},
		Decision:     decision,
		NextNodeName: "record_mock_action",
	}, true, nil
}

func (r *fakeRepository) Ping(context.Context) error {
	return nil
}
