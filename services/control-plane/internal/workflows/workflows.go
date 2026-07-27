package workflows

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusQueued          Status = "queued"
	StatusRunning         Status = "running"
	StatusWaitingForHuman Status = "waiting_for_human"
	StatusSucceeded       Status = "succeeded"
	StatusFailed          Status = "failed"
	StatusCanceled        Status = "canceled"
)

var (
	ErrInvalidInput          = errors.New("invalid workflow input")
	ErrInvalidHumanDecision  = errors.New("invalid human decision")
	ErrInvalidWorkflowName   = errors.New("workflow name is required")
	ErrMissingID             = errors.New("workflow run id is required")
	ErrMissingIdempotencyKey = errors.New("idempotency key is required")
	ErrNotFound              = errors.New("workflow run not found")
	ErrIdempotencyConflict   = errors.New("idempotency key was used with a different request")
	ErrDecisionConflict      = errors.New("decision key was used with a different decision")
	ErrWorkflowNotWaiting    = errors.New("workflow run is not waiting for a human decision")
)

type Run struct {
	ID             string
	WorkflowName   string
	Status         Status
	Input          json.RawMessage
	IdempotencyKey string
	RequestHash    string
	CorrelationID  string
	TraceParent    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AuditRecord struct {
	ID              int64
	WorkflowRunID   string
	NodeExecutionID string
	EventType       string
	ActorType       string
	ActorID         string
	Details         json.RawMessage
	CreatedAt       time.Time
}

type HumanDecision struct {
	ID            string
	WorkflowRunID string
	DecisionKey   string
	NodeName      string
	Decision      string
	ActorID       string
	Reason        string
	Details       json.RawMessage
	CreatedAt     time.Time
}

type CreateRunRequest struct {
	WorkflowName   string
	Input          json.RawMessage
	IdempotencyKey string
	CorrelationID  string
	TraceParent    string
}

type SubmitHumanDecisionRequest struct {
	WorkflowRunID string
	DecisionKey   string
	Decision      string
	ActorID       string
	Reason        string
}

type SubmitHumanDecisionParams struct {
	ID            string
	WorkflowRunID string
	DecisionKey   string
	Decision      string
	ActorID       string
	Reason        string
}

type HumanDecisionResult struct {
	Run          Run
	Decision     HumanDecision
	NextNodeName string
}

type CreateRunParams struct {
	ID              string
	WorkflowName    string
	Status          Status
	Input           json.RawMessage
	IdempotencyKey  string
	RequestHash     string
	CorrelationID   string
	TraceParent     string
	InitialNodeName string
}

type Repository interface {
	CreateRun(ctx context.Context, params CreateRunParams) (Run, bool, error)
	GetRun(ctx context.Context, id string) (Run, error)
	ListRuns(ctx context.Context, limit int) ([]Run, error)
	ListAuditRecords(ctx context.Context, workflowRunID string) ([]AuditRecord, error)
	SubmitHumanDecision(ctx context.Context, params SubmitHumanDecisionParams) (HumanDecisionResult, bool, error)
	Ping(ctx context.Context) error
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateRun(ctx context.Context, req CreateRunRequest) (Run, bool, error) {
	workflowName := strings.TrimSpace(req.WorkflowName)
	if workflowName == "" {
		return Run{}, false, ErrInvalidWorkflowName
	}
	if _, ok := DefinitionFor(workflowName); !ok {
		return Run{}, false, ErrUnknownWorkflow
	}

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		return Run{}, false, ErrMissingIdempotencyKey
	}

	input, err := canonicalInput(req.Input)
	if err != nil {
		return Run{}, false, err
	}

	id, err := newUUID()
	if err != nil {
		return Run{}, false, fmt.Errorf("generate workflow run id: %w", err)
	}

	params := CreateRunParams{
		ID:              id,
		WorkflowName:    workflowName,
		Status:          StatusQueued,
		Input:           input,
		IdempotencyKey:  idempotencyKey,
		RequestHash:     requestHash(workflowName, input),
		CorrelationID:   strings.TrimSpace(req.CorrelationID),
		TraceParent:     strings.TrimSpace(req.TraceParent),
		InitialNodeName: FirstNodeName(workflowName),
	}

	return s.repository.CreateRun(ctx, params)
}

func (s *Service) GetRun(ctx context.Context, id string) (Run, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Run{}, ErrMissingID
	}
	return s.repository.GetRun(ctx, id)
}

func (s *Service) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repository.ListRuns(ctx, limit)
}

func (s *Service) ListAuditRecords(ctx context.Context, workflowRunID string) ([]AuditRecord, error) {
	workflowRunID = strings.TrimSpace(workflowRunID)
	if workflowRunID == "" {
		return nil, ErrMissingID
	}
	return s.repository.ListAuditRecords(ctx, workflowRunID)
}

func (s *Service) SubmitHumanDecision(ctx context.Context, req SubmitHumanDecisionRequest) (HumanDecisionResult, bool, error) {
	workflowRunID := strings.TrimSpace(req.WorkflowRunID)
	if workflowRunID == "" {
		return HumanDecisionResult{}, false, ErrMissingID
	}

	decisionKey := strings.TrimSpace(req.DecisionKey)
	if decisionKey == "" {
		return HumanDecisionResult{}, false, ErrMissingIdempotencyKey
	}

	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if decision != "approved" && decision != "rejected" {
		return HumanDecisionResult{}, false, ErrInvalidHumanDecision
	}

	actorID := strings.TrimSpace(req.ActorID)
	if actorID == "" {
		return HumanDecisionResult{}, false, ErrInvalidHumanDecision
	}

	id, err := newUUID()
	if err != nil {
		return HumanDecisionResult{}, false, fmt.Errorf("generate human decision id: %w", err)
	}

	return s.repository.SubmitHumanDecision(ctx, SubmitHumanDecisionParams{
		ID:            id,
		WorkflowRunID: workflowRunID,
		DecisionKey:   decisionKey,
		Decision:      decision,
		ActorID:       actorID,
		Reason:        strings.TrimSpace(req.Reason),
	})
}

func canonicalInput(input json.RawMessage) (json.RawMessage, error) {
	if len(input) == 0 {
		return json.RawMessage(`{}`), nil
	}

	var value map[string]any
	if err := json.Unmarshal(input, &value); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if value == nil {
		return nil, ErrInvalidInput
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonicalize workflow input: %w", err)
	}
	return json.RawMessage(canonical), nil
}

func requestHash(workflowName string, input json.RawMessage) string {
	hash := sha256.New()
	hash.Write([]byte(workflowName))
	hash.Write([]byte{0})
	hash.Write(input)
	return hex.EncodeToString(hash.Sum(nil))
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
