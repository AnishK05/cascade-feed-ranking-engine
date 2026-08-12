# Cascade — Real-Time Feed & Ranking System

Consumer serving infrastructure for a personalized content feed (think: the backend behind a
home feed / timeline), built as an undergrad systems-design learning project. It focuses on the
serving path — fanout-on-write vs fanout-on-read, distributed caching, ranking, and
latency/throughput under simulated load — not on building a full social app.

**Start here:** [`IMPLEMENTATION_PLAN.md`](./IMPLEMENTATION_PLAN.md) is the detailed design doc
and phase-by-phase roadmap for this project. Everything below is a quick-start; the plan has the
full architecture, rationale, and a decisions log for every non-obvious design choice.

## Status

Phases 0-7 are complete: repository/schema bootstrap, Social Graph and Post services, Kafka
KRaft, hybrid fanout, the Feed Service read path, and configurable heuristic ranking. Phase 8
(cache invalidation/warming hardening) is next.

## Repository layout

```
proto/                 Shared .proto contracts (Post, Feed services)
services/
  post-service/         Go, gRPC — owns posts, publishes Kafka events
  feed-service/          Go, gRPC — serves ranked, paginated timelines
  fanout-worker/         Go — Kafka consumer, fans posts out to Redis timelines
  social-graph-service/  Java Spring Boot — owns users/follows, REST
gateway/                Java Spring Boot — API Gateway / BFF for the frontend
ranking/                Python — offline ranking-model training (stretch goal, deprioritized)
loadtest/               Python — dataset seeding + Locust load tests
frontend/               Next.js + TypeScript — demo UI and admin metrics dashboard
migrations/             SQL migrations (applied with golang-migrate)
deploy/                 Docker Compose + Kubernetes manifests
docs/                   Architecture notes, benchmark write-ups, ADRs
```

## Prerequisites

- Go 1.25+ (a `go.work` ties the Go modules together — see below)
- Java 21+ and the checked-in Maven Wrapper (`./mvnw`, no local Maven install required)
- `protoc` (the Protocol Buffers compiler) on your `PATH`, plus the Go plugins:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```
- Node 22+ (for the frontend)
- Python 3.12+ (for `loadtest/`)
- PostgreSQL (for running migrations; Docker Compose support for this lands in a later phase)
- The [`migrate` CLI](https://github.com/golang-migrate/migrate) for running migrations:
  ```bash
  go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  ```

## Quickstart

```bash
make help          # list all available targets
make proto          # generate Go + Java stubs from proto/*.proto (do this before building Go)
make build          # generate stubs, then build every Go/Java service
make test           # run the full test suite (Go, Java, Python) — mirrors CI
make migrate-up      # apply all SQL migrations to $DATABASE_URL
make up              # start PostgreSQL, Redis, and Kafka (KRaft; no ZooKeeper)
make kafka-topics    # create/verify all application and DLQ topics
make kafka-smoke     # create a temporary topic and prove produce -> consume
```

`DATABASE_URL` defaults to `postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable`;
override it by exporting `DATABASE_URL` or passing it inline, e.g.
`make migrate-up DATABASE_URL=postgres://...`.

## Implemented service APIs

### Social Graph Service (`services/social-graph-service`, port `8081`)

```text
POST   /users
GET    /users/{id}
POST   /follows
DELETE /follows/{followerId}/{followeeId}
GET    /users/{id}/followers?cursor=&limit=
GET    /users/{id}/following?cursor=&limit=
GET    /internal/celebrities
```

It uses `DATABASE_URL` as a JDBC URL (or `DB_HOST`/`DB_PORT`/`DB_NAME`), `DB_USER`,
`DB_PASSWORD`, `KAFKA_BOOTSTRAP_SERVERS`, `FOLLOW_EVENTS_TOPIC`, and
`CELEBRITY_THRESHOLD`. Follow/unfollow updates the denormalized follower count and celebrity
flag in the same transaction, then publishes `FollowCreated`/`FollowDeleted` JSON after commit.

### Post Service (`services/post-service`, gRPC port `9090`)

Implements `CreatePost`, batch `GetPosts`, and authorized soft-delete `DeletePost` from
`proto/post.proto`. It uses `DATABASE_URL`, `REDIS_ADDR`, `KAFKA_BROKERS`, `KAFKA_TOPIC`,
`POST_CACHE_TTL`, and `POST_SERVICE_GRPC_PORT`. Successful commits write/cache-invalidate Redis
and publish keyed `PostCreated`/`PostDeleted` JSON events; `GetPosts` uses a batched Redis
MGET/cache-aside path and one PostgreSQL query for misses.

### Fanout Worker (`services/fanout-worker`)

Consumes `post-events` and `follow-events` with manual offset commits. Normal-author posts are
pipelined into each follower's bounded `timeline:{userId}` Redis ZSET; celebrity posts are
written once to `celebrity_posts:global` for Phase 6's read-time merge. New normal follows
backfill recent posts, while celebrity follows maintain `following:celebrities:{userId}`.
Deletes add idempotent tombstones. Malformed events and retry-exhausted transient failures are
published to the corresponding DLQ before the original offset is committed.

Important Fanout Worker settings include `KAFKA_BROKERS`, `POST_EVENTS_TOPIC`,
`FOLLOW_EVENTS_TOPIC`, both `*_DLQ_TOPIC` variables, `KAFKA_CONSUMER_GROUP`, `DATABASE_URL`,
`REDIS_ADDR`, `CELEBRITY_FOLLOWER_THRESHOLD`, `MAX_TIMELINE_LEN`, `BACKFILL_COUNT`,
`FANOUT_BATCH_SIZE`, `MAX_RETRIES`, and `RETRY_BACKOFF`.

### Local Kafka backbone

`deploy/docker-compose.yml` pins real `apache/kafka:4.3.1` in one-node combined
broker/controller KRaft mode. Containers connect through `kafka:29092`; host-native services
connect through `localhost:9092`. Topic auto-creation is disabled so configuration mistakes do
not silently create misspelled topics; `kafka-init` explicitly creates the application topics.

### Feed Service (`services/feed-service`, gRPC port `9091`)

Implements `GetFeed` from `proto/feed.proto`. It merges bounded normal timeline candidates with
the global celebrity stream, filters celebrity posts to authors the viewer follows, removes
tombstones and duplicates, then hydrates content with one Redis MGET plus at most one batched
Post Service call. Missing cache values are warmed for subsequent reads.

Ranking runs in-process over the bounded candidate pool:

```text
score = recency_weight * exp(-age / half_life)
      + engagement_weight * log1p(likes + 2 * comments)
      + affinity_weight * viewer_author_affinity
```

Engagement and 30-day affinity signals come from two batched PostgreSQL queries rather than
N+1 lookups. Results use deterministic score/time/post-ID ordering and an opaque, validated
keyset cursor. Configure the service with `FEED_SERVICE_GRPC_PORT`, `DATABASE_URL`,
`REDIS_ADDR`, `POST_SERVICE_ADDR`, `FEED_CANDIDATE_POOL_SIZE`, page-size settings,
`FEED_POST_CACHE_TTL`, the three `FEED_*_WEIGHT` variables, `FEED_RECENCY_HALF_LIFE`,
`FEED_AFFINITY_WINDOW`, and `FEED_AFFINITY_DEFAULT`.

### A note on generated code

Generated protobuf/gRPC stubs (`proto/gen/go/**/*.pb.go`, and the Java sources under each Maven
module's `target/generated-sources/`) are **not** committed — they're build artifacts,
regenerated by `make proto` / the Maven build lifecycle. Run `make proto` once after cloning,
before building or testing the Go modules directly.

### Go workspace

The Go modules (`proto/gen/go`, `services/post-service`, `services/feed-service`,
`services/fanout-worker`) are tied together with a `go.work` file at the repo root. This lets
each service resolve the generated proto module (`proto/gen/go`) from the local filesystem
during development instead of needing a real published module. Some service modules also carry
an explicit local `replace` so they remain buildable when invoked directly. Build/test Go code
from the repo root using explicit paths (`go build ./services/...`), not a bare `./...`, since
the repo root itself isn't a Go module — or just use the `make` targets, which already do this
correctly.
