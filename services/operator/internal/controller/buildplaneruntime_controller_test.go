package controller

import (
	"testing"

	buildplanev1alpha1 "github.com/sivagirish/buildplane/services/operator/api/v1alpha1"
)

func TestDeploymentPlansUseDefaults(t *testing.T) {
	plans := DeploymentPlans(buildplanev1alpha1.BuildPlaneRuntime{})

	assertPlan(t, plans, "buildplane-scheduler", 1)
	assertPlan(t, plans, "buildplane-worker-general", 2)
	assertPlan(t, plans, "buildplane-worker-ai", 1)
}

func TestDeploymentPlansUseRuntimeSpec(t *testing.T) {
	scheduler := int32(2)
	general := int32(3)
	ai := int32(4)

	plans := DeploymentPlans(buildplanev1alpha1.BuildPlaneRuntime{
		Spec: buildplanev1alpha1.BuildPlaneRuntimeSpec{
			SchedulerReplicas: &scheduler,
			WorkerPools: buildplanev1alpha1.BuildPlaneWorkerPools{
				General: buildplanev1alpha1.RuntimeWorkerPoolSpec{Replicas: &general},
				AI:      buildplanev1alpha1.RuntimeWorkerPoolSpec{Replicas: &ai},
			},
		},
	})

	assertPlan(t, plans, "buildplane-scheduler", 2)
	assertPlan(t, plans, "buildplane-worker-general", 3)
	assertPlan(t, plans, "buildplane-worker-ai", 4)
}

func assertPlan(t *testing.T, plans []DeploymentPlan, name string, replicas int32) {
	t.Helper()
	for _, plan := range plans {
		if plan.Name == name {
			if plan.Replicas != replicas {
				t.Fatalf("expected %s replicas %d, got %d", name, replicas, plan.Replicas)
			}
			return
		}
	}
	t.Fatalf("missing plan for %s", name)
}
