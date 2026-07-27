# ADR 0005: Bounded Python AI Service

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane needs AI-powered workflow nodes, but the durable control plane should
not let model calls own orchestration state. The Go worker already has leases,
fencing tokens, retries, audit records, and idempotent completion. The AI layer
needs to fit inside that model.

OpenAI structured outputs let the service ask for JSON that matches a schema,
but BuildPlane still needs application-side validation, bounded provider code,
and deterministic tests.

## Decision

Add a separate Python FastAPI AI service with Pydantic request and response
schemas. The service exposes `POST /v1/classify-issue`.

The default provider is a deterministic mock provider. An OpenAI provider exists
behind `BUILDPLANE_AI_PROVIDER=openai` and requires `OPENAI_API_KEY`.

The Go worker calls the AI service only through a typed `AIClassifier`
interface. The workflow node `classify_issue` is added to
`phase4.local-demo`, making the new node order:

```text
validate_input -> classify_issue -> compose_summary
```

The worker remains responsible for leases, retries, audit, and durable state.
The Python service is responsible only for classification.

## Consequences

- AI calls are isolated behind an HTTP boundary and can be scaled separately in
  Kubernetes.
- Tests can run deterministically with the mock provider and without live model
  calls.
- Provider failures use the existing node retry path.
- Prompt text is versioned in `ai-service/prompts`.
- The OpenAI provider is present but not live-tested without credentials and
  network access.

