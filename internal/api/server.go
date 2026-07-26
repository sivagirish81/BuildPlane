package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/buildplane/buildplane/internal/auth"
	"github.com/buildplane/buildplane/internal/config"
	"github.com/buildplane/buildplane/internal/database"
	"github.com/buildplane/buildplane/internal/domain"
	"github.com/buildplane/buildplane/internal/kubernetes"
	"github.com/buildplane/buildplane/internal/leases"
	"github.com/buildplane/buildplane/internal/queue"
	"github.com/buildplane/buildplane/internal/telemetry"
	"github.com/google/uuid"
)

type Server struct {
	cfg    config.Config
	store  *database.Store
	queue  *queue.Queue
	kube   *kubernetes.Client
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(cfg config.Config, store *database.Store, q *queue.Queue, kube *kubernetes.Client, logger *slog.Logger) *Server {
	s := &Server{cfg: cfg, store: store, queue: q, kube: kube, logger: logger, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.instrument(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.Handle("GET /metrics", telemetry.MetricsHandler())
	s.mux.HandleFunc("POST /v1/workflows", s.createWorkflow)
	s.mux.HandleFunc("POST /v1/runs", s.createRun)
	s.mux.HandleFunc("GET /v1/runs/{id}", s.getRun)
	s.mux.HandleFunc("POST /v1/runs/{id}/cancel", s.cancelRun)
	s.mux.HandleFunc("GET /v1/jobs/{id}/logs", s.jobLogs)
	s.mux.HandleFunc("POST /internal/attempts/start", s.internalStart)
	s.mux.HandleFunc("POST /internal/attempts/heartbeat", s.internalHeartbeat)
	s.mux.HandleFunc("POST /internal/attempts/logs", s.internalLogs)
	s.mux.HandleFunc("POST /internal/attempts/complete", s.internalComplete)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var input database.WorkflowInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	workflow, err := s.store.CreateWorkflow(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, workflow)
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	type request struct {
		WorkflowID    uuid.UUID       `json:"workflow_id"`
		RepositoryURL string          `json:"repository_url"`
		CommitSHA     string          `json:"commit_sha"`
		Priority      domain.Priority `json:"priority"`
	}
	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, http.StatusBadRequest, errors.New("Idempotency-Key header is required"))
		return
	}
	globalQueued, err := s.store.CountQueued(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if globalQueued >= s.cfg.GlobalQueueLimit {
		telemetry.SubmissionRejections.WithLabelValues("global_queue_limit").Inc()
		writeError(w, http.StatusTooManyRequests, errors.New("global queued job limit reached"))
		return
	}
	run, jobs, idempotent, err := s.store.CreateRun(r.Context(), database.CreateRunInput{
		WorkflowID: req.WorkflowID, RepositoryURL: req.RepositoryURL, CommitSHA: req.CommitSHA, Priority: req.Priority, IdempotencyKey: key,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, job := range jobs {
		if err := s.queue.Enqueue(r.Context(), job); err != nil {
			s.logger.Warn("enqueue failed; reconciler will repair", "job_id", job.ID, "error", err)
		}
	}
	code := http.StatusCreated
	if idempotent {
		code = http.StatusOK
	}
	writeJSON(w, code, map[string]any{"run": run, "jobs": jobs, "idempotent": idempotent})
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	run, jobs, attempts, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	for i := range attempts {
		attempts[i].LeaseTokenHash = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "jobs": jobs, "attempts": attempts})
}

func (s *Server) jobLogs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	logs, err := s.store.JobLogs(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(logs))
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	kubeJobs, err := s.store.CancelRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, name := range kubeJobs {
		if s.kube != nil {
			if err := s.kube.DeleteJob(context.Background(), name); err != nil {
				s.logger.Warn("delete kubernetes job failed", "kubernetes_job_name", name, "error", err)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": true, "kubernetes_jobs": kubeJobs})
}

type runnerUpdate struct {
	JobID         uuid.UUID        `json:"job_id"`
	AttemptID     uuid.UUID        `json:"attempt_id"`
	LeaseToken    string           `json:"lease_token"`
	Chunk         string           `json:"chunk"`
	Status        domain.JobStatus `json:"status"`
	ExitCode      int              `json:"exit_code"`
	FailureReason string           `json:"failure_reason"`
}

func (s *Server) internalStart(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckBearer(r, s.cfg.InternalToken) {
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}
	update, ok := s.decodeAndValidateRunner(w, r)
	if !ok {
		return
	}
	if err := s.store.MarkAttemptRunning(r.Context(), update.AttemptID); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) internalHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckBearer(r, s.cfg.InternalToken) {
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}
	update, ok := s.decodeAndValidateRunner(w, r)
	if !ok {
		return
	}
	if err := s.store.RenewLease(r.Context(), update.AttemptID, time.Now().Add(s.cfg.LeaseDuration)); err != nil {
		telemetry.StaleUpdatesRejected.Inc()
		writeError(w, http.StatusConflict, err)
		return
	}
	telemetry.WorkerHeartbeats.Inc()
	writeJSON(w, http.StatusOK, map[string]string{"status": "renewed"})
}

func (s *Server) internalLogs(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckBearer(r, s.cfg.InternalToken) {
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}
	update, ok := s.decodeAndValidateRunner(w, r)
	if !ok {
		return
	}
	if err := s.store.AppendLogs(r.Context(), update.AttemptID, update.Chunk); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged"})
}

func (s *Server) internalComplete(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckBearer(r, s.cfg.InternalToken) {
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}
	update, ok := s.decodeAndValidateRunner(w, r)
	if !ok {
		return
	}
	if err := s.store.CompleteAttempt(r.Context(), update.AttemptID, update.Status, update.ExitCode, update.FailureReason); err != nil {
		telemetry.StaleUpdatesRejected.Inc()
		writeError(w, http.StatusConflict, err)
		return
	}
	telemetry.JobsTotal.WithLabelValues(string(update.Status)).Inc()
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (s *Server) decodeAndValidateRunner(w http.ResponseWriter, r *http.Request) (runnerUpdate, bool) {
	var update runnerUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return update, false
	}
	if _, err := leases.ValidateAttempt(r.Context(), s.store, update.JobID, update.AttemptID, update.LeaseToken); err != nil {
		telemetry.StaleUpdatesRejected.Inc()
		writeError(w, http.StatusConflict, err)
		return update, false
	}
	return update, true
}

func (s *Server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(ww, r)
		route := r.URL.Path
		if strings.Contains(route, "/v1/runs/") {
			route = "/v1/runs/{id}"
		}
		if strings.Contains(route, "/v1/jobs/") {
			route = "/v1/jobs/{id}/logs"
		}
		telemetry.APIRequests.WithLabelValues(route, r.Method, fmt.Sprint(ww.code)).Inc()
		s.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", ww.code, "duration_ms", time.Since(start).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
