# ADR 0005: Lease-Based Attempt Ownership

Attempts receive short-lived lease tokens. Only token hashes are stored. Runner callbacks must prove ownership and freshness, preventing expired workers from overwriting newer attempts.
