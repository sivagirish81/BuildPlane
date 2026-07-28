# Local kind Deployment

This guide runs BuildPlane locally on Kubernetes with `kind` and Helm.

Use this path when you want to learn the Kubernetes deployment model. Helm
installs the full local stack into the cluster:

- control plane API
- scheduler
- general and AI workers
- AI service
- web console
- Redis
- PostgreSQL
- operator, CRD, Prometheus, and Grafana

For local kind, PostgreSQL runs inside Kubernetes as a single-replica
StatefulSet. You do not need Docker Compose Postgres, `host.docker.internal`,
or a manually created `buildplane-postgres` Secret.

## 1. Prerequisites

Install:

- Docker Desktop
- `kubectl`
- `kind`
- Helm

Verify the tools:

```bash
docker version
kubectl version --client
kind version
helm version
```

## 2. Build Local Images

Run from the repository root:

```bash
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
docker build -f deploy/docker/ai-service.Dockerfile -t buildplane/ai-service:dev .
docker build -f deploy/docker/operator.Dockerfile -t buildplane/operator:dev .
docker build -f deploy/docker/web.Dockerfile -t buildplane/web:dev .
```

These image tags match `deploy/helm/buildplane/values-kind.yaml`.

## 3. Create Or Select The kind Cluster

Create the cluster once:

```bash
kind create cluster --config deploy/kind/cluster.yaml
```

Select the context:

```bash
kubectl config use-context kind-buildplane
```

Confirm it:

```bash
kubectl cluster-info
kubectl get nodes
```

## 4. Load Images Into kind

`kind` runs Kubernetes nodes as Docker containers. Images built on your Mac are
not automatically visible inside the kind cluster, so load them:

```bash
kind load docker-image buildplane/control-plane:dev --name buildplane
kind load docker-image buildplane/ai-service:dev --name buildplane
kind load docker-image buildplane/operator:dev --name buildplane
kind load docker-image buildplane/web:dev --name buildplane
```

## 5. Install With Helm

Render first so template errors fail before resources are created:

```bash
helm lint deploy/helm/buildplane
helm template buildplane deploy/helm/buildplane \
  --namespace buildplane-system \
  --include-crds \
  --values deploy/helm/buildplane/values-kind.yaml
```

Install or upgrade:

```bash
helm upgrade --install buildplane deploy/helm/buildplane \
  --namespace buildplane-system \
  --create-namespace \
  --values deploy/helm/buildplane/values-kind.yaml \
  --wait \
  --timeout 10m
```

The local values file sets:

```yaml
database:
  createSecret: true

postgres:
  enabled: true
```

That makes Helm create:

- `Secret/buildplane-postgres`
- `Service/buildplane-postgres`
- `StatefulSet/buildplane-postgres`

The app connects through the in-cluster DNS name:

```text
postgres://buildplane:buildplane_dev_password@buildplane-postgres:5432/buildplane?sslmode=disable
```

## 6. Verify The Rollout

Check workload status:

```bash
kubectl get pods -n buildplane-system
kubectl get deploy -n buildplane-system
kubectl get statefulset -n buildplane-system
kubectl get svc -n buildplane-system
```

Check the custom resource:

```bash
kubectl get buildplaneruntime -n buildplane-system
```

Check the database Secret exists:

```bash
kubectl get secret buildplane-postgres -n buildplane-system
```

Confirm the database URL is injected into a worker pod spec:

```bash
kubectl get deployment buildplane-worker-general \
  -n buildplane-system \
  -o jsonpath='{.spec.template.spec.containers[0].env[0]}'
```

## 7. Open The API

Port-forward the control plane:

```bash
kubectl port-forward -n buildplane-system service/buildplane-control-plane 8080:80
```

In another terminal:

```bash
curl http://localhost:8080/readyz
curl http://localhost:8080/version
```

## 8. Open The Web Console

Port-forward the web service:

```bash
kubectl port-forward -n buildplane-system service/buildplane-web 8088:80
```

Open:

```text
http://localhost:8088
```

## 9. Open Observability

Prometheus:

```bash
kubectl port-forward -n buildplane-system service/buildplane-prometheus 9090:9090
```

Grafana:

```bash
kubectl port-forward -n buildplane-system service/buildplane-grafana 3000:3000
```

Open:

```text
http://localhost:9090
http://localhost:3000
```

## 10. Reset A Broken Local Install

Use this when you previously mixed raw `kubectl apply` resources with Helm, or
when Helm reports that a resource already exists without Helm ownership labels:

```bash
kubectl delete namespace buildplane-system
kubectl wait --for=delete namespace/buildplane-system --timeout=120s
```

Then reinstall:

```bash
helm upgrade --install buildplane deploy/helm/buildplane \
  --namespace buildplane-system \
  --create-namespace \
  --values deploy/helm/buildplane/values-kind.yaml \
  --wait \
  --timeout 10m
```

If you want a completely fresh cluster:

```bash
kind delete cluster --name buildplane
kind create cluster --config deploy/kind/cluster.yaml
kubectl config use-context kind-buildplane
```

Then reload images and reinstall with Helm.

## 11. Stop Or Remove The Local Stack

Remove only the Helm release:

```bash
helm uninstall buildplane -n buildplane-system
```

Remove the namespace and local PostgreSQL volume claim:

```bash
kubectl delete namespace buildplane-system
```

Remove the whole kind cluster:

```bash
kind delete cluster --name buildplane
```

## 12. Troubleshooting

If pods are stuck in `ImagePullBackOff`, the image was not loaded into kind:

```bash
kubectl describe pod -n buildplane-system <pod-name>
kind load docker-image buildplane/control-plane:dev --name buildplane
```

If the control plane, scheduler, or workers log
`BUILDPLANE_DATABASE_URL is required`, inspect the rendered deployment:

```bash
kubectl get deployment buildplane-control-plane \
  -n buildplane-system \
  -o yaml
```

You should see:

```yaml
- name: BUILDPLANE_DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: buildplane-postgres
      key: database-url
```

If pods cannot connect to Postgres, check the Service, StatefulSet, and logs:

```bash
kubectl get svc buildplane-postgres -n buildplane-system
kubectl get statefulset buildplane-postgres -n buildplane-system
kubectl logs -n buildplane-system statefulset/buildplane-postgres
```

If `helm upgrade --install --wait` times out, inspect events:

```bash
kubectl get events -n buildplane-system --sort-by=.lastTimestamp
kubectl get pods -n buildplane-system
```

## 13. Local Versus Production Database

Local kind:

```text
BuildPlane pods -> buildplane-postgres Service -> Postgres StatefulSet
```

Production-like Helm:

```text
BuildPlane pods -> externally managed buildplane-postgres Secret -> managed PostgreSQL
```

Use `deploy/helm/buildplane/values-kind.yaml` for local learning. Use
`deploy/helm/buildplane/values-gke.example.yaml` as the starting point for
production-like environments where PostgreSQL is operated outside the chart.
