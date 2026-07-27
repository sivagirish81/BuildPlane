package releases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestEvaluationCanaryPromotionAndRollback(t *testing.T) {
	repository := newFakeRepository()
	service := NewService(repository)

	candidate, err := service.CreateComponentVersion(context.Background(), CreateComponentVersionRequest{
		ComponentName: IssueClassifierComponent,
		Version:       "v2",
		PromptVersion: "issue_classifier_v2",
		Spec:          passingSpec(),
		ChangeSummary: "Adds explicit logistics keywords",
		CreatedBy:     "operator-1",
	})
	if err != nil {
		t.Fatalf("create component version: %v", err)
	}
	if candidate.Status != StatusCandidate {
		t.Fatalf("expected candidate status, got %q", candidate.Status)
	}

	run, err := service.RunEvaluation(context.Background(), candidate.ID)
	if err != nil {
		t.Fatalf("run evaluation: %v", err)
	}
	if !run.CandidatePassed {
		t.Fatal("expected candidate evaluation to pass")
	}
	if run.CandidatePassedCases != 3 || run.CandidateTotalCases != 3 {
		t.Fatalf("expected 3/3 candidate cases, got %d/%d", run.CandidatePassedCases, run.CandidateTotalCases)
	}

	canary, err := service.StartCanary(context.Background(), candidate.ID, 10)
	if err != nil {
		t.Fatalf("start canary: %v", err)
	}
	if canary.Status != StatusCanary || canary.CanaryPercent != 10 {
		t.Fatalf("expected 10 percent canary, got status=%q percent=%d", canary.Status, canary.CanaryPercent)
	}

	promotion, err := service.Promote(context.Background(), candidate.ID)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if promotion.Promoted.Status != StatusPromoted {
		t.Fatalf("expected promoted status, got %q", promotion.Promoted.Status)
	}
	if promotion.Previous.ID != "issue-classifier-v1" {
		t.Fatalf("expected v1 previous, got %q", promotion.Previous.ID)
	}

	rollback, err := service.Rollback(context.Background(), RollbackRequest{
		ComponentName: IssueClassifierComponent,
		ActorID:       "operator-1",
		Reason:        "synthetic rollback exercise",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rollback.Promoted.ID != "issue-classifier-v1" {
		t.Fatalf("expected v1 restored, got %q", rollback.Promoted.ID)
	}
	if rollback.Previous.ID != candidate.ID {
		t.Fatalf("expected candidate rolled back, got %q", rollback.Previous.ID)
	}
}

func TestEvaluationFailsCandidateWithMissingCategory(t *testing.T) {
	repository := newFakeRepository()
	service := NewService(repository)

	candidate, err := service.CreateComponentVersion(context.Background(), CreateComponentVersionRequest{
		ComponentName: IssueClassifierComponent,
		Version:       "v2",
		PromptVersion: "issue_classifier_v2",
		Spec:          json.RawMessage(`{"rules":[{"category":"billing","keywords":["invoice"]}],"default_category":"general"}`),
		CreatedBy:     "operator-1",
	})
	if err != nil {
		t.Fatalf("create component version: %v", err)
	}

	run, err := service.RunEvaluation(context.Background(), candidate.ID)
	if err != nil {
		t.Fatalf("run evaluation: %v", err)
	}
	if run.CandidatePassed {
		t.Fatal("expected candidate evaluation to fail")
	}
	if _, err := service.StartCanary(context.Background(), candidate.ID, 10); !errors.Is(err, ErrReleaseGate) {
		t.Fatalf("expected ErrReleaseGate, got %v", err)
	}
}

func TestAffectedWorkflows(t *testing.T) {
	service := NewService(newFakeRepository())

	dependencies, err := service.AffectedWorkflows(IssueClassifierComponent)
	if err != nil {
		t.Fatalf("affected workflows: %v", err)
	}
	if len(dependencies) != 3 {
		t.Fatalf("expected three affected workflows, got %d", len(dependencies))
	}
}

func passingSpec() json.RawMessage {
	return json.RawMessage(`{
		"rules": [
			{"category": "billing", "keywords": ["invoice", "payment", "charge"]},
			{"category": "logistics", "keywords": ["freight", "shipment", "carrier", "delivery"]},
			{"category": "access", "keywords": ["access", "login", "portal"]}
		],
		"default_category": "general"
	}`)
}

type fakeRepository struct {
	versions map[string]ComponentVersion
}

func newFakeRepository() *fakeRepository {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	return &fakeRepository{
		versions: map[string]ComponentVersion{
			"issue-classifier-v1": {
				ID:               "issue-classifier-v1",
				ComponentName:    IssueClassifierComponent,
				Version:          "v1",
				PromptVersion:    "issue_classifier_v1",
				Spec:             passingSpec(),
				Status:           StatusPromoted,
				CreatedBy:        "system",
				EvaluationPassed: true,
				CreatedAt:        now,
				UpdatedAt:        now,
			},
		},
	}
}

func (r *fakeRepository) CreateComponentVersion(_ context.Context, params CreateComponentVersionParams) (ComponentVersion, error) {
	now := time.Date(2026, 7, 27, 12, 1, 0, 0, time.UTC)
	version := ComponentVersion{
		ID:            params.ID,
		ComponentName: params.ComponentName,
		Version:       params.Version,
		PromptVersion: params.PromptVersion,
		Spec:          params.Spec,
		Status:        params.Status,
		ChangeSummary: params.ChangeSummary,
		CreatedBy:     params.CreatedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.versions[version.ID] = version
	return version, nil
}

func (r *fakeRepository) GetComponentVersion(_ context.Context, id string) (ComponentVersion, error) {
	version, ok := r.versions[id]
	if !ok {
		return ComponentVersion{}, ErrComponentVersionNotFound
	}
	return version, nil
}

func (r *fakeRepository) LatestPromotedComponentVersion(_ context.Context, componentName string) (ComponentVersion, error) {
	for _, version := range r.versions {
		if version.ComponentName == componentName && version.Status == StatusPromoted {
			return version, nil
		}
	}
	return ComponentVersion{}, ErrComponentVersionNotFound
}

func (r *fakeRepository) RecordEvaluation(_ context.Context, params RecordEvaluationParams) (EvaluationRun, error) {
	candidatePassedCases := 0
	baselinePassedCases := 0
	for _, result := range params.Results {
		if result.CandidatePassed {
			candidatePassedCases++
		}
		if result.BaselinePassed {
			baselinePassedCases++
		}
	}
	version := r.versions[params.ComponentVersionID]
	version.Status = StatusEvaluated
	version.EvaluationPassed = params.CandidatePassed
	r.versions[version.ID] = version

	return EvaluationRun{
		ID:                   params.ID,
		ComponentVersionID:   params.ComponentVersionID,
		ComponentName:        params.ComponentName,
		BaselineVersionID:    params.BaselineVersionID,
		DatasetName:          params.DatasetName,
		Status:               params.Status,
		CandidatePassed:      params.CandidatePassed,
		BaselinePassed:       params.BaselinePassed,
		CandidatePassedCases: candidatePassedCases,
		CandidateTotalCases:  len(params.Results),
		BaselinePassedCases:  baselinePassedCases,
		BaselineTotalCases:   len(params.Results),
		Summary:              params.Summary,
	}, nil
}

func (r *fakeRepository) StartCanary(_ context.Context, id string, percent int) (ComponentVersion, error) {
	version, ok := r.versions[id]
	if !ok {
		return ComponentVersion{}, ErrComponentVersionNotFound
	}
	if version.Status != StatusEvaluated || !version.EvaluationPassed {
		return ComponentVersion{}, ErrReleaseGate
	}
	version.Status = StatusCanary
	version.CanaryPercent = percent
	r.versions[id] = version
	return version, nil
}

func (r *fakeRepository) Promote(_ context.Context, id string) (PromotionResult, error) {
	candidate, ok := r.versions[id]
	if !ok {
		return PromotionResult{}, ErrComponentVersionNotFound
	}
	if candidate.Status != StatusCanary || !candidate.EvaluationPassed {
		return PromotionResult{}, ErrReleaseGate
	}
	previous, err := r.LatestPromotedComponentVersion(context.Background(), candidate.ComponentName)
	if err != nil {
		return PromotionResult{}, err
	}
	previous.Status = StatusSuperseded
	r.versions[previous.ID] = previous
	candidate.Status = StatusPromoted
	candidate.CanaryPercent = 0
	candidate.PreviousPromotedVersionID = previous.ID
	r.versions[candidate.ID] = candidate
	return PromotionResult{Promoted: candidate, Previous: previous}, nil
}

func (r *fakeRepository) Rollback(_ context.Context, req RollbackRequest) (PromotionResult, error) {
	current, err := r.LatestPromotedComponentVersion(context.Background(), req.ComponentName)
	if err != nil {
		return PromotionResult{}, err
	}
	if current.PreviousPromotedVersionID == "" {
		return PromotionResult{}, ErrRollbackUnavailable
	}
	previous := r.versions[current.PreviousPromotedVersionID]
	current.Status = StatusRolledBack
	previous.Status = StatusPromoted
	previous.PreviousPromotedVersionID = ""
	r.versions[current.ID] = current
	r.versions[previous.ID] = previous
	return PromotionResult{Promoted: previous, Previous: current}, nil
}
