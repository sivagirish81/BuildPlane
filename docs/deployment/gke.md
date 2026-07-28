# BuildPlane GKE Deployment Guide

This guide describes the Phase 12 deployment path. It is intentionally explicit
and conservative.

## Prerequisites

- GCP project with billing enabled.
- `gcloud` authenticated for the target project.
- Docker authenticated to Artifact Registry.
- `terraform`, `helm`, and `kubectl` installed.
- A PostgreSQL database reachable from the GKE cluster.

No real secret values should be committed to this repository.

## 1. Create GKE Infrastructure

```bash
cd infra/gke
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform plan
terraform apply
```

Configure `kubectl`:

```bash
terraform output get_credentials_command
```

Run the printed `gcloud container clusters get-credentials ...` command.

## 2. Build and Push Images

Set your repository URL from Terraform:

```bash
REPOSITORY="$(terraform -chdir=infra/gke output -raw artifact_registry_repository_url)"
TAG="phase-12-demo"
```

Build images:

```bash
docker build -f deploy/docker/control-plane.Dockerfile -t "$REPOSITORY/control-plane:$TAG" .
docker build -f deploy/docker/ai-service.Dockerfile -t "$REPOSITORY/ai-service:$TAG" .
docker build -f deploy/docker/operator.Dockerfile -t "$REPOSITORY/operator:$TAG" .
docker build -f deploy/docker/web.Dockerfile -t "$REPOSITORY/web:$TAG" .
```

Push images:

```bash
docker push "$REPOSITORY/control-plane:$TAG"
docker push "$REPOSITORY/ai-service:$TAG"
docker push "$REPOSITORY/operator:$TAG"
docker push "$REPOSITORY/web:$TAG"
```

## 3. Create the Database Secret

Create the namespace and database URL Secret. Replace the placeholder with your
real PostgreSQL URL in your shell, not in a committed file.

```bash
kubectl create namespace buildplane-system
kubectl create secret generic buildplane-postgres \
  --namespace buildplane-system \
  --from-literal=database-url="$BUILDPLANE_DATABASE_URL"
```

## 4. Render Before Installing

Copy the example values and replace image repositories/tags:

```bash
cp deploy/helm/buildplane/values-gke.example.yaml /tmp/buildplane-values.yaml
helm template buildplane deploy/helm/buildplane \
  --namespace buildplane-system \
  --include-crds \
  --values /tmp/buildplane-values.yaml
```

## 5. Install or Upgrade

```bash
helm upgrade --install buildplane deploy/helm/buildplane \
  --namespace buildplane-system \
  --create-namespace \
  --values /tmp/buildplane-values.yaml
```

## 6. Inspect the Rollout

```bash
kubectl get pods -n buildplane-system
kubectl rollout status deployment/buildplane-control-plane -n buildplane-system
kubectl rollout status deployment/buildplane-web -n buildplane-system
kubectl get buildplaneruntimes -n buildplane-system
```

Port-forward the web console:

```bash
kubectl port-forward -n buildplane-system service/buildplane-web 8088:80
```

Open `http://localhost:8088`.

## EKS Mapping Notes

The same Helm chart can target EKS if image repositories and storage/secrets are
changed for AWS.

Conceptual mapping:

- GKE cluster -> EKS cluster
- Artifact Registry -> ECR
- GCP IAM/Workload Identity -> AWS IAM Roles for Service Accounts
- Cloud SQL candidate -> RDS PostgreSQL
- GCP Load Balancer -> AWS Load Balancer Controller

The Terraform in this phase is GKE-specific. An EKS Terraform module should be
added separately rather than mixing providers in one first scaffold.
