from fastapi.testclient import TestClient

from app.main import create_app
from app.providers.mock import MockIssueClassifier
from app.settings import Settings


def test_health_and_ready_endpoints() -> None:
    client = TestClient(create_app(settings=Settings(), provider=MockIssueClassifier()))

    assert client.get("/healthz").json() == {"status": "ok"}
    assert client.get("/readyz").json() == {"status": "ready", "provider": "mock"}


def test_classify_issue_returns_structured_result() -> None:
    client = TestClient(create_app(settings=Settings(), provider=MockIssueClassifier()))

    response = client.post(
        "/v1/classify-issue",
        json={
            "workflow_run_id": "run-1",
            "node_name": "classify_issue",
            "case_id": "case-001",
            "customer_message": "Urgent invoice charge dispute needs escalation",
            "source": "test",
        },
    )

    assert response.status_code == 200
    body = response.json()
    assert body["case_id"] == "case-001"
    assert body["category"] == "billing"
    assert body["urgency"] == "high"
    assert body["provider"] == "mock"
    assert body["evidence"][0]["source"] == "test"
    assert response.headers["traceparent"]


def test_metrics_endpoint_contains_bounded_metrics() -> None:
    client = TestClient(create_app(settings=Settings(), provider=MockIssueClassifier()))

    client.get("/healthz")
    response = client.get("/metrics")

    assert response.status_code == 200
    assert "buildplane_ai_http_requests_total" in response.text
    assert 'service="buildplane-ai-service"' in response.text


def test_classify_issue_requires_context() -> None:
    client = TestClient(create_app(settings=Settings(), provider=MockIssueClassifier()))

    response = client.post("/v1/classify-issue", json={"case_id": "case-001"})

    assert response.status_code == 422
