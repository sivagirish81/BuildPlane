# Phase 10 Learning Artifact: Evaluation, Canary, and Promotion

## What You Built

Phase 10 adds a first release-safety loop for the reusable
`issue_classifier` component.

The lifecycle is:

```text
candidate -> evaluated -> canary -> promoted
```

Rollback restores the previous promoted version:

```text
promoted v2 -> rolled_back v2
superseded v1 -> promoted v1
```

## Why This Matters

AI components behave like code and configuration at the same time. A prompt or
classification rule change can affect every workflow that depends on it.
BuildPlane now records the candidate, checks affected workflows, runs synthetic
evaluation cases, stores the comparison, and gates canary/promotion.

## Manual Exercise

Create a candidate:

```bash
curl -i -X POST http://localhost:8080/v1/component-versions \
  -H 'Content-Type: application/json' \
  --data @examples/component-versions/issue-classifier-v2.json
```

Check affected workflows:

```bash
curl -i http://localhost:8080/v1/components/issue_classifier/affected-workflows
```

Run evaluation:

```bash
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/evaluations
```

Start a 10 percent canary:

```bash
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/canary \
  -H 'Content-Type: application/json' \
  -d '{"percent":10}'
```

Promote:

```bash
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/promote
```

Rollback:

```bash
curl -i -X POST http://localhost:8080/v1/components/issue_classifier/rollback \
  -H 'Content-Type: application/json' \
  -d '{"actor_id":"operator-1","reason":"Synthetic rollback exercise"}'
```

## Database Queries

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select component_name, version, status, canary_percent, evaluation_passed, previous_promoted_version_id from component_versions order by created_at;'

docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select component_name, dataset_name, status, candidate_passed_cases, candidate_total_cases from component_evaluation_runs order by created_at;'

docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select component_name, event_type, actor_id, details, created_at from component_release_events order by created_at;'
```

## Debugging Model

- If canary fails, check whether the latest evaluation passed.
- If promotion fails, check whether the component version is in `canary`.
- If rollback fails, check whether the promoted version has
  `previous_promoted_version_id`.
- If affected workflows look wrong, inspect `ComponentDependenciesFor` in the
  workflow definitions.

## What This Phase Does Not Claim

The canary percentage is durable desired release state. It does not yet route
live workflow runs by percentage or prove production canary health. That comes
later when online routing and measured metrics exist.
