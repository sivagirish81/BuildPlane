# Phase 7: Operator and Custom Resources

## What You Built

Phase 7 added BuildPlane's first Kubernetes API extension:

```text
BuildPlaneRuntime.buildplane.io/v1alpha1
```

The custom resource controls a narrow runtime shape:

- scheduler replicas
- general worker replicas
- AI worker replicas

The Go operator reconciles that spec onto existing Deployments and writes
status back to the custom resource.

## CRD Mental Model

A CRD teaches Kubernetes a new noun.

Before the CRD exists, this object is meaningless to the API server:

```yaml
apiVersion: buildplane.io/v1alpha1
kind: BuildPlaneRuntime
```

After applying the CRD, Kubernetes can store and validate
`BuildPlaneRuntime` objects. That still does not make anything happen. The
operator is what watches those objects and changes the cluster.

## Spec Versus Status

`spec` is user intent:

```yaml
spec:
  schedulerReplicas: 1
  workerPools:
    general:
      replicas: 2
    ai:
      replicas: 1
```

`status` is controller observation:

```yaml
status:
  observedGeneration: 1
  conditions:
    - type: Ready
      status: "True"
      reason: DeploymentsReconciled
```

Do not treat status as input. The user changes spec; the controller updates
status.

## Reconciliation Loop

The operator repeatedly does this:

```text
read BuildPlaneRuntime
compute desired Deployment replicas
read actual Deployments
patch replicas when different
write status conditions
repeat later or when the object changes
```

This is why Kubernetes controllers are usually safe to rerun. A reconcile pass
should converge on desired state, not depend on hidden one-time steps.

## What The Operator Does Not Do Yet

The Phase 7 operator does not:

- create every BuildPlane Deployment
- manage Services, Secrets, or ConfigMaps
- own existing resources with owner references
- run workflow nodes
- read or write PostgreSQL
- autoscale
- use admission webhooks

That restraint is intentional. The first operator should make reconciliation
visible before it becomes powerful.

## Inspect It

After Docker and `kind` are available:

```bash
kubectl apply -f deploy/kind/buildplane-runtime-crd.yaml
kubectl apply -f deploy/kind/buildplane-operator.yaml
kubectl apply -f deploy/kind/buildplane-runtime-sample.yaml
```

Inspect the CRD:

```bash
kubectl get crd buildplaneruntimes.buildplane.io
kubectl explain buildplaneruntime.spec
```

Inspect the runtime object:

```bash
kubectl get buildplaneruntime -n buildplane-system
kubectl get buildplaneruntime local-runtime -n buildplane-system -o yaml
```

Try changing desired state:

```bash
kubectl patch buildplaneruntime local-runtime -n buildplane-system \
  --type merge \
  -p '{"spec":{"workerPools":{"ai":{"replicas":2}}}}'

kubectl get deploy buildplane-worker-ai -n buildplane-system
```

The operator should reconcile the AI worker Deployment to two replicas.

