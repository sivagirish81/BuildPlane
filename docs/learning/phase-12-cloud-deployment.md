# Phase 12 Learning Notes: Cloud Deployment

## What We Built

Phase 12 added a first cloud-deployment package:

- A Helm chart in `deploy/helm/buildplane`.
- A GKE Terraform scaffold in `infra/gke`.
- A frontend web image using Nginx.
- A GKE values example for registry-hosted image tags.
- A deployment guide and cloud runbook.
- CI checks for Helm and Terraform packaging.

## The Split

Terraform owns cloud infrastructure:

```text
GCP APIs -> VPC -> subnet -> GKE cluster -> node pool -> Artifact Registry
```

Helm owns Kubernetes application resources:

```text
Deployments -> Services -> ConfigMaps -> Secrets -> RBAC -> CRD/runtime
```

That split matters because cloud infrastructure and app releases change at
different speeds. You might upgrade BuildPlane weekly, but change the VPC or
cluster shape rarely.

## Helm Lesson

Helm templates are Kubernetes YAML with values.

The important command is:

```bash
helm template buildplane deploy/helm/buildplane --namespace buildplane-system --include-crds
```

This renders YAML without touching a cluster. Rendering first is how you catch
many mistakes before `helm install`.

## Terraform Lesson

Terraform keeps state. State is Terraform's memory of what it created.

For personal experiments, local state is fine. For shared infrastructure, state
should move to a remote backend such as a GCS bucket.

The important flow is:

```bash
terraform init
terraform plan
terraform apply
```

`plan` is the review step. Read it before applying.

## GKE Lesson

GKE gives you a managed Kubernetes control plane. You still own:

- worker node sizing
- image registry access
- Kubernetes manifests
- secrets
- database availability
- rollouts and debugging

After Terraform creates a cluster, `gcloud container clusters get-credentials`
adds the cluster to your kubeconfig. From there, `kubectl` and Helm talk to GKE
the same way they talk to `kind`.

## Image Registry Lesson

Local `kind` can load local images. GKE cannot. GKE nodes pull images from a
registry, so BuildPlane needs stable image tags like:

```text
us-central1-docker.pkg.dev/<project>/buildplane/control-plane:phase-12-demo
```

Using explicit tags is better than `latest` because rollbacks and debugging
need reproducibility.

## What Is Still Not Production

This phase does not add:

- Cloud SQL
- managed Redis
- ingress, DNS, or TLS
- real secret-manager integration
- workload identity IAM bindings
- persistent Prometheus storage
- alert rules
- autoscaling policies

Those are real production concerns, but adding all of them at once would blur
the teaching boundary.
