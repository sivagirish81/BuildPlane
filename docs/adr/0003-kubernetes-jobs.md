# ADR 0003: Kubernetes Jobs for Execution

Each attempt runs as one Kubernetes Job with explicit resources, deadlines, labels, and `backoffLimit: 0`. Kubernetes owns pod lifecycle; BuildPlane owns retries and state.
