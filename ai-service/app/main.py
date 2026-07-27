import asyncio

from fastapi import FastAPI

from app.providers.base import IssueClassifierProvider
from app.providers.factory import build_provider
from app.schemas import ClassificationRequest, IssueClassification
from app.settings import Settings


def create_app(
    settings: Settings | None = None,
    provider: IssueClassifierProvider | None = None,
) -> FastAPI:
    resolved_settings = settings or Settings.from_env()
    classifier = provider or build_provider(resolved_settings)

    app = FastAPI(title="BuildPlane AI Service", version="0.1.0")

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/readyz")
    def readyz() -> dict[str, str]:
        return {"status": "ready", "provider": resolved_settings.provider}

    @app.post("/v1/classify-issue", response_model=IssueClassification)
    async def classify_issue(request: ClassificationRequest) -> IssueClassification:
        return await asyncio.to_thread(classifier.classify, request)

    return app


app = create_app()

