# ADR 0002: Redis as Derived Scheduling Queue

Redis is used for fast ready-job discovery by priority and tenant. It is not authoritative. Queue reconciliation rebuilds missing entries from PostgreSQL.
