package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	response := request(t, http.MethodGet, "/healthz")

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := decodeBody(t, response)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestReadyz(t *testing.T) {
	response := request(t, http.MethodGet, "/readyz")

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := decodeBody(t, response)
	if body["status"] != "ready" {
		t.Fatalf("expected status ready, got %q", body["status"])
	}
}

func TestVersion(t *testing.T) {
	response := request(t, http.MethodGet, "/version")

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := decodeBody(t, response)
	if body["name"] != serviceName {
		t.Fatalf("expected service name %q, got %q", serviceName, body["name"])
	}
	if body["version"] != "test-version" {
		t.Fatalf("expected version test-version, got %q", body["version"])
	}
}

func TestRoutesRejectNonGETMethods(t *testing.T) {
	response := request(t, http.MethodPost, "/healthz")

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, allow)
	}
}

func request(t *testing.T, method string, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()

	NewServer("test-version").ServeHTTP(rec, req)

	return rec
}

func decodeBody(t *testing.T, response *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return body
}
