# Caching

Cache keys are SHA-256 hashes of normalized inputs: tenant, user prefix, key file contents, commands, runner image version, OS, architecture, and selected environment variables.

Objects use:

```text
cache/sha256/<first-two-characters>/<full-hash>.tar.zst
```

The MVP includes safe archive extraction that rejects path traversal and unsupported entry types. Checksum verification protects against corrupted downloads. Cache misses never fail a job.

Production cache isolation should include tenant namespace separation, trust-boundary separation, signed metadata, stricter key inputs, and lifecycle policies. Cache poisoning remains possible if untrusted jobs can write cache entries consumed by more trusted jobs.
