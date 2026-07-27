package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/releases"
	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

func (s *Server) componentVersions(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/component-versions" {
		writeError(w, r, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if r.Method == http.MethodGet {
		s.componentVersionList(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if s.releases == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "release service is not configured")
		return
	}

	var request createComponentVersionRequest
	if err := decodeStrictJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	version, err := s.releases.CreateComponentVersion(r.Context(), releases.CreateComponentVersionRequest{
		ComponentName: request.ComponentName,
		Version:       request.Version,
		PromptVersion: request.PromptVersion,
		Spec:          request.Spec,
		ChangeSummary: request.ChangeSummary,
		CreatedBy:     request.CreatedBy,
	})
	if err != nil {
		s.writeReleaseError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, componentVersionResponse{
		ComponentVersion: toComponentVersionDTO(version),
	})
}

func (s *Server) componentVersionList(w http.ResponseWriter, r *http.Request) {
	if s.releases == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "release service is not configured")
		return
	}

	versions, err := s.releases.ListComponentVersions(
		r.Context(),
		strings.TrimSpace(r.URL.Query().Get("component_name")),
		queryLimit(r, 25),
	)
	if err != nil {
		s.writeReleaseError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, componentVersionListResponse{
		ComponentVersions: toComponentVersionDTOs(versions),
	})
}

func (s *Server) componentVersionByID(w http.ResponseWriter, r *http.Request) {
	if s.releases == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "release service is not configured")
		return
	}

	suffix := strings.TrimPrefix(r.URL.Path, "/v1/component-versions/")
	if suffix == "" {
		writeError(w, r, http.StatusNotFound, "not_found", "component version not found")
		return
	}

	if strings.HasSuffix(suffix, "/evaluations") {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		id := strings.TrimSuffix(suffix, "/evaluations")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, r, http.StatusNotFound, "not_found", "component version not found")
			return
		}
		run, err := s.releases.RunEvaluation(r.Context(), id)
		if err != nil {
			s.writeReleaseError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, evaluationRunResponse{EvaluationRun: toEvaluationRunDTO(run)})
		return
	}

	if strings.HasSuffix(suffix, "/canary") {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		id := strings.TrimSuffix(suffix, "/canary")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, r, http.StatusNotFound, "not_found", "component version not found")
			return
		}
		var request startCanaryRequest
		if err := decodeStrictJSON(w, r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		version, err := s.releases.StartCanary(r.Context(), id, request.Percent)
		if err != nil {
			s.writeReleaseError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, componentVersionResponse{ComponentVersion: toComponentVersionDTO(version)})
		return
	}

	if strings.HasSuffix(suffix, "/promote") {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		id := strings.TrimSuffix(suffix, "/promote")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, r, http.StatusNotFound, "not_found", "component version not found")
			return
		}
		result, err := s.releases.Promote(r.Context(), id)
		if err != nil {
			s.writeReleaseError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, promotionResponse{Promotion: toPromotionDTO(result)})
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if strings.Contains(suffix, "/") {
		writeError(w, r, http.StatusNotFound, "not_found", "component version not found")
		return
	}
	version, err := s.releases.GetComponentVersion(r.Context(), suffix)
	if err != nil {
		s.writeReleaseError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, componentVersionResponse{ComponentVersion: toComponentVersionDTO(version)})
}

func (s *Server) componentByName(w http.ResponseWriter, r *http.Request) {
	if s.releases == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "release service is not configured")
		return
	}

	suffix := strings.TrimPrefix(r.URL.Path, "/v1/components/")
	if suffix == "" {
		writeError(w, r, http.StatusNotFound, "not_found", "component not found")
		return
	}

	if strings.HasSuffix(suffix, "/affected-workflows") {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		name := strings.TrimSuffix(suffix, "/affected-workflows")
		if name == "" || strings.Contains(name, "/") {
			writeError(w, r, http.StatusNotFound, "not_found", "component not found")
			return
		}
		dependencies, err := s.releases.AffectedWorkflows(name)
		if err != nil {
			s.writeReleaseError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, affectedWorkflowsResponse{AffectedWorkflows: dependencies})
		return
	}

	if strings.HasSuffix(suffix, "/rollback") {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		name := strings.TrimSuffix(suffix, "/rollback")
		if name == "" || strings.Contains(name, "/") {
			writeError(w, r, http.StatusNotFound, "not_found", "component not found")
			return
		}
		var request rollbackRequest
		if err := decodeStrictJSON(w, r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		result, err := s.releases.Rollback(r.Context(), releases.RollbackRequest{
			ComponentName: name,
			ActorID:       request.ActorID,
			Reason:        request.Reason,
		})
		if err != nil {
			s.writeReleaseError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, promotionResponse{Promotion: toPromotionDTO(result)})
		return
	}

	writeError(w, r, http.StatusNotFound, "not_found", "component route not found")
}

func (s *Server) writeReleaseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, releases.ErrUnknownComponent):
		writeError(w, r, http.StatusBadRequest, "unknown_component", "component is not supported")
	case errors.Is(err, releases.ErrInvalidComponentVersion):
		writeError(w, r, http.StatusBadRequest, "invalid_component_version", "component version request is invalid")
	case errors.Is(err, releases.ErrComponentVersionNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "component version not found")
	case errors.Is(err, releases.ErrInvalidCanaryPercent):
		writeError(w, r, http.StatusBadRequest, "invalid_canary_percent", "canary percent must be between 1 and 50")
	case errors.Is(err, releases.ErrReleaseGate):
		writeError(w, r, http.StatusConflict, "release_gate_not_satisfied", "evaluation and canary gates must pass before this transition")
	case errors.Is(err, releases.ErrRollbackUnavailable):
		writeError(w, r, http.StatusConflict, "rollback_unavailable", "no previous promoted component version is available")
	default:
		s.logger.Error("release request failed", "error", err, "correlation_id", correlationIDFromRequest(r))
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("request body must be valid JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

type createComponentVersionRequest struct {
	ComponentName string          `json:"component_name"`
	Version       string          `json:"version"`
	PromptVersion string          `json:"prompt_version"`
	Spec          json.RawMessage `json:"spec"`
	ChangeSummary string          `json:"change_summary"`
	CreatedBy     string          `json:"created_by"`
}

type startCanaryRequest struct {
	Percent int `json:"percent"`
}

type rollbackRequest struct {
	ActorID string `json:"actor_id"`
	Reason  string `json:"reason"`
}

type componentVersionResponse struct {
	ComponentVersion componentVersionDTO `json:"component_version"`
}

type componentVersionListResponse struct {
	ComponentVersions []componentVersionDTO `json:"component_versions"`
}

type evaluationRunResponse struct {
	EvaluationRun evaluationRunDTO `json:"evaluation_run"`
}

type promotionResponse struct {
	Promotion promotionDTO `json:"promotion"`
}

type affectedWorkflowsResponse struct {
	AffectedWorkflows []workflows.ComponentDependency `json:"affected_workflows"`
}

type componentVersionDTO struct {
	ID                        string                   `json:"id"`
	ComponentName             string                   `json:"component_name"`
	Version                   string                   `json:"version"`
	PromptVersion             string                   `json:"prompt_version"`
	Spec                      json.RawMessage          `json:"spec"`
	Status                    releases.ComponentStatus `json:"status"`
	ChangeSummary             string                   `json:"change_summary"`
	CreatedBy                 string                   `json:"created_by"`
	CanaryPercent             int                      `json:"canary_percent"`
	EvaluationPassed          bool                     `json:"evaluation_passed"`
	PreviousPromotedVersionID string                   `json:"previous_promoted_version_id,omitempty"`
	CreatedAt                 time.Time                `json:"created_at"`
	UpdatedAt                 time.Time                `json:"updated_at"`
}

type evaluationRunDTO struct {
	ID                   string          `json:"id"`
	ComponentVersionID   string          `json:"component_version_id"`
	ComponentName        string          `json:"component_name"`
	BaselineVersionID    string          `json:"baseline_version_id"`
	DatasetName          string          `json:"dataset_name"`
	Status               string          `json:"status"`
	CandidatePassed      bool            `json:"candidate_passed"`
	BaselinePassed       bool            `json:"baseline_passed"`
	CandidatePassedCases int             `json:"candidate_passed_cases"`
	CandidateTotalCases  int             `json:"candidate_total_cases"`
	BaselinePassedCases  int             `json:"baseline_passed_cases"`
	BaselineTotalCases   int             `json:"baseline_total_cases"`
	Summary              json.RawMessage `json:"summary"`
	CreatedAt            time.Time       `json:"created_at"`
}

type promotionDTO struct {
	Promoted componentVersionDTO `json:"promoted"`
	Previous componentVersionDTO `json:"previous"`
}

func toComponentVersionDTO(version releases.ComponentVersion) componentVersionDTO {
	return componentVersionDTO{
		ID:                        version.ID,
		ComponentName:             version.ComponentName,
		Version:                   version.Version,
		PromptVersion:             version.PromptVersion,
		Spec:                      version.Spec,
		Status:                    version.Status,
		ChangeSummary:             version.ChangeSummary,
		CreatedBy:                 version.CreatedBy,
		CanaryPercent:             version.CanaryPercent,
		EvaluationPassed:          version.EvaluationPassed,
		PreviousPromotedVersionID: version.PreviousPromotedVersionID,
		CreatedAt:                 version.CreatedAt,
		UpdatedAt:                 version.UpdatedAt,
	}
}

func toComponentVersionDTOs(versions []releases.ComponentVersion) []componentVersionDTO {
	dtos := make([]componentVersionDTO, 0, len(versions))
	for _, version := range versions {
		dtos = append(dtos, toComponentVersionDTO(version))
	}
	return dtos
}

func toEvaluationRunDTO(run releases.EvaluationRun) evaluationRunDTO {
	return evaluationRunDTO{
		ID:                   run.ID,
		ComponentVersionID:   run.ComponentVersionID,
		ComponentName:        run.ComponentName,
		BaselineVersionID:    run.BaselineVersionID,
		DatasetName:          run.DatasetName,
		Status:               run.Status,
		CandidatePassed:      run.CandidatePassed,
		BaselinePassed:       run.BaselinePassed,
		CandidatePassedCases: run.CandidatePassedCases,
		CandidateTotalCases:  run.CandidateTotalCases,
		BaselinePassedCases:  run.BaselinePassedCases,
		BaselineTotalCases:   run.BaselineTotalCases,
		Summary:              run.Summary,
		CreatedAt:            run.CreatedAt,
	}
}

func toPromotionDTO(result releases.PromotionResult) promotionDTO {
	return promotionDTO{
		Promoted: toComponentVersionDTO(result.Promoted),
		Previous: toComponentVersionDTO(result.Previous),
	}
}
