package releases

import (
	"encoding/json"
	"fmt"
	"strings"
)

type classifierSpec struct {
	Rules           []classifierRule `json:"rules"`
	DefaultCategory string           `json:"default_category"`
}

type classifierRule struct {
	Category string   `json:"category"`
	Keywords []string `json:"keywords"`
}

type evaluationCase struct {
	Name             string
	WorkflowName     string
	Text             string
	ExpectedCategory string
}

func evaluateIssueClassifier(candidateSpec json.RawMessage, baselineSpec json.RawMessage) ([]EvaluationResult, json.RawMessage, bool, bool, error) {
	candidate, err := decodeClassifierSpec(candidateSpec)
	if err != nil {
		return nil, nil, false, false, fmt.Errorf("decode candidate spec: %w", err)
	}
	baseline, err := decodeClassifierSpec(baselineSpec)
	if err != nil {
		return nil, nil, false, false, fmt.Errorf("decode baseline spec: %w", err)
	}

	cases := syntheticIssueClassifierDataset()
	results := make([]EvaluationResult, 0, len(cases))
	candidatePassedCases := 0
	baselinePassedCases := 0
	for _, evalCase := range cases {
		baselineCategory := classifyWithSpec(baseline, evalCase.Text)
		candidateCategory := classifyWithSpec(candidate, evalCase.Text)
		baselinePassed := baselineCategory == evalCase.ExpectedCategory
		candidatePassed := candidateCategory == evalCase.ExpectedCategory
		if baselinePassed {
			baselinePassedCases++
		}
		if candidatePassed {
			candidatePassedCases++
		}

		details, err := json.Marshal(map[string]any{
			"text": evalCase.Text,
		})
		if err != nil {
			return nil, nil, false, false, fmt.Errorf("encode evaluation details: %w", err)
		}
		results = append(results, EvaluationResult{
			CaseName:          evalCase.Name,
			WorkflowName:      evalCase.WorkflowName,
			ExpectedCategory:  evalCase.ExpectedCategory,
			BaselineCategory:  baselineCategory,
			CandidateCategory: candidateCategory,
			BaselinePassed:    baselinePassed,
			CandidatePassed:   candidatePassed,
			Details:           json.RawMessage(details),
		})
	}

	candidatePassed := candidatePassedCases == len(cases)
	baselinePassed := baselinePassedCases == len(cases)
	summary, err := json.Marshal(map[string]any{
		"dataset_name":           DefaultDatasetName,
		"candidate_passed":       candidatePassed,
		"candidate_passed_cases": candidatePassedCases,
		"candidate_total_cases":  len(cases),
		"baseline_passed":        baselinePassed,
		"baseline_passed_cases":  baselinePassedCases,
		"baseline_total_cases":   len(cases),
	})
	if err != nil {
		return nil, nil, false, false, fmt.Errorf("encode evaluation summary: %w", err)
	}

	return results, json.RawMessage(summary), candidatePassed, baselinePassed, nil
}

func decodeClassifierSpec(raw json.RawMessage) (classifierSpec, error) {
	var spec classifierSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return classifierSpec{}, err
	}
	if len(spec.Rules) == 0 {
		return classifierSpec{}, ErrInvalidComponentVersion
	}
	if strings.TrimSpace(spec.DefaultCategory) == "" {
		spec.DefaultCategory = "general"
	}
	for _, rule := range spec.Rules {
		if strings.TrimSpace(rule.Category) == "" || len(rule.Keywords) == 0 {
			return classifierSpec{}, ErrInvalidComponentVersion
		}
	}
	return spec, nil
}

func classifyWithSpec(spec classifierSpec, text string) string {
	lowerText := strings.ToLower(text)
	for _, rule := range spec.Rules {
		for _, keyword := range rule.Keywords {
			if keyword = strings.TrimSpace(strings.ToLower(keyword)); keyword != "" && strings.Contains(lowerText, keyword) {
				return rule.Category
			}
		}
	}
	return spec.DefaultCategory
}

func syntheticIssueClassifierDataset() []evaluationCase {
	return []evaluationCase{
		{
			Name:             "invoice-price-mismatch",
			WorkflowName:     "demo.invoice-exception",
			Text:             "Synthetic invoice has a disputed charge and payment should be reviewed.",
			ExpectedCategory: "billing",
		},
		{
			Name:             "freight-late-delivery",
			WorkflowName:     "demo.freight-exception",
			Text:             "Synthetic freight shipment arrived late with a carrier exception.",
			ExpectedCategory: "logistics",
		},
		{
			Name:             "portal-access-blocked",
			WorkflowName:     "phase4.local-demo",
			Text:             "Synthetic user cannot access the operations portal.",
			ExpectedCategory: "access",
		},
	}
}
