from app.providers.base import IssueClassifierProvider
from app.providers.mock import MockIssueClassifier
from app.providers.openai_provider import OpenAIIssueClassifier
from app.settings import Settings


def build_provider(settings: Settings) -> IssueClassifierProvider:
    provider = settings.provider.strip().lower()
    if provider == "mock":
        return MockIssueClassifier(model="mock-issue-classifier", prompt_version=settings.prompt_version)
    if provider == "openai":
        return OpenAIIssueClassifier(
            api_key=settings.openai_api_key,
            model=settings.model,
            prompt_version=settings.prompt_version,
        )
    raise ValueError(f"unknown AI provider: {settings.provider}")

