package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

func TestHealthz(t *testing.T) {
	response := request(t, testServer(t), http.MethodGet, "/healthz", nil, nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := decodeMap(t, response)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
	if response.Header().Get("X-Correlation-ID") == "" {
		t.Fatal("expected response correlation id")
	}
}

func TestReadyz(t *testing.T) {
	response := request(t, testServer(t), http.MethodGet, "/readyz", nil, nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := decodeMap(t, response)
	if body["status"] != "ready" {
		t.Fatalf("expected status ready, got %q", body["status"])
	}
}

func TestReadyzReturnsUnavailableWhenDependencyFails(t *testing.T) {
	server := NewServer(Options{
		Version: "test-version",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready: func(context.Context) error {
			return errors.New("database down")
		},
		Workflows: workflows.NewService(newFakeRepository()),
	})

	response := request(t, server, http.MethodGet, "/readyz", nil, nil)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, response.Code)
	}
}

func TestVersion(t *testing.T) {
	response := request(t, testServer(t), http.MethodGet, "/version", nil, nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := decodeMap(t, response)
	if body["name"] != serviceName {
		t.Fatalf("expected service name %q, got %q", serviceName, body["name"])
	}
	if body["version"] != "test-version" {
		t.Fatalf("expected version test-version, got %q", body["version"])
	}
}

func TestRoutesRejectInvalidMethods(t *testing.T) {
	response := request(t, testServer(t), http.MethodPost, "/healthz", nil, nil)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, allow)
	}
}

func TestCreateWorkflowRun(t *testing.T) {
	headers := map[string]string{
		"Idempotency-Key":  "demo-key",
		"X-Correlation-ID": "correlation-1",
		"Content-Type":     "application/json",
	}
	response := request(t, testServer(t), http.MethodPost, "/v1/workflow-runs", []byte(`{
		"workflow_name": "invoice-demo",
		"input": {"invoice_id": "synthetic-inv-001"}
	}`), headers)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	body := decodeWorkflowRunResponse(t, response)
	if body.WorkflowRun.ID == "" {
		t.Fatal("expected workflow run id")
	}
	if body.WorkflowRun.WorkflowName != "invoice-demo" {
		t.Fatalf("expected workflow name invoice-demo, got %q", body.WorkflowRun.WorkflowName)
	}
	if body.WorkflowRun.Status != workflows.StatusQueued {
		t.Fatalf("expected queued status, got %q", body.WorkflowRun.Status)
	}
	if body.WorkflowRun.CorrelationID != "correlation-1" {
		t.Fatalf("expected correlation id correlation-1, got %q", body.WorkflowRun.CorrelationID)
	}
	if body.Replayed {
		t.Fatal("expected newly created response")
	}
}

func TestCreateWorkflowRunReplaysSameIdempotencyKeyAndBody(t *testing.T) {
	server := testServer(t)
	headers := map[string]string{
		"Idempotency-Key": "demo-key",
		"Content-Type":    "application/json",
	}
	body := []byte(`{"workflow_name":"invoice-demo","input":{"invoice_id":"synthetic-inv-001"}}`)

	first := request(t, server, http.MethodPost, "/v1/workflow-runs", body, headers)
	second := request(t, server, http.MethodPost, "/v1/workflow-runs", body, headers)

	if first.Code != http.StatusCreated {
		t.Fatalf("expected first status %d, got %d", http.StatusCreated, first.Code)
	}
	if second.Code != http.StatusOK {
		t.Fatalf("expected second status %d, got %d", http.StatusOK, second.Code)
	}

	firstBody := decodeWorkflowRunResponse(t, first)
	secondBody := decodeWorkflowRunResponse(t, second)
	if firstBody.WorkflowRun.ID != secondBody.WorkflowRun.ID {
		t.Fatalf("expected same run id, got %q and %q", firstBody.WorkflowRun.ID, secondBody.WorkflowRun.ID)
	}
	if !secondBody.Replayed {
		t.Fatal("expected replayed response")
	}
}

func TestCreateWorkflowRunRejectsIdempotencyConflict(t *testing.T) {
	server := testServer(t)
	headers := map[string]string{
		"Idempotency-Key": "demo-key",
		"Content-Type":    "application/json",
	}

	first := request(t, server, http.MethodPost, "/v1/workflow-runs", []byte(`{"workflow_name":"invoice-demo","input":{"invoice_id":"one"}}`), headers)
	second := request(t, server, http.MethodPost, "/v1/workflow-runs", []byte(`{"workflow_name":"invoice-demo","input":{"invoice_id":"two"}}`), headers)

	if first.Code != http.StatusCreated {
		t.Fatalf("expected first status %d, got %d", http.StatusCreated, first.Code)
	}
	if second.Code != http.StatusConflict {
		t.Fatalf("expected conflict status %d, got %d", http.StatusConflict, second.Code)
	}
}

func TestCreateWorkflowRunRequiresIdempotencyKey(t *testing.T) {
	response := request(t, testServer(t), http.MethodPost, "/v1/workflow-runs", []byte(`{"workflow_name":"invoice-demo"}`), map[string]string{
		"Content-Type": "application/json",
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestGetWorkflowRun(t *testing.T) {
	server := testServer(t)
	headers := map[string]string{
		"Idempotency-Key": "demo-key",
		"Content-Type":    "application/json",
	}
	created := request(t, server, http.MethodPost, "/v1/workflow-runs", []byte(`{"workflow_name":"invoice-demo"}`), headers)
	createdBody := decodeWorkflowRunResponse(t, created)

	response := request(t, server, http.MethodGet, "/v1/workflow-runs/"+createdBody.WorkflowRun.ID, nil, nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := decodeWorkflowRunResponse(t, response)
	if body.WorkflowRun.ID != createdBody.WorkflowRun.ID {
		t.Fatalf("expected run id %q, got %q", createdBody.WorkflowRun.ID, body.WorkflowRun.ID)
	}
}

func TestGetWorkflowRunReturnsNotFound(t *testing.T) {
	response := request(t, testServer(t), http.MethodGet, "/v1/workflow-runs/missing", nil, nil)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func testServer(t *testing.T) http.Handler {
	t.Helper()

	repository := newFakeRepository()
	return NewServer(Options{
		Version:   "test-version",
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:     repository.Ping,
		Workflows: workflows.NewService(repository),
	})
}

func request(t *testing.T, server http.Handler, method string, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	return rec
}

func decodeMap(t *testing.T, response *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return body
}

func decodeWorkflowRunResponse(t *testing.T, response *httptest.ResponseRecorder) workflowRunResponse {
	t.Helper()

	var body workflowRunResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode workflow run response: %v; body=%s", err, response.Body.String())
	}
	return body
}

type fakeRepository struct {
	byID             map[string]workflows.Run
	byIdempotencyKey map[string]string
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		byID:             map[string]workflows.Run{},
		byIdempotencyKey: map[string]string{},
	}
}

func (r *fakeRepository) CreateRun(_ context.Context, params workflows.CreateRunParams) (workflows.Run, bool, error) {
	if existingID, ok := r.byIdempotencyKey[params.IdempotencyKey]; ok {
		run := r.byID[existingID]
		if run.RequestHash != params.RequestHash {
			return workflows.Run{}, false, workflows.ErrIdempotencyConflict
		}
		return run, false, nil
	}

	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	run := workflows.Run{
		ID:             params.ID,
		WorkflowName:   params.WorkflowName,
		Status:         params.Status,
		Input:          params.Input,
		IdempotencyKey: params.IdempotencyKey,
		RequestHash:    params.RequestHash,
		CorrelationID:  params.CorrelationID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	r.byID[run.ID] = run
	r.byIdempotencyKey[run.IdempotencyKey] = run.ID
	return run, true, nil
}

func (r *fakeRepository) GetRun(_ context.Context, id string) (workflows.Run, error) {
	run, ok := r.byID[id]
	if !ok {
		return workflows.Run{}, workflows.ErrNotFound
	}
	return run, nil
}

func (r *fakeRepository) Ping(context.Context) error {
	return nil
}
