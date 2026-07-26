# BuildPlane Agent Instructions

BuildPlane is being built as a serious, incremental, open-source learning
project. The goal is to learn production backend engineering, Kubernetes, AI
workflow infrastructure, and operational UX while producing a credible system.

## Project Direction

BuildPlane is a Kubernetes-native control plane for composing, versioning,
executing, evaluating, and safely updating reusable AI workflow components
across enterprise operational workflows.

PostgreSQL is the durable source of truth. Redis may be used for dispatch and
short-lived coordination, but durable workflow state must live in PostgreSQL.

The LLM may interpret, classify, extract, and propose actions. Deterministic
application code must validate, authorize, and execute actions.

## Phase Rules

1. Build only one phase at a time.
2. Do not move to the next phase until the user explicitly says `CONTINUE`.
3. Start every phase by reading:
   - `AGENTS.md`
   - `docs/BUILD_ROADMAP.md`
   - `docs/BUILD_STATUS.md`
   - Relevant architecture decisions in `docs/adr/`
4. Explain concepts before implementing them.
5. Prefer simple, explicit designs over framework-heavy abstractions.
6. Preserve existing working behavior.
7. Never fabricate benchmarks, test results, users, scale, or reliability claims.
8. Never add resume metrics until they have been measured.
9. Do not commit changes unless the user explicitly requests a commit.
10. At the end of each phase, propose one conventional commit message.
11. Never expose real credentials or secret values.
12. Use synthetic demo data only.
13. Run relevant formatting, linting, unit tests, integration tests, and build
    commands before declaring a phase complete.
14. When tests cannot run, state exactly why.
15. Record important architectural decisions as ADRs.
16. Update `docs/BUILD_STATUS.md` at the end of every phase.
17. Do not silently add technologies. Explain why each dependency is required.
18. Avoid multi-agent architecture, custom workflow languages, or premature
    microservices until the simpler design is proven insufficient.
19. Treat PostgreSQL as authoritative workflow state.
20. Design for at-least-once delivery unless a narrower exactly-once boundary is
    explicitly proven and documented.

## Teaching Protocol

For every phase, use this structure before coding:

### A. Phase objective

Describe what is being added and what behavior will exist afterward.

### B. Concepts I need first

For every major concept, explain:

- What it is
- Why it exists
- How it works
- Where it appears in BuildPlane
- A common mistake
- How to inspect or debug it

### C. Design

Show:

- Component responsibilities
- Data flow
- State transitions
- Failure cases
- Security boundary
- Alternatives considered
- Why the selected design is appropriate now

### D. Planned file changes

List the files expected to be created or modified and what each one will do.

After coding, report:

### A. What changed
### B. Walkthrough
### C. Correctness
### D. Kubernetes explanation
### E. Commands
### F. Test evidence
### G. Learning artifact
### H. Manual exercise for me
### I. Phase checkpoint

Then stop until the user says `CONTINUE`.

## Engineering Principles

- Use explicit workflow and node state machines.
- Persist every state transition.
- Use optimistic concurrency or row locking where races are possible.
- Use a transactional outbox where database state and event publication must
  remain consistent.
- Make task completion idempotent.
- Assume workers can crash at any point.
- Assume acknowledgments and network responses can be lost.
- Worker leases must include worker identity, task identity, lease expiration,
  heartbeat, attempt number, and a fencing token or equivalent stale-worker
  protection.
- Backpressure must account for worker capacity, tenant limits, queue depth,
  oldest task age, priority, retry storms, and downstream rate limits.
- Use stable idempotency keys for workflow creation, node execution, human
  decisions, external tool actions, callback processing, and replay requests.
- Apply least privilege, avoid secret values in logs, and keep tenant boundaries
  explicit.
