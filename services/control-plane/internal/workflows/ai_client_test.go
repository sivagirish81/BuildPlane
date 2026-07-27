package workflows

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPAIClientPropagatesTraceParent(t *testing.T) {
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("traceparent"); got != traceparent {
			t.Fatalf("expected traceparent %q, got %q", traceparent, got)
		}
		body, err := json.Marshal(map[string]any{
			"case_id":               "case-001",
			"category":              "billing",
			"urgency":               "high",
			"confidence":            0.9,
			"rationale":             "test",
			"suggested_next_action": "test",
			"provider":              "mock",
			"model":                 "mock",
			"prompt_version":        "test",
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})

	client, err := NewHTTPAIClient("http://buildplane-ai-service:8090", time.Second)
	if err != nil {
		t.Fatalf("new ai client: %v", err)
	}
	client.httpClient.Transport = transport

	_, err = client.ClassifyIssue(context.Background(), IssueClassificationRequest{
		TraceParent: traceparent,
		CaseID:      "case-001",
		Description: "invoice issue",
	})
	if err != nil {
		t.Fatalf("classify issue: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
