\
# Cascade — Real-Time Feed & Ranking System
#
# This Makefile wraps the common developer commands described in IMPLEMENTATION_PLAN.md.
# Run `make help` to list all targets.

SHELL := /bin/bash
.DEFAULT_GOAL := help

GO_MODULES := services/post-service services/feed-service services/fanout-worker proto/gen/go
JAVA_MODULES := gateway services/social-graph-service

PROTO_GO_MODULE := github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go

DATABASE_URL ?= postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable
MIGRATIONS_DIR := migrations

.PHONY: help
help: ## Show this help.
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-22s %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Codegen
# ---------------------------------------------------------------------------

.PHONY: proto
proto: proto-go proto-java ## Generate Go and Java stubs from proto/*.proto (Phase 0).

.PHONY: proto-go
proto-go: ## Generate Go protobuf + gRPC stubs into proto/gen/go.
	protoc \
		--go_out=proto/gen/go --go_opt=module=$(PROTO_GO_MODULE) \
		--go-grpc_out=proto/gen/go --go-grpc_opt=module=$(PROTO_GO_MODULE) \
		-I proto proto/post.proto proto/feed.proto

.PHONY: proto-java
proto-java: ## Generate Java protobuf + gRPC stubs for the gateway (via protobuf-maven-plugin).
	cd gateway && ./mvnw -q generate-sources

# ---------------------------------------------------------------------------
# Build / test / lint — Go
# ---------------------------------------------------------------------------
# NOTE: these targets assume `make proto-go` has already been run at least once; generated
# *.pb.go files are gitignored build artifacts, not checked into version control.

GO_PACKAGES := $(addprefix ./,$(addsuffix /...,$(GO_MODULES)))

.PHONY: go-build
go-build: ## Build all Go services.
	go build $(GO_PACKAGES)

.PHONY: go-test
go-test: ## Run all Go unit tests.
	go test $(GO_PACKAGES)

.PHONY: go-cover
go-cover: ## Run Go tests with coverage summaries (Phase 16).
	go test -cover $(GO_PACKAGES)

.PHONY: go-vet
go-vet: ## Run go vet across all Go services.
	go vet $(GO_PACKAGES)

.PHONY: go-fmt-check
go-fmt-check: ## Fail if any Go file is not gofmt-formatted.
	@unformatted=$$(gofmt -l services proto/gen/go); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

# ---------------------------------------------------------------------------
# Build / test — Java
# ---------------------------------------------------------------------------

.PHONY: java-test
java-test: ## Run tests for both Java services (gateway, social-graph-service).
	@for m in $(JAVA_MODULES); do \
		echo "==> $$m"; \
		(cd $$m && ./mvnw -q test) || exit 1; \
	done

.PHONY: java-build
java-build: ## Package both Java services.
	@for m in $(JAVA_MODULES); do \
		echo "==> $$m"; \
		(cd $$m && ./mvnw -q package) || exit 1; \
	done

# ---------------------------------------------------------------------------
# Build / test — Python
# ---------------------------------------------------------------------------

.PHONY: loadtest-test
loadtest-test: ## Run loadtest/'s pytest suite (creates a local venv on first run).
	cd loadtest && \
	( [ -d .venv ] || python3 -m venv .venv ) && \
	source .venv/bin/activate && \
	pip install -q -r requirements-dev.txt && \
	pytest -q

# ---------------------------------------------------------------------------
# Build / test — Frontend
# ---------------------------------------------------------------------------

.PHONY: frontend-lint
frontend-lint: ## Lint the Next.js frontend.
	cd frontend && npm ci --no-audit --no-fund && npm run lint

.PHONY: frontend-build
frontend-build: ## Production-build the Next.js frontend.
	cd frontend && npm ci --no-audit --no-fund && npm run build

# ---------------------------------------------------------------------------
# Aggregate targets
# ---------------------------------------------------------------------------

.PHONY: build
build: proto go-build java-build ## Generate stubs and build every service.

.PHONY: test
test: proto go-vet go-fmt-check go-test java-test loadtest-test ## Run the full test suite (mirrors CI).

# ---------------------------------------------------------------------------
# Database migrations (Phase 1) — requires the `migrate` CLI:
#   go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
# ---------------------------------------------------------------------------

.PHONY: migrate-up
migrate-up: ## Apply all up migrations to $$DATABASE_URL.
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back all migrations on $$DATABASE_URL.
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down -all

.PHONY: migrate-create
migrate-create: ## Create a new migration pair, e.g. `make migrate-create name=add_widgets`.
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

# ---------------------------------------------------------------------------
# Local orchestration
# ---------------------------------------------------------------------------

.PHONY: up
up: ## Build and start the full local stack (Phase 13).
	docker compose -f deploy/docker-compose.yml up -d --build

.PHONY: down
down: ## Stop local infrastructure started via `make up`.
	docker compose -f deploy/docker-compose.yml down

.PHONY: kafka-topics
kafka-topics: ## Create/verify the Kafka topics used by Cascade.
	docker compose -f deploy/docker-compose.yml run --rm kafka-init

.PHONY: kafka-smoke
kafka-smoke: ## Prove a message can be produced and consumed through Compose Kafka.
	./scripts/kafka-smoke.sh

.PHONY: smoke
smoke: ## Create users → follow → create post → wait for fanout → follower GetFeed.
	python3 scripts/smoke_test.py

.PHONY: warm-cache
warm-cache: ## Rebuild Redis timelines from Postgres (cold-start cache warming, Phase 8).
	go run ./services/fanout-worker/cmd/warm-cache

.PHONY: warm-cache-compose
warm-cache-compose: ## Run cache warming inside Compose (profile: tools).
	docker compose -f deploy/docker-compose.yml --profile tools run --rm warm-cache

.PHONY: seed
seed: ## Seed the ci follow-graph into Postgres (`PRESET=full` for 50k users).
	cd loadtest && \
	( [ -d .venv ] || python3 -m venv .venv ) && \
	source .venv/bin/activate && \
	pip install -q -r requirements.txt && \
	python seed.py --preset $(or $(PRESET),ci)

.PHONY: loadtest
loadtest: ## Headless Locust against the Gateway (override USERS/DURATION/HOST).
	cd loadtest && \
	( [ -d .venv ] || python3 -m venv .venv ) && \
	source .venv/bin/activate && \
	pip install -q -r requirements.txt && \
	python -m locust -f locustfile.py --headless \
		--host $(or $(HOST),http://localhost:8080) \
		-u $(or $(USERS),50) -r $(or $(SPAWN_RATE),10) \
		-t $(or $(DURATION),30s) \
		--csv reports/manual --html reports/manual.html --only-summary

.PHONY: benchmark
benchmark: ## Run one labeled Locust+metrics capture (`LABEL=baseline` or `cached`).
	cd loadtest && \
	( [ -d .venv ] || python3 -m venv .venv ) && \
	source .venv/bin/activate && \
	pip install -q -r requirements.txt && \
	python benchmark.py run --label $(or $(LABEL),cached) \
		--host $(or $(HOST),http://localhost:8080) \
		--users $(or $(USERS),50) --spawn-rate $(or $(SPAWN_RATE),10) \
		--duration $(or $(DURATION),30s)

# ---------------------------------------------------------------------------
# Kubernetes (Phase 14) — local kind only
# ---------------------------------------------------------------------------

.PHONY: k8s-validate
k8s-validate: ## Validate kind manifests (no cluster required).
	./scripts/validate_k8s.sh

.PHONY: kind-up
kind-up: ## Create a local kind cluster, load images, and apply deploy/k8s.
	./scripts/kind-up.sh

.PHONY: kind-down
kind-down: ## Delete the local kind cluster.
	./scripts/kind-down.sh

.PHONY: k8s-smoke
k8s-smoke: ## Run the Compose smoke test against kind's Gateway NodePort.
	GATEWAY_URL=http://localhost:8080 python3 scripts/smoke_test.py

.PHONY: k8s-hpa
k8s-hpa: ## Show Feed Service HPA/deployment status.
	./scripts/kind-hpa.sh

.PHONY: k8s-chaos
k8s-chaos: ## Delete a Feed pod and wait for Gateway /api/ping to recover.
	./scripts/kind-chaos.sh

.PHONY: k8s-warm-cache
k8s-warm-cache: ## Run the warm-cache Job inside kind.
	kubectl apply -f deploy/k8s/warm-cache-job.yaml
	kubectl -n cascade wait --for=condition=complete --timeout=180s job/warm-cache
