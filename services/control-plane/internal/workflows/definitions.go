package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const LocalDemoWorkflowName = "phase4.local-demo"

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
	Input         json.RawMessage
}

type NodeOutput struct {
	Result       json.RawMessage
	NextNodeName string
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
	definition := WorkflowDefinition{
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

	if workflowName != definition.Name {
		return WorkflowDefinition{}, false
	}
	return definition, true
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
		if output.NextNodeName == "" && index+1 < len(definition.Nodes) {
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
