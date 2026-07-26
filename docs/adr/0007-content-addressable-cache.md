# ADR 0007: S3-Compatible Content-Addressable Cache

BuildPlane stores cache archives in S3-compatible object storage with SHA-256-addressed object keys and PostgreSQL metadata. This keeps cache storage portable across MinIO and cloud S3 APIs.
