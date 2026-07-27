package workflows

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestExecuteLocalWorkflowNodes(t *testing.T) {
	first, attempts, err := ExecuteNode(NodeInput{
		WorkflowName: LocalDemoWorkflowName,
		NodeName:     "validate_input",
		Input:        json.RawMessage(`{"case_id":"synthetic-case-001"}`),
	})
	if err != nil {
		t.Fatalf("execute validate_input: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected max attempts 2, got %d", attempts)
	}
	if first.NextNodeName != "compose_summary" {
		t.Fatalf("expected next compose_summary, got %q", first.NextNodeName)
	}

	second, _, err := ExecuteNode(NodeInput{
		WorkflowName: LocalDemoWorkflowName,
		NodeName:     first.NextNodeName,
		Input:        json.RawMessage(`{"case_id":"synthetic-case-001"}`),
	})
	if err != nil {
		t.Fatalf("execute compose_summary: %v", err)
	}
	if second.NextNodeName != "" {
		t.Fatalf("expected terminal node, got next %q", second.NextNodeName)
	}
}

func TestExecuteLocalWorkflowRejectsMissingCaseID(t *testing.T) {
	_, _, err := ExecuteNode(NodeInput{
		WorkflowName: LocalDemoWorkflowName,
		NodeName:     "validate_input",
		Input:        json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestExecuteNodeRejectsUnknownWorkflow(t *testing.T) {
	_, _, err := ExecuteNode(NodeInput{
		WorkflowName: "missing",
		NodeName:     "validate_input",
		Input:        json.RawMessage(`{"case_id":"synthetic-case-001"}`),
	})
	if !errors.Is(err, ErrUnknownWorkflow) {
		t.Fatalf("expected ErrUnknownWorkflow, got %v", err)
	}
}
