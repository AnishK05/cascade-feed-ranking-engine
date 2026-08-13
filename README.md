# Cascade — Real-Time Feed & Ranking System

Consumer serving infrastructure for a personalized content feed (think: the backend behind a
home feed / timeline), built as an undergrad systems-design learning project. It focuses on the
serving path — fanout-on-write vs fanout-on-read, distributed caching, ranking, and
latency/throughput under simulated load — not on building a full social app.

**Start here:** [`IMPLEMENTATION_PLAN.md`](./IMPLEMENTATION_PLAN.md) is the detailed design doc
and phase-by-phase roadmap for this project. Everything below is a quick-start; the plan has the
full architecture, rationale, and a decisions log for every non-obvious design choice.

**Windows:** Docker Desktop + PowerShell or cmd. From the repo root:

```bat
cascade.cmd up
cascade.cmd smoke
```

Then open http://localhost:3000 . Full walkthrough and error table:
[`docs/running-on-windows.md`](./docs/running-on-windows.md). You do not need Make,
Git Bash, or an Ubuntu terminal to run the demo.

## Status

Phases 0–16 of the roadmap that this repo implements are complete: schema through Compose,
Locust benchmarking, a local `kind` cluster (`make kind-up`), ADRs, README/resume polish, and
the testing strategy (unit + Testcontainers integration + smoke + a CI graph-generation
budget). **Phase 9.5** (Fanout Worker REST boundary) is still deferred on purpose; see
[ADR 0004](./docs/decisions/0004-fanout-direct-postgres.md).

There is no cloud Kubernetes and no cloud cost.

## Resume line (measured, not a target)

Use only numbers you have actually produced. As of the committed write-up
([`docs/benchmarks/2026-08-13-cache-comparison.md`](./docs/benchmarks/2026-08-13-cache-comparison.md)):

> Built a Go/gRPC hybrid fanout feed (write-path ZSETs + read-path celebrity merge) with a
> Zipf seeder that materializes a 50,000-user / 50-celebrity graph (7.84M follow edges in
> 3.6s) and COPY-loads a 500-user celebrity-scaled dataset into Postgres in 262ms; Redis vs
> Postgres GetFeed cost is compared via `feed_postgres_queries_total` around Locust — fill in
> req/s and the reduction % after running the Phase 12 protocol on your machine.

Do not quote the plan's example "8,000 req/s" or "80% cache reduction" until your Locust
CSV and Prometheus snapshots contain those values.

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
- Node 22+ (for the frontend; not required if you only run the Compose image)
- Python 3.12+ (for `loadtest/` and `make smoke` / `cascade.cmd smoke`)
- Docker with Compose v2 (`make up` or `cascade.cmd up` starts the entire stack)
- **Windows:** Docker Desktop + Python is enough to demo; use `cascade.cmd` (see
  [`docs/running-on-windows.md`](./docs/running-on-windows.md)). Go/Java/Make are optional.
- `kind` and `kubectl` if you want the Phase 14 local Kubernetes path (`make kind-up`)
- The [`migrate` CLI](https://github.com/golang-migrate/migrate) for running migrations against
  a host-native Postgres (Compose applies the up SQL files on first boot):
  ```bash
  go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  ```

## Quickstart

**Windows (PowerShell or cmd):** [`docs/running-on-windows.md`](./docs/running-on-windows.md)

```bat
cascade.cmd up
cascade.cmd smoke
```

**Linux / macOS:**

```bash
make help          # list all available targets
make proto          # generate Go + Java stubs from proto/*.proto (do this before building Go)
make build          # generate stubs, then build every Go/Java service
make test           # run the full test suite (Go, Java, Python) — mirrors CI
make migrate-up      # apply all SQL migrations to $DATABASE_URL
make up              # build and start the full stack (Postgres, Redis, Kafka, all services, UI)
make smoke           # create users → follow → create post → wait until the follower feed shows it
make kafka-topics    # create/verify all application and DLQ topics
make kafka-smoke     # create a temporary topic and prove produce -> consume
make seed            # COPY a power-law follow graph into Postgres (`PRESET=full` for 50k users)
make warm-cache      # rebuild Redis timelines from Postgres after a cold start
make kind-up         # local kind cluster (loads Compose images; Gateway on localhost:8080)
make k8s-smoke       # same smoke test against kind
make k8s-validate    # kubeconform/kustomize check (no cluster)
```

`DATABASE_URL` defaults to `postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable`;
override it by exporting `DATABASE_URL` or passing it inline, e.g.
`make migrate-up DATABASE_URL=postgres://...`.

## Implemented service APIs

### Social Graph Service (`services/social-graph-service`, port `8081`)

```text
POST   /users
GET    /users/{id}
GET    /users?ids=1,2,3
GET    /users?limit=                       omit `ids` to list users (max 100)
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
`FEED_AFFINITY_WINDOW`, `FEED_AFFINITY_DEFAULT`, and `FEED_METRICS_ADDR` (Prometheus scrape
port, default `:9101`). Post Service scrapes at `POST_METRICS_ADDR` (`:9100`); Fanout Worker
at `FANOUT_METRICS_ADDR` (`:9102`).

### Cache invalidation and warming (Phase 8)

Deletes do not walk every follower ZSET. Post Service immediately drops `post:{id}` and adds
the ID to a global Redis `tombstones` set (TTL 24h by default, refreshed on each delete). Feed
Service filters that set on both candidate load and hydration, so the next `GetFeed` after a
delete omits the post even if it is still sitting in a timeline ZSET. Fanout Worker also writes
the same tombstone when it consumes `PostDeleted`, which covers the case where the original
cache side effect was lost.

New follows of normal accounts backfill recent posts into `timeline:{followerId}` immediately
(Fanout Worker). After a Redis restart, rebuild those keys from Postgres:

```bash
make warm-cache
```

`TOMBSTONE_TTL` / `POST_TOMBSTONE_TTL` (default `24h`) should outlast typical
`MAX_TIMELINE_LEN` turnover. Soft-deleted Postgres rows remain the fallback once a tombstone
expires.

### API Gateway / BFF (`gateway/`, port `8080`)

The frontend talks only to the Gateway. It translates HTTP/JSON into gRPC calls to Post/Feed
Service and REST calls to Social Graph Service. There is no real authentication: send
`X-User-Id` as the simulated viewer. That header is trivially spoofable by design.

```text
GET    /api/feed?pageToken=&pageSize=     X-User-Id required; hydrates author display names
POST   /api/posts                         X-User-Id is the author; response is prependable
DELETE /api/posts/{id}                    X-User-Id must be the author
GET    /api/posts?ids=1,2,3
POST   /api/users                         no auth (creates a simulated identity)
GET    /api/users?limit=                  user switcher directory
GET    /api/users/{id}
POST   /api/follows                       X-User-Id is the follower; body `{ "followeeId": N }`
DELETE /api/follows/{followeeId}          X-User-Id is the follower
GET    /api/users/{id}/followers
GET    /api/users/{id}/following
GET    /api/admin/metrics                 Prometheus snapshot for the admin dashboard
GET    /api/ping
```

Configure with `GATEWAY_PORT`, `POST_SERVICE_ADDR`, `FEED_SERVICE_ADDR`,
`SOCIAL_GRAPH_BASE_URL`, `CORS_ALLOWED_ORIGINS`, `GATEWAY_GRPC_DEADLINE`, and
`PROMETHEUS_URL` (default `http://localhost:9095`). Create-post responses include `postId` /
`authorId` / `createdAtUnixMs` so the author's own client can optimistically prepend the new
post while follower feeds catch up through Kafka. Feed items include the heuristic score
breakdown (`recencyScore`, `engagementScore`, `affinityScore`) for the debug toggle.

### Demo frontend (`frontend/`, port `3000`)

Next.js App Router UI talks only to the Gateway. Pick a seeded user in the top-bar switcher
(`X-User-Id`), compose a post on `/feed`, follow people on `/graph`, and watch cache hit
ratio / feed latency / Kafka lag on `/admin`. Point `NEXT_PUBLIC_API_BASE` at the Gateway
(default `http://localhost:8080`).

```bash
cd frontend && npm install && npm run dev
```

### Observability (Prometheus `9095`, Grafana `3001`)

Every service exposes Prometheus metrics. Go services bind `/metrics` on 9100/9101/9102; Java
services expose `/actuator/prometheus` on their HTTP ports. The Gateway stamps `X-Request-Id`
(or generates one) and forwards it as gRPC metadata `x-request-id` and as an HTTP header to
Social Graph, so one ID greps across JSON logs. Compose Prometheus scrapes the **container**
names (`feed-service:9101`, `gateway:8080`, …). Grafana on 3001 is provisioned with the
Cascade Feed dashboard (anonymous viewer, or `admin`/`admin`).

### Full Docker Compose (Phase 13)

`make up` builds and starts Postgres, Redis, Kafka (KRaft), Post/Feed/Fanout, Social Graph,
Gateway, the Next.js UI, Prometheus, and Grafana. Browser calls still use
`http://localhost:8080` (`NEXT_PUBLIC_API_BASE` is baked into the frontend image at build
time). After the stack is healthy:

```bash
make smoke           # create-post → Kafka fanout → follower GetFeed
make seed            # default `--preset ci` (500 users); `make seed PRESET=full` for 50k
make warm-cache-compose
```

`docker compose -f deploy/docker-compose.yml up -d --wait postgres redis kafka` still starts
only the data plane (CI Kafka smoke does this and must not build app images).

### Load testing (Phase 12)

`loadtest/seed.py` COPY-inserts a Zipf follow graph. `loadtest/locustfile.py` hits Gateway
`GET /api/feed` vs `POST /api/posts` at **100:1**. The cache comparison is:

1. Seed + `make warm-cache`.
2. Baseline: `FEED_BYPASS_CACHE=true` (Feed reads candidates and post bodies from Postgres).
3. Cached: `FEED_BYPASS_CACHE=false` after warming Redis.
4. Reduction = `1 − (Postgres QPS with cache ÷ Postgres QPS baseline)` on
   `feed_postgres_queries_total{op="candidates|hydrate"}` (and `post_postgres_queries_total`
   when GetPosts is on the path).

Protocol, machine specs, and measured numbers:
[`docs/benchmarks/2026-08-13-cache-comparison.md`](./docs/benchmarks/2026-08-13-cache-comparison.md).

```bash
make loadtest USERS=50 DURATION=30s
# or the scrape-around-Locust helper:
make benchmark LABEL=baseline
```

Set `CELEBRITY_FOLLOWER_THRESHOLD` / `CELEBRITY_THRESHOLD` to the value printed by `seed.py`
(80 for `--preset ci`, 10000 for `--preset full`) so live fanout agrees with the seeded
`is_celebrity` flags.

### Local Kubernetes (Phase 14)

`deploy/k8s/` is a kustomize bundle for a **local kind cluster only**. It is not wired to any
cloud provider.

```bash
make kind-up       # docker compose build + kind load + apply + wait
make k8s-smoke     # GATEWAY_URL=http://localhost:8080 make smoke
make k8s-hpa       # Feed Service HPA 1–4 replicas on CPU
make k8s-chaos     # delete a feed-service pod; Gateway /api/ping must recover
make kind-down
```

Credentials are a Secret; ranking weights and the celebrity threshold are a ConfigMap. App
images use `imagePullPolicy: Never`. Details: [`deploy/k8s/README.md`](./deploy/k8s/README.md)
and [ADR 0007](./docs/decisions/0007-local-kind-only.md).

### Testing strategy (Phase 16)

- **Go:** table-driven unit tests (ranking, fanout decision, pagination, cache bypass). Feed
  Service and Fanout Worker integration tests start Postgres/Redis (and Kafka for fanout)
  with **testcontainers-go** when Docker is running, and skip otherwise. Post Service still
  has env-gated Compose tests. `make go-cover` prints coverage.
- **Java:** `@WebMvcTest` for Gateway controllers; Social Graph uses Testcontainers Postgres.
- **Smoke:** `scripts/smoke_test.py` is the cross-service regression net (Compose or kind).
- **Load as a test:** CI asserts the `--preset ci` graph builds in under a second and that
  ranking 500 posts stays under 50ms. Full Locust against a live Gateway is `make loadtest`,
  not a default PR check.

### Security (deliberately minimal)

`X-User-Id` is **trivially spoofable**. That is the auth model. Do not put this stack on a
public address. Demo credentials (`cascade`/`cascade`) live in `.env.example` and a
Kubernetes Secret named `cascade-db`; do not commit real secrets. Input length limits on
posts exist so Locust garbage does not crash handlers, not as a security boundary.

### Architecture decision records

[`docs/decisions/`](./docs/decisions/) — Kafka vs Redpanda, hybrid fanout, ZSETs, the deferred
Fanout REST refactor, the auth stub, heuristic ranking, local-only kind, JSON events.

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
