# ADR 0010: Component Evaluation, Canary, and Promotion

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane now has reusable workflow components, including the AI-backed
`classify_issue` node used by the local, invoice, and freight workflows.
Changing that component should not mean editing a prompt in place and hoping
the workflows still behave correctly.

The roadmap calls for component version graphs, affected workflow detection,
evaluation datasets, candidate comparison, canary rollout, promotion, and
rollback.

## Decision

Add a first release-safety model for `issue_classifier`.

PostgreSQL stores immutable component versions in `component_versions`. The
first seeded baseline is `issue_classifier@v1` with status `promoted`.

Candidate versions include:

- component name
- semantic version label
- prompt version label
- deterministic synthetic classification spec
- status
- evaluation result gate
- canary percentage
- previous promoted version pointer

BuildPlane adds a deterministic synthetic evaluation dataset named
`synthetic.issue-classifier.v1`. Evaluation compares the candidate version
against the currently promoted baseline and stores both summary and per-case
results.

Release state transitions are:

```text
candidate -> evaluated -> canary -> promoted
```

A failing evaluated candidate cannot enter canary. A version cannot be promoted
until it is in canary and has passed evaluation. Promotion supersedes the
previous promoted version and records a pointer for rollback. Rollback restores
that previous promoted version and marks the current promoted version
`rolled_back`.

Affected workflow detection remains application-defined for now. The known
affected workflows for `issue_classifier` are:

- `phase4.local-demo`
- `demo.invoice-exception`
- `demo.freight-exception`

## Consequences

- BuildPlane can now demonstrate eval-driven AI component releases.
- Release decisions are durable and inspectable in PostgreSQL.
- Candidate comparison is deterministic and does not require live model calls.
- Canary state is recorded, but this phase does not yet route live traffic by
  percentage.
- Evaluation quality is limited by the small synthetic dataset.
- Component definitions are still app-defined rather than stored in a general
  registry.

## Alternatives Considered

Route live workflow traffic by canary percentage immediately:

- Deferred. The first phase should teach the release gates before adding live
  traffic routing and online metrics.

Use production workflow outcomes as evaluation data:

- Rejected for now. The project uses synthetic demo data only at this stage.

Adopt a full feature flag system:

- Deferred. A durable in-repo release model is enough for the first learning
  slice and avoids introducing a new operational dependency.
