import asyncio

from fastapi import FastAPI, Request
from fastapi.responses import PlainTextResponse

from app.observability import (
    TRACEPARENT_HEADER,
    Metrics,
    configure_logging,
    log_event,
    new_traceparent,
    normalize_traceparent,
    now_seconds,
    trace_id,
)
from app.providers.base import IssueClassifierProvider
from app.providers.factory import build_provider
from app.schemas import ClassificationRequest, IssueClassification
from app.settings import Settings


def create_app(
    settings: Settings | None = None,
    provider: IssueClassifierProvider | None = None,
) -> FastAPI:
    configure_logging()
    resolved_settings = settings or Settings.from_env()
    classifier = provider or build_provider(resolved_settings)
    metrics = Metrics("buildplane-ai-service")

    app = FastAPI(title="BuildPlane AI Service", version="0.1.0")
    app.state.metrics = metrics

    @app.middleware("http")
    async def observe_requests(request: Request, call_next):
        traceparent = normalize_traceparent(request.headers.get(TRACEPARENT_HEADER)) or new_traceparent()
        correlation_id = request.headers.get("X-Correlation-ID", "")
        start = now_seconds()
        response = await call_next(request)
        duration = now_seconds() - start
        response.headers[TRACEPARENT_HEADER] = traceparent
        if correlation_id:
            response.headers["X-Correlation-ID"] = correlation_id

        labels = {
            "method": request.method,
            "path": route_label(request.url.path),
            "status": status_class(response.status_code),
        }
        metrics.inc("buildplane_ai_http_requests_total", labels)
        metrics.inc("buildplane_ai_http_request_duration_seconds_count", labels)
        metrics.add("buildplane_ai_http_request_duration_seconds_sum", labels, duration)
        log_event(
            "ai_http_request",
            method=request.method,
            path=request.url.path,
            status=response.status_code,
            duration_ms=round(duration * 1000),
            correlation_id=correlation_id,
            trace_id=trace_id(traceparent),
        )
        return response

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/readyz")
    def readyz() -> dict[str, str]:
        return {"status": "ready", "provider": resolved_settings.provider}

    @app.get("/metrics")
    def metrics_endpoint() -> PlainTextResponse:
        return PlainTextResponse(metrics.render(), media_type="text/plain; version=0.0.4")

    @app.post("/v1/classify-issue", response_model=IssueClassification)
    async def classify_issue(request: ClassificationRequest, http_request: Request) -> IssueClassification:
        request.traceparent = request.traceparent or normalize_traceparent(http_request.headers.get(TRACEPARENT_HEADER))
        start = now_seconds()
        try:
            result = await asyncio.to_thread(classifier.classify, request)
        except Exception:
            metrics.inc("buildplane_ai_classifications_total", {"provider": resolved_settings.provider, "result": "error"})
            raise
        duration = now_seconds() - start
        metrics.inc("buildplane_ai_classifications_total", {"provider": resolved_settings.provider, "result": "ok"})
        metrics.inc("buildplane_ai_classification_duration_seconds_count", {"provider": resolved_settings.provider})
        metrics.add("buildplane_ai_classification_duration_seconds_sum", {"provider": resolved_settings.provider}, duration)
        log_event(
            "ai_classification_completed",
            provider=resolved_settings.provider,
            case_id=request.case_id,
            category=result.category,
            urgency=result.urgency,
            duration_ms=round(duration * 1000),
            trace_id=trace_id(request.traceparent or ""),
        )
        return result

    return app


app = create_app()


def route_label(path: str) -> str:
    if path in {"/healthz", "/readyz", "/metrics", "/v1/classify-issue"}:
        return path
    return "other"


def status_class(status: int) -> str:
    if status >= 500:
        return "5xx"
    if status >= 400:
        return "4xx"
    if status >= 300:
        return "3xx"
    if status >= 200:
        return "2xx"
    return "unknown"
