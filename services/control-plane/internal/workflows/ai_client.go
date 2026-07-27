package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type IssueClassificationRequest struct {
	WorkflowRunID   string          `json:"workflow_run_id,omitempty"`
	NodeName        string          `json:"node_name,omitempty"`
	CaseID          string          `json:"case_id"`
	Title           string          `json:"title,omitempty"`
	Description     string          `json:"description,omitempty"`
	CustomerMessage string          `json:"customer_message,omitempty"`
	Source          string          `json:"source,omitempty"`
	Input           json.RawMessage `json:"input,omitempty"`
}

type AIClassifier interface {
	ClassifyIssue(ctx context.Context, request IssueClassificationRequest) (json.RawMessage, error)
}

type HTTPAIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPAIClient(baseURL string, timeout time.Duration) (*HTTPAIClient, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("AI service URL is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPAIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *HTTPAIClient) ClassifyIssue(ctx context.Context, request IssueClassificationRequest) (json.RawMessage, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode AI classification request: %w", err)
	}

	endpoint, err := url.JoinPath(c.baseURL, "/v1/classify-issue")
	if err != nil {
		return nil, fmt.Errorf("build AI classification URL: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build AI classification request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("call AI classification service: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("AI classification service returned %s: %s", response.Status, strings.TrimSpace(string(errorBody)))
	}

	var result json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode AI classification response: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("AI classification response was empty")
	}
	return result, nil
}
