package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/buildplane/buildplane/internal/config"
	"github.com/buildplane/buildplane/internal/database"
	"github.com/buildplane/buildplane/internal/domain"
	"github.com/buildplane/buildplane/internal/kubernetes"
	"github.com/buildplane/buildplane/internal/leases"
	"github.com/buildplane/buildplane/internal/queue"
	"github.com/buildplane/buildplane/internal/telemetry"
	"github.com/google/uuid"
)

type Service struct {
	cfg    config.Config
	store  *database.Store
	queue  *queue.Queue
	kube   *kubernetes.Client
	fair   *FairScheduler
	canary *FairScheduler
	logger *slog.Logger
}

func New(cfg config.Config, store *database.Store, q *queue.Queue, kube *kubernetes.Client, logger *slog.Logger) *Service {
	return &Service{
		cfg: cfg, store: store, queue: q, kube: kube,
		fair: NewFairScheduler(), canary: NewFairScheduler(), logger: logger,
	}
}

func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.SchedulerInterval)
	defer ticker.Stop()
	reconcileTicker := time.NewTicker(s.cfg.ReconcileInterval)
	defer reconcileTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.scheduleOnce(ctx)
		case <-reconcileTicker.C:
			s.reconcile(ctx)
		}
	}
}

func (s *Service) scheduleOnce(ctx context.Context) {
	start := time.Now()
	defer func() {
		telemetry.SchedulerDecisionDuration.Observe(time.Since(start).Seconds())
	}()
	priorities := []domain.Priority{domain.PriorityCritical, domain.PriorityHigh, domain.PriorityNormal, domain.PriorityLow}
	candidates, err := s.queue.Candidates(ctx, priorities, 10)
	if err != nil {
		s.logger.Warn("read redis candidates failed", "error", err)
		return
	}
	ids := make([]uuid.UUID, 0, len(candidates))
	versionByID := map[uuid.UUID]int{}
	ordinalByID := map[uuid.UUID]int{}
	for i, c := range candidates {
		ids = append(ids, c.JobID)
		versionByID[c.JobID] = c.Version
		ordinalByID[c.JobID] = i
	}
	jobs, err := s.store.GetJobs(ctx, ids)
	if err != nil {
		s.logger.Warn("load candidate jobs failed", "error", err)
		return
	}
	active, activeByTenant, cpu, _, err := s.store.ActiveCounts(ctx)
	if err != nil {
		s.logger.Warn("active count failed", "error", err)
		return
	}
	tenants, err := s.store.Tenants(ctx)
	if err != nil {
		s.logger.Warn("load tenants failed", "error", err)
		return
	}
	choices := make([]Candidate, 0, len(jobs))
	jobByChoiceID := map[string]domain.Job{}
	for _, job := range jobs {
		if job.Status != domain.StatusQueued || versionByID[job.ID] != job.Version {
			_ = s.queue.Remove(ctx, job.Priority, job.TenantID, job.ID)
			continue
		}
		tenant := tenants[job.TenantID]
		c := Candidate{
			ID: job.ID.String(), TenantID: job.TenantID, TenantWeight: tenant.SchedulingWeight, Priority: job.Priority,
			QueuedOrdinal: ordinalByID[job.ID], CPUMillis: job.RequestedCPUMillis,
			TenantActive: activeByTenant[job.TenantID], TenantLimit: tenant.MaxConcurrentJobs,
		}
		choices = append(choices, c)
		jobByChoiceID[c.ID] = job
	}
	limits := Limits{GlobalActive: active, GlobalActiveLimit: s.cfg.GlobalActiveLimit, CPUMillisInFlight: cpu, MaxCPUMillisInFlight: s.cfg.MaxCPUMillisInFlight}
	decision := s.fair.Choose(choices, limits)
	if s.cfg.ShadowScheduler {
		canary := s.canary.Choose(choices, limits)
		if canary.Admitted != decision.Admitted || canary.Candidate.ID != decision.Candidate.ID {
			s.logger.Info("shadow scheduler disagreement", "stable", decision.Candidate.ID, "canary", canary.Candidate.ID, "reason", canary.Reason)
		}
	}
	if !decision.Admitted {
		return
	}
	job := jobByChoiceID[decision.Candidate.ID]
	claimed, ok, err := s.store.ClaimJob(ctx, job.ID, job.Version)
	if err != nil || !ok {
		s.logger.Warn("claim job failed", "job_id", job.ID, "claimed", ok, "error", err)
		_ = s.queue.Remove(ctx, job.Priority, job.TenantID, job.ID)
		return
	}
	_ = s.queue.Remove(ctx, claimed.Priority, claimed.TenantID, claimed.ID)
	token, hash, err := leases.NewToken()
	if err != nil {
		s.logger.Error("lease token generation failed", "error", err)
		return
	}
	attemptID := uuid.New()
	attempt := domain.Attempt{ID: attemptID, JobID: claimed.ID, KubernetesJobName: kubernetes.KubernetesJobName(claimed.ID.String(), attemptID.String())}
	attempt, err = s.store.CreateAttempt(ctx, claimed, attempt.KubernetesJobName, hash, time.Now().Add(s.cfg.LeaseDuration))
	if err != nil {
		s.logger.Error("create attempt failed", "job_id", claimed.ID, "error", err)
		return
	}
	if err := s.kube.CreateRunnerJob(ctx, claimed, attempt, token); err != nil {
		telemetry.KubernetesAPIErrors.WithLabelValues("create_job").Inc()
		s.logger.Error("create kubernetes job failed", "job_id", claimed.ID, "attempt_id", attempt.ID, "error", err)
		return
	}
	s.logger.Info("scheduled job", "tenant_id", claimed.TenantID, "workflow_run_id", claimed.WorkflowRunID, "job_id", claimed.ID, "attempt_id", attempt.ID, "kubernetes_job_name", attempt.KubernetesJobName)
}

func (s *Service) reconcile(ctx context.Context) {
	for _, priority := range []domain.Priority{domain.PriorityCritical, domain.PriorityHigh, domain.PriorityNormal, domain.PriorityLow} {
		if depth, err := s.queue.Depth(ctx, priority); err == nil {
			telemetry.QueueDepth.WithLabelValues(string(priority)).Set(float64(depth))
		}
	}
	queued, err := s.store.ListQueuedJobs(ctx, 500)
	if err != nil {
		s.logger.Warn("list queued for reconciliation failed", "error", err)
		return
	}
	for _, job := range queued {
		if err := s.queue.Enqueue(ctx, job); err != nil {
			s.logger.Warn("re-enqueue failed", "job_id", job.ID, "error", err)
		} else {
			telemetry.ReconciliationActions.WithLabelValues("reenqueue").Inc()
		}
	}
	expired, err := s.store.ExpireLeases(ctx)
	if err != nil {
		s.logger.Warn("expire leases failed", "error", err)
		return
	}
	for _, job := range expired {
		telemetry.LeaseExpirations.Inc()
		telemetry.ReconciliationActions.WithLabelValues("lease_expired").Inc()
		if err := s.queue.Enqueue(ctx, job); err != nil {
			s.logger.Warn("enqueue expired job failed", "job_id", job.ID, "error", err)
		}
	}
}
