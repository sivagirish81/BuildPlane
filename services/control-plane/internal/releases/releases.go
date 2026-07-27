package releases

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

const (
	IssueClassifierComponent = "issue_classifier"
	DefaultDatasetName       = "synthetic.issue-classifier.v1"
)

type ComponentStatus string

const (
	StatusCandidate  ComponentStatus = "candidate"
	StatusEvaluated  ComponentStatus = "evaluated"
	StatusCanary     ComponentStatus = "canary"
	StatusPromoted   ComponentStatus = "promoted"
	StatusSuperseded ComponentStatus = "superseded"
	StatusRolledBack ComponentStatus = "rolled_back"
)

var (
	ErrInvalidComponentVersion  = errors.New("invalid component version")
	ErrUnknownComponent         = errors.New("unknown component")
	ErrComponentVersionNotFound = errors.New("component version not found")
	ErrReleaseGate              = errors.New("release gate not satisfied")
	ErrInvalidCanaryPercent     = errors.New("invalid canary percent")
	ErrRollbackUnavailable      = errors.New("rollback target is unavailable")
)

type ComponentVersion struct {
	ID                        string
	ComponentName             string
	Version                   string
	PromptVersion             string
	Spec                      json.RawMessage
	Status                    ComponentStatus
	ChangeSummary             string
	CreatedBy                 string
	CanaryPercent             int
	EvaluationPassed          bool
	PreviousPromotedVersionID string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type CreateComponentVersionRequest struct {
	ComponentName string
	Version       string
	PromptVersion string
	Spec          json.RawMessage
	ChangeSummary string
	CreatedBy     string
}

type CreateComponentVersionParams struct {
	ID            string
	ComponentName string
	Version       string
	PromptVersion string
	Spec          json.RawMessage
	Status        ComponentStatus
	ChangeSummary string
	CreatedBy     string
}

type EvaluationRun struct {
	ID                   string
	ComponentVersionID   string
	ComponentName        string
	BaselineVersionID    string
	DatasetName          string
	Status               string
	CandidatePassed      bool
	BaselinePassed       bool
	CandidatePassedCases int
	CandidateTotalCases  int
	BaselinePassedCases  int
	BaselineTotalCases   int
	Summary              json.RawMessage
	CreatedAt            time.Time
}

type EvaluationResult struct {
	CaseName          string
	WorkflowName      string
	ExpectedCategory  string
	BaselineCategory  string
	CandidateCategory string
	BaselinePassed    bool
	CandidatePassed   bool
	Details           json.RawMessage
}

type RecordEvaluationParams struct {
	ID                 string
	ComponentVersionID string
	ComponentName      string
	BaselineVersionID  string
	DatasetName        string
	Status             string
	CandidatePassed    bool
	BaselinePassed     bool
	Results            []EvaluationResult
	Summary            json.RawMessage
}

type PromotionResult struct {
	Promoted ComponentVersion
	Previous ComponentVersion
}

type RollbackRequest struct {
	ComponentName string
	ActorID       string
	Reason        string
}

type Repository interface {
	CreateComponentVersion(ctx context.Context, params CreateComponentVersionParams) (ComponentVersion, error)
	GetComponentVersion(ctx context.Context, id string) (ComponentVersion, error)
	ListComponentVersions(ctx context.Context, componentName string, limit int) ([]ComponentVersion, error)
	LatestPromotedComponentVersion(ctx context.Context, componentName string) (ComponentVersion, error)
	RecordEvaluation(ctx context.Context, params RecordEvaluationParams) (EvaluationRun, error)
	StartCanary(ctx context.Context, id string, percent int) (ComponentVersion, error)
	Promote(ctx context.Context, id string) (PromotionResult, error)
	Rollback(ctx context.Context, req RollbackRequest) (PromotionResult, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateComponentVersion(ctx context.Context, req CreateComponentVersionRequest) (ComponentVersion, error) {
	componentName := strings.TrimSpace(req.ComponentName)
	if !isKnownComponent(componentName) {
		return ComponentVersion{}, ErrUnknownComponent
	}
	version := strings.TrimSpace(req.Version)
	promptVersion := strings.TrimSpace(req.PromptVersion)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if version == "" || promptVersion == "" || createdBy == "" {
		return ComponentVersion{}, ErrInvalidComponentVersion
	}

	spec, err := canonicalSpec(req.Spec)
	if err != nil {
		return ComponentVersion{}, err
	}
	id, err := newID()
	if err != nil {
		return ComponentVersion{}, fmt.Errorf("generate component version id: %w", err)
	}

	return s.repository.CreateComponentVersion(ctx, CreateComponentVersionParams{
		ID:            id,
		ComponentName: componentName,
		Version:       version,
		PromptVersion: promptVersion,
		Spec:          spec,
		Status:        StatusCandidate,
		ChangeSummary: strings.TrimSpace(req.ChangeSummary),
		CreatedBy:     createdBy,
	})
}

func (s *Service) GetComponentVersion(ctx context.Context, id string) (ComponentVersion, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ComponentVersion{}, ErrComponentVersionNotFound
	}
	return s.repository.GetComponentVersion(ctx, id)
}

func (s *Service) ListComponentVersions(ctx context.Context, componentName string, limit int) ([]ComponentVersion, error) {
	componentName = strings.TrimSpace(componentName)
	if !isKnownComponent(componentName) {
		return nil, ErrUnknownComponent
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	return s.repository.ListComponentVersions(ctx, componentName, limit)
}

func (s *Service) AffectedWorkflows(componentName string) ([]workflows.ComponentDependency, error) {
	componentName = strings.TrimSpace(componentName)
	if !isKnownComponent(componentName) {
		return nil, ErrUnknownComponent
	}
	return workflows.ComponentDependenciesFor(componentName), nil
}

func (s *Service) RunEvaluation(ctx context.Context, componentVersionID string) (EvaluationRun, error) {
	candidate, err := s.GetComponentVersion(ctx, componentVersionID)
	if err != nil {
		return EvaluationRun{}, err
	}
	if !isKnownComponent(candidate.ComponentName) {
		return EvaluationRun{}, ErrUnknownComponent
	}

	baseline, err := s.repository.LatestPromotedComponentVersion(ctx, candidate.ComponentName)
	if err != nil {
		return EvaluationRun{}, err
	}

	results, summary, candidatePassed, baselinePassed, err := evaluateIssueClassifier(candidate.Spec, baseline.Spec)
	if err != nil {
		return EvaluationRun{}, err
	}

	id, err := newID()
	if err != nil {
		return EvaluationRun{}, fmt.Errorf("generate evaluation id: %w", err)
	}
	status := "failed"
	if candidatePassed {
		status = "passed"
	}

	return s.repository.RecordEvaluation(ctx, RecordEvaluationParams{
		ID:                 id,
		ComponentVersionID: candidate.ID,
		ComponentName:      candidate.ComponentName,
		BaselineVersionID:  baseline.ID,
		DatasetName:        DefaultDatasetName,
		Status:             status,
		CandidatePassed:    candidatePassed,
		BaselinePassed:     baselinePassed,
		Results:            results,
		Summary:            summary,
	})
}

func (s *Service) StartCanary(ctx context.Context, id string, percent int) (ComponentVersion, error) {
	if percent < 1 || percent > 50 {
		return ComponentVersion{}, ErrInvalidCanaryPercent
	}
	return s.repository.StartCanary(ctx, strings.TrimSpace(id), percent)
}

func (s *Service) Promote(ctx context.Context, id string) (PromotionResult, error) {
	return s.repository.Promote(ctx, strings.TrimSpace(id))
}

func (s *Service) Rollback(ctx context.Context, req RollbackRequest) (PromotionResult, error) {
	req.ComponentName = strings.TrimSpace(req.ComponentName)
	req.ActorID = strings.TrimSpace(req.ActorID)
	req.Reason = strings.TrimSpace(req.Reason)
	if !isKnownComponent(req.ComponentName) {
		return PromotionResult{}, ErrUnknownComponent
	}
	return s.repository.Rollback(ctx, req)
}

func isKnownComponent(componentName string) bool {
	return componentName == IssueClassifierComponent
}

func canonicalSpec(spec json.RawMessage) (json.RawMessage, error) {
	if len(spec) == 0 {
		return nil, ErrInvalidComponentVersion
	}
	var value map[string]any
	if err := json.Unmarshal(spec, &value); err != nil {
		return nil, fmt.Errorf("%w: spec must be a JSON object", ErrInvalidComponentVersion)
	}
	if value == nil {
		return nil, ErrInvalidComponentVersion
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonicalize component spec: %w", err)
	}
	return json.RawMessage(canonical), nil
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
