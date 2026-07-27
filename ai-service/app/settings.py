from dataclasses import dataclass
import os


@dataclass(frozen=True)
class Settings:
    provider: str = "mock"
    model: str = "gpt-5.6"
    prompt_version: str = "issue_classifier_v1"
    openai_api_key: str | None = None

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            provider=os.getenv("BUILDPLANE_AI_PROVIDER", "mock"),
            model=os.getenv("BUILDPLANE_AI_MODEL", "gpt-5.6"),
            prompt_version=os.getenv("BUILDPLANE_AI_PROMPT_VERSION", "issue_classifier_v1"),
            openai_api_key=os.getenv("OPENAI_API_KEY"),
        )

