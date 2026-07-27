package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestExecuteLocalWorkflowNodes(t *testing.T) {
	classifier := &fakeAIClassifier{}
	dependencies := NodeDependencies{AIClassifier: classifier}

	first, attempts, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowRunID: "run-1",
		WorkflowName:  LocalDemoWorkflowName,
		NodeName:      "validate_input",
		Input:         json.RawMessage(`{"case_id":"synthetic-case-001","customer_message":"urgent invoice dispute"}`),
	}, dependencies)
	if err != nil {
		t.Fatalf("execute validate_input: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected max attempts 2, got %d", attempts)
	}
	if first.NextNodeName != "classify_issue" {
		t.Fatalf("expected next classify_issue, got %q", first.NextNodeName)
	}

	second, _, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowRunID: "run-1",
		WorkflowName:  LocalDemoWorkflowName,
		NodeName:      first.NextNodeName,
		Input:         json.RawMessage(`{"case_id":"synthetic-case-001","customer_message":"urgent invoice dispute"}`),
	}, dependencies)
	if err != nil {
		t.Fatalf("execute classify_issue: %v", err)
	}
	if second.NextNodeName != "compose_summary" {
		t.Fatalf("expected next compose_summary, got %q", second.NextNodeName)
	}
	if !classifier.called {
		t.Fatal("expected AI classifier to be called")
	}

	third, _, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowRunID: "run-1",
		WorkflowName:  LocalDemoWorkflowName,
		NodeName:      second.NextNodeName,
		Input:         json.RawMessage(`{"case_id":"synthetic-case-001","customer_message":"urgent invoice dispute"}`),
	}, dependencies)
	if err != nil {
		t.Fatalf("execute compose_summary: %v", err)
	}
	if third.NextNodeName != "" {
		t.Fatalf("expected terminal node, got next %q", third.NextNodeName)
	}
}

func TestExecuteLocalWorkflowRejectsMissingCaseID(t *testing.T) {
	_, _, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowName: LocalDemoWorkflowName,
		NodeName:     "validate_input",
		Input:        json.RawMessage(`{}`),
	}, NodeDependencies{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestExecuteNodeRejectsUnknownWorkflow(t *testing.T) {
	_, _, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowName: "missing",
		NodeName:     "validate_input",
		Input:        json.RawMessage(`{"case_id":"synthetic-case-001"}`),
	}, NodeDependencies{})
	if !errors.Is(err, ErrUnknownWorkflow) {
		t.Fatalf("expected ErrUnknownWorkflow, got %v", err)
	}
}

type fakeAIClassifier struct {
	called bool
}

func (c *fakeAIClassifier) ClassifyIssue(context.Context, IssueClassificationRequest) (json.RawMessage, error) {
	c.called = true
	return json.RawMessage(`{"case_id":"synthetic-case-001","category":"billing","urgency":"high","confidence":0.9}`), nil
}
