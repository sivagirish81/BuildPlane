# ADR 0012: Cloud Deployment Packaging

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane has a local Kubernetes runtime, an operator, observability manifests,
and a React operations console. The roadmap now calls for deploying the platform
to GKE with Terraform and Helm.

The existing `kind` manifests are useful for learning individual Kubernetes
objects, but cloud deployment needs environment-specific values:

- registry image paths
- image tags
- replica counts
- service types
- database secret wiring
- cloud cluster and node pool configuration

## Decision

Add two deployment layers with separate responsibilities:

- Terraform in `infra/gke` creates GKE infrastructure and Artifact Registry.
- Helm in `deploy/helm/buildplane` installs BuildPlane application resources.

The Terraform layer creates:

- required GCP APIs
- VPC network and subnet
- secondary ranges for Pods and Services
- GKE cluster
- managed node pool
- Artifact Registry Docker repository

The Helm chart installs:

- control-plane API
- scheduler
- general and AI worker pools
- AI service
- Redis
- frontend web console
- BuildPlane operator
- BuildPlaneRuntime CRD and sample runtime
- first Prometheus and Grafana observability slice

The chart keeps exact workload names such as `buildplane-scheduler` because the
current operator reconciler expects those Deployment names.

The frontend is packaged as a static Nginx container. Nginx serves the compiled
React assets and proxies `/api` to the in-cluster control-plane Service.

## Consequences

- BuildPlane now has a clear path from source code to cloud Kubernetes:
  Terraform creates the cluster, images are pushed to Artifact Registry, Helm
  installs the app.
- Terraform does not manage Helm releases yet. This keeps infrastructure and
  application release boundaries easy to inspect while learning.
- PostgreSQL remains external to the chart and must be supplied through a
  Kubernetes Secret. Managed Cloud SQL is deferred.
- The chart is deployable, but this phase does not claim a successful live GKE
  apply. Live deployment still requires GCP credentials, billing, a project, and
  a reachable PostgreSQL database.
- Ingress, DNS, TLS, managed Redis, Cloud SQL, secret-manager integration,
  workload identity bindings, and production alerting are deferred.

## Alternatives Considered

Use Kustomize instead of Helm:

- Rejected for this phase because the roadmap explicitly calls for Helm and the
  project needs value-driven image/replica configuration.

Use Terraform to install Helm:

- Deferred. It is common in some teams, but keeping Terraform and Helm separate
  makes the infrastructure/application boundary clearer for this learning
  stage.

Create Cloud SQL immediately:

- Deferred. Database networking, migrations, backups, IAM, and secret handling
  deserve their own focused phase.
