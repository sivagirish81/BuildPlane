package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServiceName          string
	HTTPAddr             string
	DatabaseURL          string
	RedisAddr            string
	RedisPassword        string
	KubeNamespace        string
	RunnerImage          string
	RunnerServiceAccount string
	InternalToken        string
	PublicBaseURL        string
	LeaseDuration        time.Duration
	HeartbeatInterval    time.Duration
	ReconcileInterval    time.Duration
	SchedulerInterval    time.Duration
	GlobalActiveLimit    int
	PerTenantQueueLimit  int
	GlobalQueueLimit     int
	MaxCPUMillisInFlight int
	MaxMemoryMBInFlight  int
	S3Endpoint           string
	S3AccessKey          string
	S3SecretKey          string
	S3Bucket             string
	S3UseSSL             bool
	ShadowScheduler      bool
}

func Load(service string) Config {
	return Config{
		ServiceName:          service,
		HTTPAddr:             env("HTTP_ADDR", ":8080"),
		DatabaseURL:          env("DATABASE_URL", "postgres://buildplane:buildplane@localhost:5432/buildplane?sslmode=disable"),
		RedisAddr:            env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        env("REDIS_PASSWORD", ""),
		KubeNamespace:        env("KUBE_NAMESPACE", "buildplane"),
		RunnerImage:          env("RUNNER_IMAGE", "buildplane-runner:local"),
		RunnerServiceAccount: env("RUNNER_SERVICE_ACCOUNT", "buildplane-runner"),
		InternalToken:        env("INTERNAL_TOKEN", "dev-internal-token"),
		PublicBaseURL:        env("PUBLIC_BASE_URL", "http://api:8080"),
		LeaseDuration:        envDuration("LEASE_DURATION", 30*time.Second),
		HeartbeatInterval:    envDuration("HEARTBEAT_INTERVAL", 10*time.Second),
		ReconcileInterval:    envDuration("RECONCILE_INTERVAL", 10*time.Second),
		SchedulerInterval:    envDuration("SCHEDULER_INTERVAL", 2*time.Second),
		GlobalActiveLimit:    envInt("GLOBAL_ACTIVE_LIMIT", 10),
		PerTenantQueueLimit:  envInt("PER_TENANT_QUEUE_LIMIT", 100),
		GlobalQueueLimit:     envInt("GLOBAL_QUEUE_LIMIT", 1000),
		MaxCPUMillisInFlight: envInt("MAX_CPU_MILLIS_IN_FLIGHT", 16000),
		MaxMemoryMBInFlight:  envInt("MAX_MEMORY_MB_IN_FLIGHT", 32768),
		S3Endpoint:           env("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey:          env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:          env("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:             env("S3_BUCKET", "buildplane-cache"),
		S3UseSSL:             envBool("S3_USE_SSL", false),
		ShadowScheduler:      envBool("SHADOW_SCHEDULER", true),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
