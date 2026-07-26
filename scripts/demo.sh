#!/usr/bin/env bash
set -euo pipefail

API="${BUILDPLANE_API_URL:-http://localhost:8080}"

echo "Waiting for BuildPlane API at ${API}"
for _ in {1..60}; do
  if curl -fsS "${API}/healthz" >/dev/null; then
    break
  fi
  sleep 2
done

echo "Creating sample workflow"
workflow_json="$(go run ./cmd/bpctl workflow create -f examples/workflows/go-fixture.yaml)"
echo "${workflow_json}"
workflow_id="$(printf '%s' "${workflow_json}" | sed -n 's/.*"ID":"\([^"]*\)".*/\1/p')"
if [[ -z "${workflow_id}" ]]; then
  echo "could not parse workflow id" >&2
  exit 1
fi

echo "Submitting idempotent run"
run_json="$(go run ./cmd/bpctl run create --workflow "${workflow_id}" --repo fixture://demo-repository --commit main --priority high --idempotency-key demo-run)"
echo "${run_json}"
run_id="$(printf '%s' "${run_json}" | sed -n 's/.*"ID":"\([^"]*\)".*/\1/p' | head -1)"
if [[ -z "${run_id}" ]]; then
  echo "could not parse run id" >&2
  exit 1
fi

echo "Submitting duplicate idempotency key"
go run ./cmd/bpctl run create --workflow "${workflow_id}" --repo fixture://demo-repository --commit main --priority high --idempotency-key demo-run

echo "Watching run ${run_id}"
for _ in {1..90}; do
  state="$(go run ./cmd/bpctl run get "${run_id}")"
  echo "${state}"
  if printf '%s' "${state}" | grep -q '"Status":"SUCCEEDED"'; then
    job_id="$(printf '%s' "${state}" | sed -n 's/.*"ID":"\([^"]*\)","WorkflowRunID".*/\1/p' | head -1)"
    echo "Run succeeded. Logs:"
    go run ./cmd/bpctl job logs "${job_id}"
    echo "Demo complete"
    exit 0
  fi
  if printf '%s' "${state}" | grep -q '"Status":"FAILED"'; then
    echo "Run failed" >&2
    exit 1
  fi
  sleep 2
done

echo "timed out waiting for run" >&2
exit 1
