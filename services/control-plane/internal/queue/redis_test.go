package queue

import (
	"testing"

	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

func TestStreamNameForPool(t *testing.T) {
	if got := StreamNameForPool(workflows.WorkerPoolGeneral); got != "buildplane:node-executions:general" {
		t.Fatalf("expected general stream, got %q", got)
	}
	if got := StreamNameForPool(workflows.WorkerPoolAI); got != "buildplane:node-executions:ai" {
		t.Fatalf("expected ai stream, got %q", got)
	}
	if got := StreamNameForPool("  "); got != "buildplane:node-executions:general" {
		t.Fatalf("expected blank pool to use general stream, got %q", got)
	}
}
