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

func TestWorkerPoolForNode(t *testing.T) {
	if got := WorkerPoolForNode("validate_input"); got != WorkerPoolGeneral {
		t.Fatalf("expected validate_input on general pool, got %q", got)
	}
	if got := WorkerPoolForNode("compose_summary"); got != WorkerPoolGeneral {
		t.Fatalf("expected compose_summary on general pool, got %q", got)
	}
	if got := WorkerPoolForNode("classify_issue"); got != WorkerPoolAI {
		t.Fatalf("expected classify_issue on ai pool, got %q", got)
	}
}

func TestDemoWorkflowPausesForHumanApproval(t *testing.T) {
	graph, ok := DependencyGraphFor(InvoiceExceptionWorkflowName)
	if !ok {
		t.Fatal("expected invoice demo dependency graph")
	}
	if got, want := graph[0], "validate_demo_input"; got != want {
		t.Fatalf("expected first node %q, got %q", want, got)
	}
	if next := NextNodeAfterApproval(InvoiceExceptionWorkflowName); next != "record_mock_action" {
		t.Fatalf("expected approval successor record_mock_action, got %q", next)
	}

	input := json.RawMessage(`{
		"case_id": "synthetic-inv-case-001",
		"title": "Invoice price mismatch",
		"description": "Synthetic invoice total is higher than purchase order",
		"customer_message": "Please review this invoice before payment",
		"source": "demo",
		"invoice_id": "synthetic-inv-001",
		"vendor_name": "Synthetic Vendor",
		"amount_disputed": 1250.50
	}`)

	output, _, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowRunID: "run-1",
		WorkflowName:  InvoiceExceptionWorkflowName,
		NodeName:      "await_human_approval",
		Input:         input,
	}, NodeDependencies{})
	if err != nil {
		t.Fatalf("execute await_human_approval: %v", err)
	}
	if !output.WaitForHuman {
		t.Fatal("expected node to wait for a human")
	}
	if output.NextNodeName != "" {
		t.Fatalf("expected no automatic successor while waiting, got %q", output.NextNodeName)
	}
}

func TestMockActionRequiresApprovedHumanDecision(t *testing.T) {
	_, _, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowName: InvoiceExceptionWorkflowName,
		NodeName:     "record_mock_action",
		Input:        json.RawMessage(`{"case_id":"synthetic-inv-case-001"}`),
	}, NodeDependencies{})
	if err == nil {
		t.Fatal("expected mock action to fail without human approval")
	}

	output, _, err := ExecuteNode(context.Background(), NodeInput{
		WorkflowName: InvoiceExceptionWorkflowName,
		NodeName:     "record_mock_action",
		Input:        json.RawMessage(`{"case_id":"synthetic-inv-case-001"}`),
		HumanDecision: json.RawMessage(`{
			"decision_key": "approve-1",
			"decision": "approved",
			"actor_id": "operator-1",
			"reason": "synthetic demo approval"
		}`),
	}, NodeDependencies{})
	if err != nil {
		t.Fatalf("execute record_mock_action: %v", err)
	}
	if output.NextNodeName != "compose_demo_summary" {
		t.Fatalf("expected next compose_demo_summary, got %q", output.NextNodeName)
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
