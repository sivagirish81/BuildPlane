package workflows

import (
	"encoding/json"
	"errors"
	"fmt"
)

const LocalDemoWorkflowName = "phase4.local-demo"

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

type NodeRunner func(input NodeInput) (NodeOutput, error)

type NodeInput struct {
	WorkflowName string
	NodeName     string
	Input        json.RawMessage
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

func ExecuteNode(input NodeInput) (NodeOutput, int, error) {
	definition, ok := DefinitionFor(input.WorkflowName)
	if !ok {
		return NodeOutput{}, 1, fmt.Errorf("%w: %s", ErrUnknownWorkflow, input.WorkflowName)
	}

	for index, node := range definition.Nodes {
		if node.Name != input.NodeName {
			continue
		}

		output, err := node.Run(input)
		if output.NextNodeName == "" && index+1 < len(definition.Nodes) {
			output.NextNodeName = definition.Nodes[index+1].Name
		}
		return output, maxAttempts(node.MaxAttempts), err
	}

	return NodeOutput{}, 1, fmt.Errorf("%w: %s", ErrUnknownNode, input.NodeName)
}

func validateInput(input NodeInput) (NodeOutput, error) {
	var payload struct {
		CaseID string `json:"case_id"`
	}
	if err := json.Unmarshal(input.Input, &payload); err != nil {
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

func composeSummary(input NodeInput) (NodeOutput, error) {
	var payload struct {
		CaseID string `json:"case_id"`
	}
	if err := json.Unmarshal(input.Input, &payload); err != nil {
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

func maxAttempts(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}
