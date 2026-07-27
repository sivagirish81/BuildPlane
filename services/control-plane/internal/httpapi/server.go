package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/observability"
	"github.com/sivagirish/buildplane/services/control-plane/internal/releases"
	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

const serviceName = "buildplane-control-plane"

type Options struct {
	Version   string
	Logger    *slog.Logger
	Ready     func(context.Context) error
	Workflows *workflows.Service
	Releases  *releases.Service
	Metrics   *observability.Registry
}

type Server struct {
	version   string
	logger    *slog.Logger
	ready     func(context.Context) error
	workflows *workflows.Service
	releases  *releases.Service
	metrics   *observability.Registry
}

// NewServer builds the HTTP surface for the control-plane process.
func NewServer(options Options) http.Handler {
	server := &Server{
		version:   options.Version,
		logger:    options.Logger,
		ready:     options.Ready,
		workflows: options.Workflows,
		releases:  options.Releases,
		metrics:   options.Metrics,
	}
	if server.version == "" {
		server.version = "dev"
	}
	if server.logger == nil {
		server.logger = slog.Default()
	}
	if server.ready == nil {
		server.ready = func(context.Context) error { return nil }
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", getOnly(server.healthz))
	mux.HandleFunc("/readyz", getOnly(server.readyz))
	mux.HandleFunc("/version", getOnly(server.versionHandler))
	if server.metrics != nil {
		mux.Handle("/metrics", server.metrics.Handler())
	}
	mux.HandleFunc("/v1/workflow-runs", server.workflowRuns)
	mux.HandleFunc("/v1/workflow-runs/", server.workflowRunByID)
	mux.HandleFunc("/v1/component-versions", server.componentVersions)
	mux.HandleFunc("/v1/component-versions/", server.componentVersionByID)
	mux.HandleFunc("/v1/components/", server.componentByName)
	return server.withRequestLogging(mux)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":         "ok",
		"correlation_id": correlationIDFromRequest(r),
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := s.ready(ctx); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "service is not ready")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":         "ready",
		"correlation_id": correlationIDFromRequest(r),
	})
}

func (s *Server) versionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"name":           serviceName,
		"version":        s.version,
		"correlation_id": correlationIDFromRequest(r),
	})
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		next(w, r)
	}
}

func (s *Server) workflowRuns(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/workflow-runs" {
		writeError(w, r, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if s.workflows == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "workflow service is not configured")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		writeError(w, r, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required")
		return
	}

	var request createWorkflowRunRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return
	}

	run, created, err := s.workflows.CreateRun(r.Context(), workflows.CreateRunRequest{
		WorkflowName:   request.WorkflowName,
		Input:          request.Input,
		IdempotencyKey: idempotencyKey,
		CorrelationID:  correlationIDFromRequest(r),
		TraceParent:    traceParentFromRequest(r),
	})
	if err != nil {
		s.writeWorkflowError(w, r, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	if s.metrics != nil {
		result := "replayed"
		if created {
			result = "created"
		}
		s.metrics.Inc("buildplane_workflow_runs_total", map[string]string{
			"workflow_name": run.WorkflowName,
			"result":        result,
		})
	}

	writeJSON(w, status, workflowRunResponse{
		WorkflowRun: toWorkflowRunDTO(run),
		Replayed:    !created,
	})
}

func (s *Server) workflowRunByID(w http.ResponseWriter, r *http.Request) {
	if s.workflows == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "workflow service is not configured")
		return
	}

	suffix := strings.TrimPrefix(r.URL.Path, "/v1/workflow-runs/")
	if suffix == "" {
		writeError(w, r, http.StatusNotFound, "not_found", "workflow run not found")
		return
	}

	if strings.HasSuffix(suffix, "/decisions") {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		id := strings.TrimSuffix(suffix, "/decisions")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, r, http.StatusNotFound, "not_found", "workflow run not found")
			return
		}
		s.workflowRunDecision(w, r, id)
		return
	}

	if strings.HasSuffix(suffix, "/audit") {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		id := strings.TrimSuffix(suffix, "/audit")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, r, http.StatusNotFound, "not_found", "workflow run not found")
			return
		}
		s.workflowRunAudit(w, r, id)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	id := suffix
	if strings.Contains(id, "/") {
		writeError(w, r, http.StatusNotFound, "not_found", "workflow run not found")
		return
	}

	run, err := s.workflows.GetRun(r.Context(), id)
	if err != nil {
		s.writeWorkflowError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, workflowRunResponse{
		WorkflowRun: toWorkflowRunDTO(run),
		Replayed:    false,
	})
}

func (s *Server) workflowRunAudit(w http.ResponseWriter, r *http.Request, id string) {
	records, err := s.workflows.ListAuditRecords(r.Context(), id)
	if err != nil {
		s.writeWorkflowError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, auditRecordsResponse{
		AuditRecords: toAuditRecordDTOs(records),
	})
}

func (s *Server) workflowRunDecision(w http.ResponseWriter, r *http.Request, id string) {
	decisionKey := r.Header.Get("Idempotency-Key")
	if strings.TrimSpace(decisionKey) == "" {
		writeError(w, r, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required")
		return
	}

	var request submitHumanDecisionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return
	}

	result, created, err := s.workflows.SubmitHumanDecision(r.Context(), workflows.SubmitHumanDecisionRequest{
		WorkflowRunID: id,
		DecisionKey:   decisionKey,
		Decision:      request.Decision,
		ActorID:       request.ActorID,
		Reason:        request.Reason,
	})
	if err != nil {
		s.writeWorkflowError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, humanDecisionResponse{
		WorkflowRun:  toWorkflowRunDTO(result.Run),
		Decision:     toHumanDecisionDTO(result.Decision),
		NextNodeName: result.NextNodeName,
		Replayed:     !created,
	})
}

func (s *Server) writeWorkflowError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, workflows.ErrInvalidWorkflowName):
		writeError(w, r, http.StatusBadRequest, "invalid_workflow_name", "workflow_name is required")
	case errors.Is(err, workflows.ErrMissingIdempotencyKey):
		writeError(w, r, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required")
	case errors.Is(err, workflows.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "input must be a JSON object")
	case errors.Is(err, workflows.ErrInvalidHumanDecision):
		writeError(w, r, http.StatusBadRequest, "invalid_human_decision", "decision must be approved or rejected and actor_id is required")
	case errors.Is(err, workflows.ErrMissingID), errors.Is(err, workflows.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "workflow run not found")
	case errors.Is(err, workflows.ErrIdempotencyConflict):
		writeError(w, r, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was used with a different request")
	case errors.Is(err, workflows.ErrDecisionConflict):
		writeError(w, r, http.StatusConflict, "decision_conflict", "Idempotency-Key was used with a different decision")
	case errors.Is(err, workflows.ErrWorkflowNotWaiting):
		writeError(w, r, http.StatusConflict, "workflow_not_waiting", "workflow run is not waiting for a human decision")
	case errors.Is(err, workflows.ErrUnknownWorkflow):
		writeError(w, r, http.StatusBadRequest, "unknown_workflow", "workflow_name is not supported")
	default:
		s.logger.Error("workflow request failed", "error", err, "correlation_id", correlationIDFromRequest(r))
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (s *Server) withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get("X-Correlation-ID")
		if strings.TrimSpace(correlationID) == "" {
			correlationID = newCorrelationID()
		}
		traceparent := observability.TraceParentFromRequest(r)
		if traceparent == "" {
			traceparent = observability.NewTraceParent()
		}

		w.Header().Set("X-Correlation-ID", correlationID)
		w.Header().Set(observability.TraceParentHeader, traceparent)
		r = r.WithContext(context.WithValue(r.Context(), correlationIDKey{}, correlationID))
		r = r.WithContext(context.WithValue(r.Context(), traceParentKey{}, traceparent))

		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		start := time.Now()
		next.ServeHTTP(recorder, r)

		if s.metrics != nil {
			statusClass := statusClass(recorder.status)
			s.metrics.Inc("buildplane_http_requests_total", map[string]string{
				"method": r.Method,
				"path":   routeLabel(r.URL.Path),
				"status": statusClass,
			})
			s.metrics.Inc("buildplane_http_request_duration_seconds_count", map[string]string{
				"method": r.Method,
				"path":   routeLabel(r.URL.Path),
				"status": statusClass,
			})
			s.metrics.Add("buildplane_http_request_duration_seconds_sum", map[string]string{
				"method": r.Method,
				"path":   routeLabel(r.URL.Path),
				"status": statusClass,
			}, time.Since(start).Seconds())
		}

		s.logger.Info(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"correlation_id", correlationID,
			"trace_id", observability.TraceID(traceParentFromRequest(r)),
		)
	})
}

func routeLabel(path string) string {
	if path == "/healthz" || path == "/readyz" || path == "/version" || path == "/metrics" || path == "/v1/workflow-runs" {
		return path
	}
	if strings.HasPrefix(path, "/v1/workflow-runs/") {
		if strings.HasSuffix(path, "/decisions") {
			return "/v1/workflow-runs/{id}/decisions"
		}
		if strings.HasSuffix(path, "/audit") {
			return "/v1/workflow-runs/{id}/audit"
		}
		return "/v1/workflow-runs/{id}"
	}
	if path == "/v1/component-versions" {
		return path
	}
	if strings.HasPrefix(path, "/v1/component-versions/") {
		if strings.HasSuffix(path, "/evaluations") {
			return "/v1/component-versions/{id}/evaluations"
		}
		if strings.HasSuffix(path, "/canary") {
			return "/v1/component-versions/{id}/canary"
		}
		if strings.HasSuffix(path, "/promote") {
			return "/v1/component-versions/{id}/promote"
		}
		return "/v1/component-versions/{id}"
	}
	if strings.HasPrefix(path, "/v1/components/") {
		if strings.HasSuffix(path, "/affected-workflows") {
			return "/v1/components/{name}/affected-workflows"
		}
		if strings.HasSuffix(path, "/rollback") {
			return "/v1/components/{name}/rollback"
		}
		return "/v1/components/{name}"
	}
	return "other"
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "unknown"
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
		CorrelationID: correlationIDFromRequest(r),
	})
}

func correlationIDFromRequest(r *http.Request) string {
	if value, ok := r.Context().Value(correlationIDKey{}).(string); ok {
		return value
	}
	return ""
}

func traceParentFromRequest(r *http.Request) string {
	if value, ok := r.Context().Value(traceParentKey{}).(string); ok {
		return value
	}
	return ""
}

func newCorrelationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		hash := sha256.Sum256([]byte(time.Now().Format(time.RFC3339Nano)))
		return hex.EncodeToString(hash[:16])
	}
	return hex.EncodeToString(b[:])
}

type correlationIDKey struct{}
type traceParentKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type createWorkflowRunRequest struct {
	WorkflowName string          `json:"workflow_name"`
	Input        json.RawMessage `json:"input"`
}

type submitHumanDecisionRequest struct {
	Decision string `json:"decision"`
	ActorID  string `json:"actor_id"`
	Reason   string `json:"reason"`
}

type workflowRunResponse struct {
	WorkflowRun workflowRunDTO `json:"workflow_run"`
	Replayed    bool           `json:"replayed"`
}

type humanDecisionResponse struct {
	WorkflowRun  workflowRunDTO   `json:"workflow_run"`
	Decision     humanDecisionDTO `json:"decision"`
	NextNodeName string           `json:"next_node_name,omitempty"`
	Replayed     bool             `json:"replayed"`
}

type workflowRunDTO struct {
	ID            string           `json:"id"`
	WorkflowName  string           `json:"workflow_name"`
	Status        workflows.Status `json:"status"`
	Input         json.RawMessage  `json:"input"`
	CorrelationID string           `json:"correlation_id"`
	TraceParent   string           `json:"traceparent"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type humanDecisionDTO struct {
	ID            string          `json:"id"`
	WorkflowRunID string          `json:"workflow_run_id"`
	DecisionKey   string          `json:"decision_key"`
	NodeName      string          `json:"node_name"`
	Decision      string          `json:"decision"`
	ActorID       string          `json:"actor_id"`
	Reason        string          `json:"reason"`
	Details       json.RawMessage `json:"details"`
	CreatedAt     time.Time       `json:"created_at"`
}

type auditRecordsResponse struct {
	AuditRecords []auditRecordDTO `json:"audit_records"`
}

type auditRecordDTO struct {
	ID              int64           `json:"id"`
	WorkflowRunID   string          `json:"workflow_run_id"`
	NodeExecutionID string          `json:"node_execution_id,omitempty"`
	EventType       string          `json:"event_type"`
	ActorType       string          `json:"actor_type"`
	ActorID         string          `json:"actor_id"`
	Details         json.RawMessage `json:"details"`
	CreatedAt       time.Time       `json:"created_at"`
}

type errorResponse struct {
	Error         errorBody `json:"error"`
	CorrelationID string    `json:"correlation_id"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func toWorkflowRunDTO(run workflows.Run) workflowRunDTO {
	return workflowRunDTO{
		ID:            run.ID,
		WorkflowName:  run.WorkflowName,
		Status:        run.Status,
		Input:         run.Input,
		CorrelationID: run.CorrelationID,
		TraceParent:   run.TraceParent,
		CreatedAt:     run.CreatedAt,
		UpdatedAt:     run.UpdatedAt,
	}
}

func toHumanDecisionDTO(decision workflows.HumanDecision) humanDecisionDTO {
	return humanDecisionDTO{
		ID:            decision.ID,
		WorkflowRunID: decision.WorkflowRunID,
		DecisionKey:   decision.DecisionKey,
		NodeName:      decision.NodeName,
		Decision:      decision.Decision,
		ActorID:       decision.ActorID,
		Reason:        decision.Reason,
		Details:       decision.Details,
		CreatedAt:     decision.CreatedAt,
	}
}

func toAuditRecordDTOs(records []workflows.AuditRecord) []auditRecordDTO {
	dtos := make([]auditRecordDTO, 0, len(records))
	for _, record := range records {
		dtos = append(dtos, auditRecordDTO{
			ID:              record.ID,
			WorkflowRunID:   record.WorkflowRunID,
			NodeExecutionID: record.NodeExecutionID,
			EventType:       record.EventType,
			ActorType:       record.ActorType,
			ActorID:         record.ActorID,
			Details:         record.Details,
			CreatedAt:       record.CreatedAt,
		})
	}
	return dtos
}
