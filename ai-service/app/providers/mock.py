from app.schemas import ClassificationRequest, Evidence, IssueCategory, IssueClassification, Urgency


class MockIssueClassifier:
    def __init__(self, model: str = "mock-issue-classifier", prompt_version: str = "issue_classifier_v1") -> None:
        self._model = model
        self._prompt_version = prompt_version

    def classify(self, request: ClassificationRequest) -> IssueClassification:
        text = " ".join(
            value
            for value in [
                request.title,
                request.description,
                request.customer_message,
                str(request.input or ""),
            ]
            if value
        ).lower()

        category = IssueCategory.UNKNOWN
        urgency = Urgency.MEDIUM
        confidence = 0.62
        action = "route to operations review"

        if any(word in text for word in ["invoice", "payment", "charge", "refund"]):
            category = IssueCategory.BILLING
            confidence = 0.82
            action = "route to billing operations"
        elif any(word in text for word in ["shipment", "carrier", "delivery", "warehouse"]):
            category = IssueCategory.LOGISTICS
            confidence = 0.8
            action = "route to logistics operations"
        elif any(word in text for word in ["policy", "contract", "regulation", "audit"]):
            category = IssueCategory.COMPLIANCE
            confidence = 0.78
            action = "route to compliance review"

        if any(word in text for word in ["blocked", "urgent", "escalate", "breach"]):
            urgency = Urgency.HIGH
        elif any(word in text for word in ["later", "minor", "low"]):
            urgency = Urgency.LOW

        evidence_text = request.customer_message or request.description or request.title or str(request.input or {})

        return IssueClassification(
            case_id=request.case_id,
            category=category,
            urgency=urgency,
            confidence=confidence,
            rationale="Deterministic mock classification based on issue keywords.",
            suggested_next_action=action,
            evidence=[Evidence(source=request.source or "workflow_input", text=evidence_text[:240])],
            provider="mock",
            model=self._model,
            prompt_version=self._prompt_version,
        )

