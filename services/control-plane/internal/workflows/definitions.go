package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	LocalDemoWorkflowName        = "phase4.local-demo"
	InvoiceExceptionWorkflowName = "demo.invoice-exception"
	FreightExceptionWorkflowName = "demo.freight-exception"
	IssueClassifierComponentName = "issue_classifier"
)

const (
	WorkerPoolGeneral = "general"
	WorkerPoolAI      = "ai"
)

var (
	ErrUnknownWorkflow = errors.New("unknown workflow")
	ErrUnknownNode     = errors.New("unknown workflow node")
)

type WorkflowDefinition struct {
	Name  string
	Nodes []NodeDefinition
}

type ComponentDependency struct {
	WorkflowName  string `json:"workflow_name"`
	NodeName      string `json:"node_name"`
	ComponentName string `json:"component_name"`
}

type NodeDefinition struct {
	Name        string
	MaxAttempts int
	Run         NodeRunner
}

type NodeRunner func(ctx context.Context, input NodeInput, dependencies NodeDependencies) (NodeOutput, error)

type NodeDependencies struct {
	AIClassifier AIClassifier
}

type NodeInput struct {
	WorkflowRunID string
	WorkflowName  string
	NodeName      string
	CorrelationID string
	TraceParent   string
	HumanDecision json.RawMessage
	Input         json.RawMessage
}

type NodeOutput struct {
	Result       json.RawMessage
	NextNodeName string
	WaitForHuman bool
}

func FirstNodeName(workflowName string) string {
	definition, ok := DefinitionFor(workflowName)
	if !ok || len(definition.Nodes) == 0 {
		return "validate_input"
	}
	return definition.Nodes[0].Name
}

func WorkerPoolForNode(nodeName string) string {
	switch nodeName {
	case "classify_issue":
		return WorkerPoolAI
	default:
		return WorkerPoolGeneral
	}
}

func DefinitionFor(workflowName string) (WorkflowDefinition, bool) {
	definitions := map[string]WorkflowDefinition{
		LocalDemoWorkflowName: {
			Name: LocalDemoWorkflowName,
			Nodes: []NodeDefinition{
				{
					Name:        "validate_input",
					MaxAttempts: 2,
					Run:         validateInput,
				},
				{
					Name:        "classify_issue",
					MaxAttempts: 2,
					Run:         classifyIssue,
				},
				{
					Name:        "compose_summary",
					MaxAttempts: 2,
					Run:         composeSummary,
				},
			},
		},
		InvoiceExceptionWorkflowName: demoWorkflowDefinition(InvoiceExceptionWorkflowName),
		FreightExceptionWorkflowName: demoWorkflowDefinition(FreightExceptionWorkflowName),
	}

	definition, ok := definitions[workflowName]
	return definition, ok
}

func demoWorkflowDefinition(name string) WorkflowDefinition {
	return WorkflowDefinition{
		Name: name,
		Nodes: []NodeDefinition{
			{
				Name:        "validate_demo_input",
				MaxAttempts: 2,
				Run:         validateDemoInput,
			},
			{
				Name:        "classify_issue",
				MaxAttempts: 2,
				Run:         classifyIssue,
			},
			{
				Name:        "plan_demo_resolution",
				MaxAttempts: 2,
				Run:         planDemoResolution,
			},
			{
				Name:        "await_human_approval",
				MaxAttempts: 1,
				Run:         awaitHumanApproval,
			},
			{
				Name:        "record_mock_action",
				MaxAttempts: 1,
				Run:         recordMockAction,
			},
			{
				Name:        "compose_demo_summary",
				MaxAttempts: 2,
				Run:         composeDemoSummary,
			},
		},
	}
}

func NextNodeAfterApproval(workflowName string) string {
	definition, ok := DefinitionFor(workflowName)
	if !ok {
		return ""
	}
	for index, node := range definition.Nodes {
		if node.Name == "await_human_approval" && index+1 < len(definition.Nodes) {
			return definition.Nodes[index+1].Name
		}
	}
	return ""
}

func DependencyGraphFor(workflowName string) ([]string, bool) {
	definition, ok := DefinitionFor(workflowName)
	if !ok {
		return nil, false
	}
	nodes := make([]string, 0, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodes = append(nodes, node.Name)
	}
	return nodes, true
}

func ComponentDependenciesFor(componentName string) []ComponentDependency {
	if componentName != IssueClassifierComponentName {
		return nil
	}
	return []ComponentDependency{
		{
			WorkflowName:  LocalDemoWorkflowName,
			NodeName:      "classify_issue",
			ComponentName: componentName,
		},
		{
			WorkflowName:  InvoiceExceptionWorkflowName,
			NodeName:      "classify_issue",
			ComponentName: componentName,
		},
		{
			WorkflowName:  FreightExceptionWorkflowName,
			NodeName:      "classify_issue",
			ComponentName: componentName,
		},
	}
}

func validateDemoInput(_ context.Context, input NodeInput, _ NodeDependencies) (NodeOutput, error) {
	payload, err := decodeDemoWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode demo workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	required := []string{"case_id", "title", "description", "customer_message", "source"}
	missing := missingDemoFields(payload, required)
	if input.WorkflowName == InvoiceExceptionWorkflowName {
		required = append(required, "invoice_id", "vendor_name", "amount_disputed")
		if payload.InvoiceID == "" {
			missing = append(missing, "invoice_id")
		}
		if payload.VendorName == "" {
			missing = append(missing, "vendor_name")
		}
		if payload.AmountDisputed <= 0 {
			missing = append(missing, "amount_disputed")
		}
	}
	if input.WorkflowName == FreightExceptionWorkflowName {
		required = append(required, "shipment_id", "carrier_name", "exception_code")
		if payload.ShipmentID == "" {
			missing = append(missing, "shipment_id")
		}
		if payload.CarrierName == "" {
			missing = append(missing, "carrier_name")
		}
		if payload.ExceptionCode == "" {
			missing = append(missing, "exception_code")
		}
	}
	if len(missing) > 0 {
		return NodeOutput{}, fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}

	result, err := json.Marshal(map[string]any{
		"valid":           true,
		"case_id":         payload.CaseID,
		"workflow_name":   input.WorkflowName,
		"required_fields": required,
	})
	if err != nil {
		return NodeOutput{}, fmt.Errorf("encode demo validation result: %w", err)
	}

	return NodeOutput{Result: json.RawMessage(result)}, nil
}

func missingDemoFields(payload demoWorkflowInput, fields []string) []string {
	var missing []string
	for _, field := range fields {
		switch field {
		case "case_id":
			if payload.CaseID == "" {
				missing = append(missing, field)
			}
		case "title":
			if payload.Title == "" {
				missing = append(missing, field)
			}
		case "description":
			if payload.Description == "" {
				missing = append(missing, field)
			}
		case "customer_message":
			if payload.CustomerMessage == "" {
				missing = append(missing, field)
			}
		case "source":
			if payload.Source == "" {
				missing = append(missing, field)
			}
		}
	}
	return missing
}

func planDemoResolution(_ context.Context, input NodeInput, _ NodeDependencies) (NodeOutput, error) {
	payload, err := decodeDemoWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode demo workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	action := map[string]any{
		"type":        "create_review_task",
		"description": "Create a synthetic operations review task",
	}
	if input.WorkflowName == InvoiceExceptionWorkflowName {
		action["type"] = "create_invoice_credit_review"
		action["invoice_id"] = payload.InvoiceID
		action["vendor_name"] = payload.VendorName
		action["amount_disputed"] = payload.AmountDisputed
	}
	if input.WorkflowName == FreightExceptionWorkflowName {
		action["type"] = "create_freight_exception_review"
		action["shipment_id"] = payload.ShipmentID
		action["carrier_name"] = payload.CarrierName
		action["exception_code"] = payload.ExceptionCode
	}

	graph, _ := DependencyGraphFor(input.WorkflowName)
	result, err := json.Marshal(map[string]any{
		"case_id":          payload.CaseID,
		"dependency_graph": graph,
		"proposed_action":  action,
		"requires_human":   true,
	})
	if err != nil {
		return NodeOutput{}, fmt.Errorf("encode demo resolution plan: %w", err)
	}

	return NodeOutput{Result: json.RawMessage(result)}, nil
}

func awaitHumanApproval(_ context.Context, input NodeInput, _ NodeDependencies) (NodeOutput, error) {
	payload, err := decodeDemoWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode demo workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	result, err := json.Marshal(map[string]any{
		"case_id":           payload.CaseID,
		"decision_required": true,
		"allowed_decisions": []string{"approved", "rejected"},
		"decision_endpoint": fmt.Sprintf("/v1/workflow-runs/%s/decisions", input.WorkflowRunID),
	})
	if err != nil {
		return NodeOutput{}, fmt.Errorf("encode human approval prompt: %w", err)
	}

	return NodeOutput{Result: json.RawMessage(result), WaitForHuman: true}, nil
}

func recordMockAction(_ context.Context, input NodeInput, _ NodeDependencies) (NodeOutput, error) {
	decision, err := decodeHumanDecision(input.HumanDecision)
	if err != nil {
		return NodeOutput{}, err
	}
	if decision.Decision != "approved" {
		return NodeOutput{}, errors.New("approved human decision is required before mock action")
	}

	payload, err := decodeDemoWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode demo workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	result, err := json.Marshal(map[string]any{
		"case_id":       payload.CaseID,
		"mock_action":   mockActionType(input.WorkflowName),
		"external":      false,
		"guarded_by":    decision.DecisionKey,
		"approved_by":   decision.ActorID,
		"approval_note": decision.Reason,
	})
	if err != nil {
		return NodeOutput{}, fmt.Errorf("encode mock action result: %w", err)
	}

	return NodeOutput{Result: json.RawMessage(result)}, nil
}

func composeDemoSummary(_ context.Context, input NodeInput, _ NodeDependencies) (NodeOutput, error) {
	payload, err := decodeDemoWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode demo workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	result, err := json.Marshal(map[string]any{
		"case_id":       payload.CaseID,
		"workflow_name": input.WorkflowName,
		"summary":       fmt.Sprintf("completed synthetic %s demo for case %s", input.WorkflowName, payload.CaseID),
	})
	if err != nil {
		return NodeOutput{}, fmt.Errorf("encode demo summary result: %w", err)
	}

	return NodeOutput{Result: json.RawMessage(result)}, nil
}

func mockActionType(workflowName string) string {
	switch workflowName {
	case InvoiceExceptionWorkflowName:
		return "invoice_credit_review_created"
	case FreightExceptionWorkflowName:
		return "freight_exception_review_created"
	default:
		return "synthetic_review_created"
	}
}

type demoWorkflowInput struct {
	CaseID          string  `json:"case_id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	CustomerMessage string  `json:"customer_message"`
	Source          string  `json:"source"`
	InvoiceID       string  `json:"invoice_id"`
	VendorName      string  `json:"vendor_name"`
	AmountDisputed  float64 `json:"amount_disputed"`
	ShipmentID      string  `json:"shipment_id"`
	CarrierName     string  `json:"carrier_name"`
	ExceptionCode   string  `json:"exception_code"`
}

func decodeDemoWorkflowInput(input json.RawMessage) (demoWorkflowInput, error) {
	var payload demoWorkflowInput
	if err := json.Unmarshal(input, &payload); err != nil {
		return demoWorkflowInput{}, err
	}
	return payload, nil
}

type humanDecisionInput struct {
	DecisionKey string `json:"decision_key"`
	Decision    string `json:"decision"`
	ActorID     string `json:"actor_id"`
	Reason      string `json:"reason"`
}

func decodeHumanDecision(input json.RawMessage) (humanDecisionInput, error) {
	if len(input) == 0 {
		return humanDecisionInput{}, errors.New("human decision is required before mock action")
	}
	var decision humanDecisionInput
	if err := json.Unmarshal(input, &decision); err != nil {
		return humanDecisionInput{}, fmt.Errorf("decode human decision: %w", err)
	}
	if decision.Decision != "approved" {
		return humanDecisionInput{}, errors.New("approved human decision is required before mock action")
	}
	if decision.DecisionKey == "" || decision.ActorID == "" {
		return humanDecisionInput{}, errors.New("human decision metadata is incomplete")
	}
	return decision, nil
}

func legacyLocalDefinition() WorkflowDefinition {
	return WorkflowDefinition{
		Name: LocalDemoWorkflowName,
		Nodes: []NodeDefinition{
			{
				Name:        "validate_input",
				MaxAttempts: 2,
				Run:         validateInput,
			},
			{
				Name:        "classify_issue",
				MaxAttempts: 2,
				Run:         classifyIssue,
			},
			{
				Name:        "compose_summary",
				MaxAttempts: 2,
				Run:         composeSummary,
			},
		},
	}
}

func ExecuteNode(ctx context.Context, input NodeInput, dependencies NodeDependencies) (NodeOutput, int, error) {
	definition, ok := DefinitionFor(input.WorkflowName)
	if !ok {
		return NodeOutput{}, 1, fmt.Errorf("%w: %s", ErrUnknownWorkflow, input.WorkflowName)
	}

	for index, node := range definition.Nodes {
		if node.Name != input.NodeName {
			continue
		}

		output, err := node.Run(ctx, input, dependencies)
		if output.NextNodeName == "" && !output.WaitForHuman && index+1 < len(definition.Nodes) {
			output.NextNodeName = definition.Nodes[index+1].Name
		}
		return output, maxAttempts(node.MaxAttempts), err
	}

	return NodeOutput{}, 1, fmt.Errorf("%w: %s", ErrUnknownNode, input.NodeName)
}

func validateInput(_ context.Context, input NodeInput, _ NodeDependencies) (NodeOutput, error) {
	payload, err := decodeLocalWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode local workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	result, err := json.Marshal(map[string]any{
		"valid":   true,
		"case_id": payload.CaseID,
	})
	if err != nil {
		return NodeOutput{}, fmt.Errorf("encode validation result: %w", err)
	}

	return NodeOutput{
		Result: json.RawMessage(result),
	}, nil
}

func classifyIssue(ctx context.Context, input NodeInput, dependencies NodeDependencies) (NodeOutput, error) {
	if dependencies.AIClassifier == nil {
		return NodeOutput{}, errors.New("AI classifier dependency is required")
	}

	payload, err := decodeLocalWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode local workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	result, err := dependencies.AIClassifier.ClassifyIssue(ctx, IssueClassificationRequest{
		WorkflowRunID:   input.WorkflowRunID,
		NodeName:        input.NodeName,
		TraceParent:     input.TraceParent,
		CaseID:          payload.CaseID,
		Title:           payload.Title,
		Description:     payload.Description,
		CustomerMessage: payload.CustomerMessage,
		Source:          payload.Source,
		Input:           input.Input,
	})
	if err != nil {
		return NodeOutput{}, err
	}

	return NodeOutput{
		Result: result,
	}, nil
}

func composeSummary(_ context.Context, input NodeInput, _ NodeDependencies) (NodeOutput, error) {
	payload, err := decodeLocalWorkflowInput(input.Input)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("decode local workflow input: %w", err)
	}
	if payload.CaseID == "" {
		return NodeOutput{}, errors.New("case_id is required")
	}

	result, err := json.Marshal(map[string]any{
		"case_id": payload.CaseID,
		"summary": fmt.Sprintf("validated synthetic case %s", payload.CaseID),
	})
	if err != nil {
		return NodeOutput{}, fmt.Errorf("encode summary result: %w", err)
	}

	return NodeOutput{
		Result: json.RawMessage(result),
	}, nil
}

type localWorkflowInput struct {
	CaseID          string `json:"case_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	CustomerMessage string `json:"customer_message"`
	Source          string `json:"source"`
}

func decodeLocalWorkflowInput(input json.RawMessage) (localWorkflowInput, error) {
	var payload localWorkflowInput
	if err := json.Unmarshal(input, &payload); err != nil {
		return localWorkflowInput{}, err
	}
	return payload, nil
}

func maxAttempts(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}
