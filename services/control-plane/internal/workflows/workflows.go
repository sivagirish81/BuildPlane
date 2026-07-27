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
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

var (
	ErrInvalidInput          = errors.New("invalid workflow input")
	ErrInvalidWorkflowName   = errors.New("workflow name is required")
	ErrMissingID             = errors.New("workflow run id is required")
	ErrMissingIdempotencyKey = errors.New("idempotency key is required")
	ErrNotFound              = errors.New("workflow run not found")
	ErrIdempotencyConflict   = errors.New("idempotency key was used with a different request")
)

type Run struct {
	ID             string
	WorkflowName   string
	Status         Status
	Input          json.RawMessage
	IdempotencyKey string
	RequestHash    string
	CorrelationID  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateRunRequest struct {
	WorkflowName   string
	Input          json.RawMessage
	IdempotencyKey string
	CorrelationID  string
}

type CreateRunParams struct {
	ID              string
	WorkflowName    string
	Status          Status
	Input           json.RawMessage
	IdempotencyKey  string
	RequestHash     string
	CorrelationID   string
	InitialNodeName string
}

type Repository interface {
	CreateRun(ctx context.Context, params CreateRunParams) (Run, bool, error)
	GetRun(ctx context.Context, id string) (Run, error)
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
		InitialNodeName: "phase3.bootstrap",
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
