# BuildPlane Build Status

Last updated: 2026-07-27

## Current Phase

Phase 12: Cloud Deployment

Status: Complete

## Completed Items

- Added a Helm chart under `deploy/helm/buildplane`.
- Added Helm values for local defaults and a GKE image-tag example.
- Added a frontend web container using Nginx to serve static React assets and
  proxy `/api` to the control-plane service.
- Added Terraform scaffold under `infra/gke` for GCP service APIs, VPC,
  subnet, secondary ranges, GKE cluster, node pool, and Artifact Registry.
- Added a GKE deployment guide at `docs/deployment/gke.md`.
- Added a cloud operations runbook at `docs/runbooks/cloud-operations.md`.
- Added ADR 0012 for the Terraform/Helm cloud deployment boundary.
- Added the Phase 12 learning artifact at
  `docs/learning/phase-12-cloud-deployment.md`.
- Added CI checks for Helm lint/render and Terraform fmt/init/validate.
- Updated README, roadmap, and architecture docs for Phase 12.

## Remaining Limitations

- The workflow definition is still compiled into Go code. A durable
  workflow/component model starts later.
- Worker pool assignment is still compiled into Go code. A durable component
  registry can replace this later.
- The operator only reconciles existing Deployment replica counts. It does not
  create/adopt every BuildPlane resource yet.
- The CRD schema is hand-written. Generated deepcopy/CRD code and webhooks are
  deferred.
- The observability stack is a first slice. It does not yet include a full
  OpenTelemetry SDK, collector, spans, exemplars, alert rules, or persistent
  metrics storage.
- Human approval is intentionally narrow: no role authorization, assignment
  queue, SLA timer, comment thread, or notification model exists yet.
- Guarded external actions are mock-only and do not call invoice, freight,
  ticketing, payment, or carrier systems.
- Component versions are durable, but workflow execution still uses the
  app-defined `classify_issue` implementation directly. Live routing by
  component version is deferred.
- Canary percentage is stored as desired release state. It does not yet split
  live workflow traffic or prove production canary health.
- The frontend is a local operational console. It does not yet include auth,
  tenant isolation, server-side sessions, role-aware action guards, persisted UI
  preferences, ingress, DNS, or TLS.
- SSE streams workflow snapshots only. It does not yet provide a global event
  stream, notification fanout, or durable event replay cursor.
- The synthetic evaluation dataset is intentionally tiny and should be expanded
  before any real release confidence claims.
- The AI service currently has one classifier endpoint. It does not yet expose
  extraction, embeddings, tool-use planning, eval hooks, or provider metrics.
- The OpenAI provider is implemented but not live-tested in this phase.
- Backpressure is coarse: the scheduler uses a batch size, but does not yet
  account for tenant limits, retry storms, downstream rate limits, or oldest
  task age.
- Redis consumer-group pending entry recovery is basic and should be expanded
  later.
- The Helm chart expects registry-hosted images and a database Secret. This
  phase did not provision Cloud SQL or a managed external PostgreSQL database.
- Terraform is a GKE scaffold only. It does not yet manage Helm releases,
  database infrastructure, DNS, TLS, secret-manager integration, managed Redis,
  workload identity IAM bindings, or alerting.
- A live GKE deployment was not applied in this phase because no GCP project,
  billing context, credentials, or external PostgreSQL URL were provided.
- Docker commands require elevated Docker socket access from the Codex sandbox.
- The local `rg` command is currently broken due a Homebrew `pcre2` dynamic
  library signing/load issue, so repo discovery used `find`.

## Known Risks

- The full BuildPlane vision is large. The roadmap intentionally starts with a
  tiny Kubernetes workload to prevent premature architecture.
- Kubernetes concepts can feel abstract until inspected live. Phases 1 through
  12 should be exercised manually in `kind`, local Compose, and eventually a
  real GKE project using the documented commands.
- The migration runner is intentionally small. It should be revisited before
  complex schema evolution, rollbacks, checksums, or multi-instance migration
  locking are needed.
- The current Redis Deployment is for local `kind` learning only and is not a
  production Redis design.
- Worker pools do not autoscale yet.
- Operator finalizers, owner references, admission webhooks, and leader election
  hardening are deferred.

## Next Phase

No later phase is defined in the current roadmap. The next continuation should
extend `docs/BUILD_ROADMAP.md` with the next learning phase before
implementation continues.

## Test Evidence

Commands executed:

```bash
helm lint deploy/helm/buildplane
helm template buildplane deploy/helm/buildplane --namespace buildplane-system --include-crds --values deploy/helm/buildplane/values-gke.example.yaml
helm template buildplane deploy/helm/buildplane --namespace buildplane-system --include-crds --values deploy/helm/buildplane/values-gke.example.yaml --output-dir /private/tmp/buildplane-phase12-rendered
ruby -e 'require "yaml"; Dir["/private/tmp/buildplane-phase12-rendered/**/*.yaml"].sort.each { |path| YAML.load_stream(File.read(path)); puts path }'
npm run typecheck
npm test
npm run build
docker build -f deploy/docker/web.Dockerfile -t buildplane/web:dev .
go test ./services/control-plane/...
go test ./services/operator/...
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
ruby -e 'require "yaml"; %w[.github/workflows/ci.yml deploy/helm/buildplane/values.yaml deploy/helm/buildplane/values-gke.example.yaml].each { |path| YAML.load_stream(File.read(path)); puts path }'
ruby -e 'require "json"; JSON.parse(File.read("deploy/grafana/buildplane-overview.json")); JSON.parse(File.read("examples/component-versions/issue-classifier-v2.json")); JSON.parse(File.read("examples/demo-workflows/invoice-exception.json")); JSON.parse(File.read("examples/demo-workflows/freight-exception.json"))'
command -v terraform
kubectl apply --dry-run=client --validate=false --recursive -f /private/tmp/buildplane-phase12-rendered/buildplane
git diff --check
```

Results:

- Helm lint passed.
- Helm render passed with the GKE example values and CRDs included.
- Rendered Helm YAML parsed offline with Ruby.
- Frontend typecheck, Vitest unit tests, and production build passed.
- Docker web image build passed after elevated Docker socket access was
  granted.
- Go unit tests passed for control-plane and operator modules.
- Python AI service tests passed.
- CI workflow YAML and Helm values YAML parsed.
- Grafana dashboard and example JSON files parsed.
- `git diff --check` passed.
- `terraform` is not installed locally, so `terraform fmt`, `terraform init`,
  and `terraform validate` could not run in this environment. CI now runs those
  checks with `hashicorp/setup-terraform`.
- `kubectl apply --dry-run=client` against the rendered chart could not run
  locally because `kubectl` attempted API discovery against the current
  localhost cluster endpoint and the sandbox blocked that connection.

## Proposed Commit Message

`feat: add cloud deployment packaging`
