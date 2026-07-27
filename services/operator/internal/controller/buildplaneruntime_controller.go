package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	buildplanev1alpha1 "github.com/sivagirish/buildplane/services/operator/api/v1alpha1"
)

const (
	DefaultSchedulerReplicas     int32 = 1
	DefaultGeneralWorkerReplicas int32 = 2
	DefaultAIWorkerReplicas      int32 = 1
)

type BuildPlaneRuntimeReconciler struct {
	client.Client
}

type DeploymentPlan struct {
	Name     string
	Replicas int32
}

func DeploymentPlans(runtime buildplanev1alpha1.BuildPlaneRuntime) []DeploymentPlan {
	return []DeploymentPlan{
		{
			Name:     "buildplane-scheduler",
			Replicas: replicaValue(runtime.Spec.SchedulerReplicas, DefaultSchedulerReplicas),
		},
		{
			Name:     "buildplane-worker-general",
			Replicas: replicaValue(runtime.Spec.WorkerPools.General.Replicas, DefaultGeneralWorkerReplicas),
		},
		{
			Name:     "buildplane-worker-ai",
			Replicas: replicaValue(runtime.Spec.WorkerPools.AI.Replicas, DefaultAIWorkerReplicas),
		},
	}
}

func (r *BuildPlaneRuntimeReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var runtime buildplanev1alpha1.BuildPlaneRuntime
	if err := r.Get(ctx, request.NamespacedName, &runtime); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	plans := DeploymentPlans(runtime)
	statuses := make([]buildplanev1alpha1.DeploymentReplicaStatus, 0, len(plans))
	var reconcileErr error

	for _, plan := range plans {
		status, err := r.reconcileDeployment(ctx, runtime.Namespace, plan)
		statuses = append(statuses, status)
		if err != nil && reconcileErr == nil {
			reconcileErr = err
		}
	}

	runtime.Status.ObservedGeneration = runtime.Generation
	runtime.Status.Deployments = statuses
	setReadyCondition(&runtime, reconcileErr)
	if err := r.Status().Update(ctx, &runtime); err != nil {
		return ctrl.Result{}, err
	}

	if reconcileErr != nil {
		logger.Error(reconcileErr, "runtime reconcile incomplete", "runtime", request.NamespacedName.String())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *BuildPlaneRuntimeReconciler) reconcileDeployment(ctx context.Context, namespace string, plan DeploymentPlan) (buildplanev1alpha1.DeploymentReplicaStatus, error) {
	var deployment appsv1.Deployment
	key := types.NamespacedName{Namespace: namespace, Name: plan.Name}
	if err := r.Get(ctx, key, &deployment); err != nil {
		status := buildplanev1alpha1.DeploymentReplicaStatus{
			Name:    plan.Name,
			Desired: plan.Replicas,
			Ready:   false,
		}
		return status, fmt.Errorf("get deployment %s: %w", key.String(), err)
	}

	currentReplicas := int32(0)
	if deployment.Spec.Replicas != nil {
		currentReplicas = *deployment.Spec.Replicas
	}
	if currentReplicas != plan.Replicas {
		patch := client.MergeFrom(deployment.DeepCopy())
		deployment.Spec.Replicas = &plan.Replicas
		if err := r.Patch(ctx, &deployment, patch); err != nil {
			status := buildplanev1alpha1.DeploymentReplicaStatus{
				Name:      plan.Name,
				Desired:   plan.Replicas,
				Available: deployment.Status.AvailableReplicas,
				Ready:     false,
			}
			return status, fmt.Errorf("patch deployment %s replicas: %w", key.String(), err)
		}
	}

	return buildplanev1alpha1.DeploymentReplicaStatus{
		Name:      plan.Name,
		Desired:   plan.Replicas,
		Available: deployment.Status.AvailableReplicas,
		Ready:     deployment.Status.AvailableReplicas >= plan.Replicas,
	}, nil
}

func setReadyCondition(runtime *buildplanev1alpha1.BuildPlaneRuntime, reconcileErr error) {
	condition := metav1.Condition{
		Type:               "Ready",
		ObservedGeneration: runtime.Generation,
		LastTransitionTime: metav1.Now(),
	}
	if reconcileErr != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ReconcileIncomplete"
		condition.Message = reconcileErr.Error()
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "DeploymentsReconciled"
		condition.Message = "scheduler and worker pool Deployments match the requested runtime spec"
	}
	meta.SetStatusCondition(&runtime.Status.Conditions, condition)
}

func replicaValue(value *int32, fallback int32) int32 {
	if value == nil {
		return fallback
	}
	return *value
}

func (r *BuildPlaneRuntimeReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&buildplanev1alpha1.BuildPlaneRuntime{}).
		Complete(r)
}
