SHELL := /bin/bash
IMAGE ?= buildplane:local
RUNNER_IMAGE ?= buildplane-runner:local
KIND_CLUSTER ?= buildplane
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GO := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go

.PHONY: bootstrap kind-up infra-up build images deploy migrate test integration-test e2e-test demo dashboards logs clean

bootstrap:
	@command -v go >/dev/null || (echo "go is required" && exit 1)
	@command -v docker >/dev/null || (echo "docker is required" && exit 1)
	@command -v kubectl >/dev/null || (echo "kubectl is required" && exit 1)
	@command -v kind >/dev/null || (echo "kind is required" && exit 1)
	$(GO) mod download

kind-up:
	kind get clusters | grep -qx "$(KIND_CLUSTER)" || kind create cluster --name "$(KIND_CLUSTER)" --config deploy/kind/kind.yaml
	kubectl config use-context kind-$(KIND_CLUSTER)

infra-up:
	docker compose up -d postgres redis minio prometheus grafana

build:
	$(GO) build ./cmd/api ./cmd/scheduler ./cmd/runner ./cmd/autoscaler ./cmd/bpctl ./cmd/migrate

images:
	docker build --target service -t $(IMAGE) .
	docker build --target runner -t $(RUNNER_IMAGE) .
	kind load docker-image --name "$(KIND_CLUSTER)" $(IMAGE)
	kind load docker-image --name "$(KIND_CLUSTER)" $(RUNNER_IMAGE)

deploy:
	kubectl apply -f deploy/kubernetes/namespace.yaml
	kubectl apply -f deploy/kubernetes/rbac.yaml
	kubectl apply -f deploy/kubernetes/config.yaml
	kubectl apply -f deploy/kubernetes/services.yaml
	kubectl apply -f deploy/kubernetes/deployments.yaml
	kubectl -n buildplane rollout status deploy/api --timeout=120s
	kubectl -n buildplane rollout status deploy/scheduler --timeout=120s

migrate:
	$(GO) run ./cmd/migrate

test:
	$(GO) test ./...
	$(GO) vet ./...

integration-test:
	$(GO) test ./tests/integration -count=1

e2e-test:
	./scripts/demo.sh

demo: bootstrap kind-up infra-up images deploy
	./scripts/demo.sh

dashboards:
	@echo "Prometheus: http://localhost:9090"
	@echo "Grafana:    http://localhost:3000"
	@echo "MinIO:      http://localhost:9001"

logs:
	kubectl -n buildplane logs deploy/api --tail=200
	kubectl -n buildplane logs deploy/scheduler --tail=200

clean:
	kubectl delete namespace buildplane --ignore-not-found
	docker compose down
