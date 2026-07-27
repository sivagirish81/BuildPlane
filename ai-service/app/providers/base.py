from typing import Protocol

from app.schemas import ClassificationRequest, IssueClassification


class IssueClassifierProvider(Protocol):
    def classify(self, request: ClassificationRequest) -> IssueClassification:
        """Classify an issue using a bounded provider implementation."""

