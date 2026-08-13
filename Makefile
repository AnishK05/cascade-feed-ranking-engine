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
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-18s %s\n", $$1, $$2}'

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
# Local orchestration (filled in further starting Phase 4/12/13)
# ---------------------------------------------------------------------------

.PHONY: up
up: ## Start local infrastructure (Postgres, Redis, Kafka, Prometheus, Grafana).
	docker compose -f deploy/docker-compose.yml up -d

.PHONY: down
down: ## Stop local infrastructure started via `make up`.
	docker compose -f deploy/docker-compose.yml down

.PHONY: kafka-topics
kafka-topics: ## Create/verify the Kafka topics used by Cascade.
	docker compose -f deploy/docker-compose.yml run --rm kafka-init

.PHONY: kafka-smoke
kafka-smoke: ## Prove a message can be produced and consumed through Compose Kafka.
	./scripts/kafka-smoke.sh

.PHONY: warm-cache
warm-cache: ## Rebuild Redis timelines from Postgres (cold-start cache warming, Phase 8).
	go run ./services/fanout-worker/cmd/warm-cache
