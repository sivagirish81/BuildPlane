# ADR 0004: At-Least-Once Delivery

BuildPlane claims at-least-once execution. Duplicate queue delivery and duplicate callbacks are expected and handled with idempotent transitions. Exactly-once execution is not claimed.
