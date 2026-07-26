package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/buildplane/buildplane/internal/config"
	"github.com/buildplane/buildplane/internal/domain"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	set       kubernetes.Interface
	namespace string
	cfg       config.Config
}

func New(cfg config.Config) (*Client, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("load kubernetes config: %w", err)
		}
	}
	set, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return &Client{set: set, namespace: cfg.KubeNamespace, cfg: cfg}, nil
}

func (c *Client) CreateRunnerJob(ctx context.Context, job domain.Job, attempt domain.Attempt, leaseToken string) error {
	spec := c.JobSpec(job, attempt, leaseToken)
	_, err := c.set.BatchV1().Jobs(c.namespace).Create(ctx, spec, metav1.CreateOptions{})
	return err
}

func (c *Client) DeleteJob(ctx context.Context, name string) error {
	propagation := metav1.DeletePropagationBackground
	return c.set.BatchV1().Jobs(c.namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
}

func (c *Client) JobSpec(job domain.Job, attempt domain.Attempt, leaseToken string) *batchv1.Job {
	backoff := int32(0)
	ttl := int32(300)
	deadline := int64(1800)
	name := attempt.KubernetesJobName
	labels := map[string]string{
		"app.kubernetes.io/name": "buildplane-runner",
		"buildplane/run":         short(job.WorkflowRunID.String()),
		"buildplane/job":         short(job.ID.String()),
		"buildplane/attempt":     short(attempt.ID.String()),
		"buildplane/tenant":      sanitizeLabel(job.TenantID),
		"buildplane/scheduler":   "mvp",
	}
	runAsNonRoot := true
	readOnly := false
	allowPrivilegeEscalation := false
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: c.namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			ActiveDeadlineSeconds:   &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: c.cfg.RunnerServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
					},
					Volumes: []corev1.Volume{{
						Name:         "workspace",
						VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
					}},
					Containers: []corev1.Container{{
						Name:  "runner",
						Image: c.cfg.RunnerImage,
						Env: []corev1.EnvVar{
							{Name: "BUILDPLANE_API_URL", Value: c.cfg.PublicBaseURL},
							{Name: "BUILDPLANE_INTERNAL_TOKEN", Value: c.cfg.InternalToken},
							{Name: "BUILDPLANE_JOB_ID", Value: job.ID.String()},
							{Name: "BUILDPLANE_RUN_ID", Value: job.WorkflowRunID.String()},
							{Name: "BUILDPLANE_ATTEMPT_ID", Value: attempt.ID.String()},
							{Name: "BUILDPLANE_LEASE_TOKEN", Value: leaseToken},
							{Name: "BUILDPLANE_REPOSITORY_URL", Value: job.RepositoryURL},
							{Name: "BUILDPLANE_COMMIT_SHA", Value: job.CommitSHA},
							{Name: "BUILDPLANE_COMMANDS", Value: strings.Join(job.Commands, "\n")},
							{Name: "HOME", Value: "/workspace"},
							{Name: "GOCACHE", Value: "/workspace/.cache/go-build"},
							{Name: "GOMODCACHE", Value: "/workspace/.cache/go-mod"},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(job.RequestedCPUMillis), resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(int64(job.RequestedMemoryMB)*1024*1024, resource.BinarySI),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(job.RequestedCPUMillis), resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(int64(job.RequestedMemoryMB)*1024*1024, resource.BinarySI),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							ReadOnlyRootFilesystem:   &readOnly,
							AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "workspace", MountPath: "/workspace"}},
					}},
				},
			},
		},
	}
}

func KubernetesJobName(jobID, attemptID string) string {
	return fmt.Sprintf("bp-%s-%s", short(jobID), short(attemptID))
}

func short(id string) string {
	noDash := strings.ReplaceAll(id, "-", "")
	if len(noDash) <= 10 {
		return noDash
	}
	return noDash[:10]
}

func sanitizeLabel(v string) string {
	v = strings.ToLower(v)
	replacer := strings.NewReplacer("_", "-", ".", "-")
	v = replacer.Replace(v)
	if len(v) > 63 {
		v = v[:63]
	}
	return strings.Trim(v, "-")
}
