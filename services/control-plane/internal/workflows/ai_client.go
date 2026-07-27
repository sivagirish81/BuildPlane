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

	"github.com/sivagirish/buildplane/services/control-plane/internal/observability"
)

type IssueClassificationRequest struct {
	WorkflowRunID   string          `json:"workflow_run_id,omitempty"`
	NodeName        string          `json:"node_name,omitempty"`
	TraceParent     string          `json:"traceparent,omitempty"`
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
	metrics    *observability.Registry
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

func (c *HTTPAIClient) SetMetrics(metrics *observability.Registry) {
	c.metrics = metrics
}

func (c *HTTPAIClient) ClassifyIssue(ctx context.Context, request IssueClassificationRequest) (json.RawMessage, error) {
	start := time.Now()
	body, err := json.Marshal(request)
	if err != nil {
		c.recordRequest("error", time.Since(start))
		return nil, fmt.Errorf("encode AI classification request: %w", err)
	}

	endpoint, err := url.JoinPath(c.baseURL, "/v1/classify-issue")
	if err != nil {
		c.recordRequest("error", time.Since(start))
		return nil, fmt.Errorf("build AI classification URL: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		c.recordRequest("error", time.Since(start))
		return nil, fmt.Errorf("build AI classification request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	if request.TraceParent != "" {
		httpRequest.Header.Set("traceparent", request.TraceParent)
	}

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		c.recordRequest("error", time.Since(start))
		return nil, fmt.Errorf("call AI classification service: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		c.recordRequest("error", time.Since(start))
		return nil, fmt.Errorf("AI classification service returned %s: %s", response.Status, strings.TrimSpace(string(errorBody)))
	}

	var result json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		c.recordRequest("error", time.Since(start))
		return nil, fmt.Errorf("decode AI classification response: %w", err)
	}
	if len(result) == 0 {
		c.recordRequest("error", time.Since(start))
		return nil, fmt.Errorf("AI classification response was empty")
	}
	c.recordRequest("ok", time.Since(start))
	return result, nil
}

func (c *HTTPAIClient) recordRequest(result string, duration time.Duration) {
	if c.metrics == nil {
		return
	}
	labels := map[string]string{"result": result}
	c.metrics.Inc("buildplane_ai_client_requests_total", labels)
	c.metrics.Inc("buildplane_ai_client_request_duration_seconds_count", labels)
	c.metrics.Add("buildplane_ai_client_request_duration_seconds_sum", labels, duration.Seconds())
}
