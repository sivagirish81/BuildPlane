package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type BuildPlaneRuntimeSpec struct {
	SchedulerReplicas *int32                `json:"schedulerReplicas,omitempty"`
	WorkerPools       BuildPlaneWorkerPools `json:"workerPools,omitempty"`
}

type BuildPlaneWorkerPools struct {
	General RuntimeWorkerPoolSpec `json:"general,omitempty"`
	AI      RuntimeWorkerPoolSpec `json:"ai,omitempty"`
}

type RuntimeWorkerPoolSpec struct {
	Replicas *int32 `json:"replicas,omitempty"`
}

type BuildPlaneRuntimeStatus struct {
	ObservedGeneration int64                     `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition        `json:"conditions,omitempty"`
	Deployments        []DeploymentReplicaStatus `json:"deployments,omitempty"`
}

type DeploymentReplicaStatus struct {
	Name      string `json:"name"`
	Desired   int32  `json:"desired"`
	Available int32  `json:"available"`
	Ready     bool   `json:"ready"`
}

type BuildPlaneRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildPlaneRuntimeSpec   `json:"spec,omitempty"`
	Status BuildPlaneRuntimeStatus `json:"status,omitempty"`
}

type BuildPlaneRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BuildPlaneRuntime `json:"items"`
}

func (in *BuildPlaneRuntime) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(BuildPlaneRuntime)
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = deepCopySpec(in.Spec)
	out.Status = deepCopyStatus(in.Status)
	return out
}

func (in *BuildPlaneRuntimeList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(BuildPlaneRuntimeList)
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]BuildPlaneRuntime, len(in.Items))
	for i := range in.Items {
		item := in.Items[i].DeepCopyObject().(*BuildPlaneRuntime)
		out.Items[i] = *item
	}
	return out
}

func deepCopySpec(in BuildPlaneRuntimeSpec) BuildPlaneRuntimeSpec {
	out := in
	if in.SchedulerReplicas != nil {
		value := *in.SchedulerReplicas
		out.SchedulerReplicas = &value
	}
	out.WorkerPools.General = deepCopyWorkerPoolSpec(in.WorkerPools.General)
	out.WorkerPools.AI = deepCopyWorkerPoolSpec(in.WorkerPools.AI)
	return out
}

func deepCopyWorkerPoolSpec(in RuntimeWorkerPoolSpec) RuntimeWorkerPoolSpec {
	out := in
	if in.Replicas != nil {
		value := *in.Replicas
		out.Replicas = &value
	}
	return out
}

func deepCopyStatus(in BuildPlaneRuntimeStatus) BuildPlaneRuntimeStatus {
	out := in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		copy(out.Conditions, in.Conditions)
	}
	if in.Deployments != nil {
		out.Deployments = make([]DeploymentReplicaStatus, len(in.Deployments))
		copy(out.Deployments, in.Deployments)
	}
	return out
}
