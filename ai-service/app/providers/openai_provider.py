from pathlib import Path

from app.schemas import ClassificationRequest, IssueClassification


class OpenAIIssueClassifier:
    def __init__(self, api_key: str | None, model: str, prompt_version: str) -> None:
        if not api_key:
            raise ValueError("OPENAI_API_KEY is required when BUILDPLANE_AI_PROVIDER=openai")
        try:
            from openai import OpenAI
        except ModuleNotFoundError as exc:
            raise RuntimeError("install the openai extra to use the OpenAI provider") from exc

        self._client = OpenAI(api_key=api_key)
        self._model = model
        self._prompt_version = prompt_version
        self._prompt = _load_prompt(prompt_version)

    def classify(self, request: ClassificationRequest) -> IssueClassification:
        response = self._client.responses.parse(
            model=self._model,
            instructions=self._prompt,
            input=_format_request(request),
            text_format=IssueClassification,
        )

        parsed = getattr(response, "output_parsed", None)
        if parsed is None:
            raise RuntimeError("OpenAI response did not include a parsed classification")

        parsed.provider = "openai"
        parsed.model = self._model
        parsed.prompt_version = self._prompt_version
        return parsed


def _load_prompt(prompt_version: str) -> str:
    prompt_path = Path(__file__).resolve().parents[2] / "prompts" / f"{prompt_version}.md"
    return prompt_path.read_text(encoding="utf-8")


def _format_request(request: ClassificationRequest) -> str:
    return request.model_dump_json(exclude_none=True)

