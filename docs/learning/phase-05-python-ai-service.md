# Phase 5: Python AI Service

## What You Built

Phase 5 added a bounded AI service and connected it to the durable workflow
engine.

The new service lives in `ai-service/` and exposes:

- `GET /healthz`
- `GET /readyz`
- `POST /v1/classify-issue`

The Go workflow now runs:

```text
validate_input -> classify_issue -> compose_summary
```

## The Kubernetes Lesson

A Kubernetes Service gives a stable network name to changing Pods.

The worker does not need to know which AI service Pod is currently alive. It
calls:

```text
http://buildplane-ai-service:8090
```

Inside the cluster, Kubernetes DNS resolves that Service name and routes traffic
to a ready AI service Pod.

## Why The AI Service Is Separate

The Go control plane owns durable orchestration:

- workflow run state
- node execution state
- leases
- retries
- fencing tokens
- audit records

The Python service owns model-facing behavior:

- prompt version
- provider selection
- Pydantic request validation
- Pydantic response validation
- mock provider for deterministic tests
- optional OpenAI provider

This keeps the model boundary small. If classification fails, the worker treats
it like any other node failure and uses the existing retry path.

## Structured Outputs

Structured outputs mean the model is asked to produce data matching a declared
schema. In BuildPlane, the schema is represented by the `IssueClassification`
Pydantic model.

This does not replace validation. BuildPlane still validates the response at the
service boundary and stores the node result only after the worker completes the
fenced node execution.

## Local Commands

Create a virtual environment and run the AI service tests:

```bash
python3 -m venv .cache/ai-service-venv
.cache/ai-service-venv/bin/pip install -e 'ai-service[dev]'
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
```

Run the AI service locally:

```bash
.cache/ai-service-venv/bin/python -m uvicorn app.main:app --app-dir ai-service --host 0.0.0.0 --port 8090
```

Then classify a synthetic issue:

```bash
curl -i -X POST http://localhost:8090/v1/classify-issue \
  -H 'Content-Type: application/json' \
  -d '{"case_id":"synthetic-case-001","customer_message":"Urgent invoice charge dispute needs escalation"}'
```

## OpenAI Provider

The OpenAI provider is disabled by default. To use it later:

```bash
export BUILDPLANE_AI_PROVIDER=openai
export BUILDPLANE_AI_MODEL=gpt-5.6
export OPENAI_API_KEY='<your-key>'
```

Do not put credentials in Git, Dockerfiles, or Kubernetes manifests.

