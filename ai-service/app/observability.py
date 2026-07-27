from __future__ import annotations

import json
import logging
import secrets
import threading
from time import time
from typing import Iterable


TRACEPARENT_HEADER = "traceparent"


class Metrics:
    def __init__(self, service: str) -> None:
        self._service = service
        self._lock = threading.Lock()
        self._counters: dict[tuple[str, tuple[tuple[str, str], ...]], float] = {}
        self._gauges: dict[tuple[str, tuple[tuple[str, str], ...]], float] = {}

    def inc(self, name: str, labels: dict[str, str] | None = None) -> None:
        self.add(name, labels, 1.0)

    def add(self, name: str, labels: dict[str, str] | None, value: float) -> None:
        key = self._key(name, labels)
        with self._lock:
            self._counters[key] = self._counters.get(key, 0.0) + value

    def set_gauge(self, name: str, labels: dict[str, str] | None, value: float) -> None:
        key = self._key(name, labels)
        with self._lock:
            self._gauges[key] = value

    def render(self) -> str:
        with self._lock:
            counters = dict(self._counters)
            gauges = dict(self._gauges)

        lines: list[str] = []
        self._render_samples(lines, "counter", counters)
        self._render_samples(lines, "gauge", gauges)
        return "\n".join(lines) + "\n"

    def _render_samples(
        self,
        lines: list[str],
        metric_type: str,
        samples: dict[tuple[str, tuple[tuple[str, str], ...]], float],
    ) -> None:
        seen: set[str] = set()
        for (name, labels), value in sorted(samples.items()):
            if name not in seen:
                lines.append(f"# TYPE {name} {metric_type}")
                seen.add(name)
            lines.append(f"{name}{_format_labels(labels)} {value:.6f}")

    def _key(self, name: str, labels: dict[str, str] | None) -> tuple[str, tuple[tuple[str, str], ...]]:
        merged = {"service": self._service}
        if labels:
            merged.update(labels)
        return name, tuple(sorted(merged.items()))


def configure_logging() -> None:
    logging.basicConfig(level=logging.INFO, format="%(message)s")


def log_event(event: str, **fields: object) -> None:
    payload = {"event": event, **fields}
    logging.getLogger("buildplane.ai").info(json.dumps(payload, sort_keys=True))


def normalize_traceparent(value: str | None) -> str:
    if not value:
        return ""
    value = value.strip().lower()
    parts = value.split("-")
    if len(parts) != 4:
        return ""
    if len(parts[0]) != 2 or len(parts[1]) != 32 or len(parts[2]) != 16 or len(parts[3]) != 2:
        return ""
    return value


def new_traceparent() -> str:
    return f"00-{secrets.token_hex(16)}-{secrets.token_hex(8)}-01"


def trace_id(traceparent: str) -> str:
    normalized = normalize_traceparent(traceparent)
    if not normalized:
        return ""
    return normalized.split("-")[1]


def now_seconds() -> float:
    return time()


def _format_labels(labels: Iterable[tuple[str, str]]) -> str:
    labels = tuple(labels)
    if not labels:
        return ""
    rendered = ",".join(f'{key}="{_escape_label(value)}"' for key, value in labels)
    return "{" + rendered + "}"


def _escape_label(value: str) -> str:
    return value.replace("\\", "\\\\").replace("\n", "\\n").replace('"', '\\"')

