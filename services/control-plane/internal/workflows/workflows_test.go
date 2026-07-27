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
		WorkflowName:   " invoice-demo ",
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
	if run.WorkflowName != "invoice-demo" {
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
		WorkflowName: "invoice-demo",
	})
	if !errors.Is(err, ErrMissingIdempotencyKey) {
		t.Fatalf("expected ErrMissingIdempotencyKey, got %v", err)
	}
}

func TestCreateRunRejectsNonObjectInput(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, _, err := service.CreateRun(context.Background(), CreateRunRequest{
		WorkflowName:   "invoice-demo",
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

func (r *fakeRepository) Ping(context.Context) error {
	return nil
}
