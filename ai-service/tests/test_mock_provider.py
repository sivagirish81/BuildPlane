from app.providers.mock import MockIssueClassifier
from app.schemas import ClassificationRequest


def test_mock_provider_classifies_logistics_issue() -> None:
    classifier = MockIssueClassifier()

    result = classifier.classify(
        ClassificationRequest(
            case_id="case-002",
            description="Carrier delivery is blocked at the warehouse.",
        )
    )

    assert result.category == "logistics"
    assert result.urgency == "high"
    assert result.confidence > 0.7

