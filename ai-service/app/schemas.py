from enum import Enum
from typing import Any

from pydantic import BaseModel, Field, model_validator


class IssueCategory(str, Enum):
    BILLING = "billing"
    LOGISTICS = "logistics"
    COMPLIANCE = "compliance"
    CUSTOMER_EXCEPTION = "customer_exception"
    UNKNOWN = "unknown"


class Urgency(str, Enum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"


class ClassificationRequest(BaseModel):
    workflow_run_id: str | None = None
    node_name: str | None = None
    case_id: str = Field(min_length=1)
    title: str | None = None
    description: str | None = None
    customer_message: str | None = None
    source: str | None = None
    input: dict[str, Any] | None = None

    @model_validator(mode="after")
    def require_issue_context(self) -> "ClassificationRequest":
        if self.title or self.description or self.customer_message or self.input:
            return self
        raise ValueError("classification requires title, description, customer_message, or input")


class Evidence(BaseModel):
    source: str
    text: str


class IssueClassification(BaseModel):
    case_id: str
    category: IssueCategory
    urgency: Urgency
    confidence: float = Field(ge=0.0, le=1.0)
    rationale: str
    suggested_next_action: str
    evidence: list[Evidence] = Field(default_factory=list)
    provider: str = "unknown"
    model: str = "unknown"
    prompt_version: str = "unknown"
