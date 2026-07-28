# BuildPlane Cloud Operations Runbook

## First Checks

```bash
kubectl get pods -n buildplane-system
kubectl get deploy -n buildplane-system
kubectl get svc -n buildplane-system
kubectl get buildplaneruntimes -n buildplane-system
```

## API Is Not Ready

Check the Deployment:

```bash
kubectl describe deployment buildplane-control-plane -n buildplane-system
kubectl describe pod -n buildplane-system -l app.kubernetes.io/name=buildplane-control-plane
kubectl logs -n buildplane-system deployment/buildplane-control-plane
```

Likely causes:

- missing `buildplane-postgres` Secret
- invalid database URL
- database unreachable from the cluster
- migration failure

## Image Pull Failures

```bash
kubectl describe pod -n buildplane-system <pod-name>
```

Look for `ErrImagePull` or `ImagePullBackOff`.

Likely causes:

- Helm values point at the wrong repository
- the tag was not pushed
- GKE nodes do not have registry read access

## Workers Are Not Progressing

```bash
kubectl logs -n buildplane-system deployment/buildplane-scheduler
kubectl logs -n buildplane-system deployment/buildplane-worker-general
kubectl logs -n buildplane-system deployment/buildplane-worker-ai
```

Then inspect database state:

```sql
select id, workflow_run_id, node_name, status, worker_pool, attempt
from node_executions
order by created_at desc
limit 20;
```

The database remains the source of truth. Redis messages are dispatch hints,
not authoritative workflow state.

## Web Console Loads But API Calls Fail

Check the web service and Nginx proxy:

```bash
kubectl port-forward -n buildplane-system service/buildplane-web 8088:80
curl -i http://localhost:8088/
curl -i http://localhost:8088/api/readyz
```

If `/` works but `/api/readyz` fails, inspect the control-plane Service:

```bash
kubectl get endpoints buildplane-control-plane -n buildplane-system
kubectl describe service buildplane-control-plane -n buildplane-system
```

## Roll Back an App Release

Helm keeps release history:

```bash
helm history buildplane -n buildplane-system
helm rollback buildplane <revision> -n buildplane-system
```

This rolls back Kubernetes manifests. It does not roll back PostgreSQL data.

## Remove the App

```bash
helm uninstall buildplane -n buildplane-system
```

This removes the Helm-managed app resources. It does not delete external
PostgreSQL data or Terraform-created cloud infrastructure.

## Destroy Cloud Infrastructure

Only run this for disposable environments:

```bash
cd infra/gke
terraform destroy
```

If `deletion_protection` is true, update it intentionally before destroying the
cluster.
