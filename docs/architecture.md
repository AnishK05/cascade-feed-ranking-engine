# Architecture

This document mirrors the diagrams and data model in
[`IMPLEMENTATION_PLAN.md`](../IMPLEMENTATION_PLAN.md) (§2, §4) so they're easy to find alongside
the rest of the design docs and ADRs. The plan is the source of truth if the two ever drift;
update both together.

## System architecture

```mermaid
flowchart LR
    subgraph Client
        FE[Next.js + TypeScript<br/>Frontend]
    end

    subgraph Gateway["API Gateway / BFF (Java Spring Boot)"]
        GW[REST API<br/>Auth stub, request aggregation]
    end

    subgraph Go Services
        PS[Post Service<br/>Go + gRPC]
        FS[Feed Service<br/>Go + gRPC]
        FW[Fanout Worker<br/>Go, Kafka consumer]
    end

    subgraph Social["Social Graph Service (Java Spring Boot)"]
        SG[Users / Follows REST API]
    end

    subgraph Data Layer
        PG[(PostgreSQL)]
        RD[(Redis)]
        KF[[Apache Kafka]]
    end

    subgraph Offline["Offline / Batch (Python)"]
        SEED[Data Seeder]
        RANK[Ranking Model Trainer]
        LOAD[Load Test Harness - Locust]
    end

    FE -->|REST/JSON| GW
    GW -->|gRPC| PS
    GW -->|gRPC| FS
    GW -->|REST| SG

    PS -->|write| PG
    PS -->|publish PostCreated| KF
    SG -->|read/write| PG

    KF -->|consume| FW
    FW -->|read follower list| PG
    FW -->|write timeline ZSET| RD
    FW -->|cache-warm post| RD

    FS -->|read timeline ZSET, post cache| RD
    FS -->|cache miss fallback| PG
    FS -->|cache miss fallback| PS

    SEED --> PG
    SEED --> KF
    RANK --> PG
    RANK -.->|weights.json| FS
    LOAD --> GW
    LOAD --> FS
```

## Write path (creating a post) — fanout-on-write

```mermaid
sequenceDiagram
    participant U as User (via FE/GW)
    participant PS as Post Service (Go)
    participant PG as PostgreSQL
    participant KF as Kafka (post-events)
    participant FW as Fanout Worker (Go)
    participant RD as Redis

    U->>PS: CreatePost(authorId, content)
    PS->>PG: INSERT INTO posts (...)
    PS->>RD: SET post:{id} (write-through cache)
    PS->>KF: publish PostCreated{postId, authorId, ts}
    PS-->>U: 201 Created (postId)

    KF->>FW: consume PostCreated
    FW->>PG: SELECT follower_id FROM follows WHERE followee_id = authorId
    alt author follower_count < CELEBRITY_THRESHOLD
        loop for each follower (batched pipeline)
            FW->>RD: ZADD timeline:{followerId} score postId
            FW->>RD: ZREMRANGEBYRANK timeline:{followerId} (trim to MAX_TIMELINE_LEN)
        end
    else author is a celebrity
        FW->>RD: ZADD celebrity_posts score postId
        Note over FW,RD: No per-follower fanout - merged at read time via fanout-on-read
    end
```

## Read path (loading a feed)

```mermaid
sequenceDiagram
    participant U as User (via FE/GW)
    participant FS as Feed Service (Go)
    participant RD as Redis
    participant PG as PostgreSQL

    U->>FS: GetFeed(userId, pageToken)
    FS->>RD: ZREVRANGE timeline:{userId} (candidate post IDs)
    FS->>RD: cached list of celebrities the user follows
    FS->>RD: ZREVRANGE celebrity_posts (recent celebrity candidates)
    FS->>FS: merge candidate sets
    FS->>RD: MGET post:{id} for all candidates (batched)
    alt cache miss on some posts
        FS->>PG: SELECT * FROM posts WHERE id IN (...)
        FS->>RD: backfill post:{id} cache (cache warming on read)
    end
    FS->>FS: rank(candidates) using scoring function
    FS-->>U: ranked, paginated feed
```

## Data model (PostgreSQL)

Schema as of Phase 1 (`migrations/000001`-`000004`). See
[`IMPLEMENTATION_PLAN.md` §4](../IMPLEMENTATION_PLAN.md#4-data-model-postgresql) for the full
rationale behind each denormalized field and index.

```mermaid
erDiagram
    USERS {
        bigint id PK
        text username
        text display_name
        boolean is_celebrity
        bigint follower_count
        timestamptz created_at
    }

    FOLLOWS {
        bigint follower_id PK, FK
        bigint followee_id PK, FK
        timestamptz created_at
    }

    POSTS {
        bigint id PK
        bigint author_id FK
        text content
        text media_url
        timestamptz created_at
        timestamptz deleted_at
    }

    ENGAGEMENTS {
        bigint id PK
        bigint post_id FK
        bigint user_id FK
        text type
        timestamptz created_at
    }

    USERS ||--o{ FOLLOWS : "follower_id"
    USERS ||--o{ FOLLOWS : "followee_id"
    USERS ||--o{ POSTS : "author_id"
    USERS ||--o{ ENGAGEMENTS : "user_id"
    POSTS ||--o{ ENGAGEMENTS : "post_id"
```

Notes:

- `users.follower_count` and `users.is_celebrity` are intentionally denormalized (§4) so the
  Fanout Worker's celebrity check doesn't require a `COUNT(*)` over `follows` on every post.
- `follows` has two indexes beyond its composite primary key: `idx_follows_followee` (fanout —
  "give me all followers of X") and `idx_follows_follower` (read-time celebrity merge — "who
  does this user follow").
- `posts.deleted_at` implements the soft-delete + filter-on-read invalidation strategy described
  in §7.4, avoiding the need to purge a deleted post ID from every follower's Redis ZSET.
- `engagements` feeds the ranking signal in §8.1 (and the optional, deprioritized offline model
  in §8.2).
- All tables currently live in a single `public` schema in one Postgres instance/database
  (`cascade`), per the "one instance, service-owned tables" decision in §4 — not split into
  separate physical databases per service, to avoid distributed-transaction concerns at this
  project's scope.

## Migrations

Applied and rolled back with [`golang-migrate`](https://github.com/golang-migrate/migrate):

```bash
make migrate-up                                    # apply all pending migrations
make migrate-down                                   # roll back everything
make migrate-create name=add_something               # scaffold a new migration pair
```

`DATABASE_URL` defaults to `postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable`
(see the root `Makefile`); override it for other environments.

## Fanout Worker reliability and Redis keys (Phase 5)

The worker consumes `post-events` and `follow-events` with Kafka auto-commit disabled. It polls
one record at a time, processes it, synchronously commits that exact offset, then permits a
rebalance. Transient processing failures receive bounded retry/backoff; permanent parse or
validation failures go directly to the topic's DLQ. A poison record is committed only after
the original key/value and failure metadata have been durably produced to the DLQ.

| Key | Type | Phase 5 behavior |
|---|---|---|
| `timeline:{userId}` | ZSET | Normal posts and new-follow backfill; member is post ID, score is creation time; bounded by `MAX_TIMELINE_LEN` |
| `celebrity_posts:global` | ZSET | One write per celebrity post, regardless of follower count |
| `following:celebrities:{userId}` | SET | Celebrity followees merged into the feed at read time |
| `fanout:follower_count:{authorId}` | STRING | Short-lived count cache, invalidated by follow events |
| `tombstones` | SET | Deleted post IDs filtered from cached timelines; TTL refreshed on each delete |

Redis `ZADD` and `SADD` operations are intentionally idempotent: Kafka's at-least-once
redelivery updates an existing member instead of duplicating it. Timeline trim commands run in
the same Redis pipeline/transaction as each add. Phase 5 directly reads the Social Graph
Service's tables as the temporary coupling documented in §7.2 of the implementation plan;
Phase 9.5 replaces that access with the paginated REST boundary.

## Feed read path and ranking (Phases 6-7)

`GetFeed` reads a bounded number of IDs from `timeline:{userId}` and
`celebrity_posts:global` in one Redis pipeline. It reads the viewer's
`following:celebrities:{userId}` set at the same time, checks all candidates against the global
`tombstones` set in one `SMISMEMBER`, then hydrates the deduplicated IDs using one Redis MGET.
All cache misses are sent in one `PostService.GetPosts` request and backfilled through a Redis
pipeline. Celebrity candidates are retained only when their hydrated author is in the viewer's
followed-celebrity set.

The resulting candidate pool is scored in Feed Service's Go process—there is no ranking
network hop:

```text
score(post, viewer) =
      w_recency    * exp(-post_age / half_life)
    + w_engagement * log1p(likes + 2 * comments)
    + w_affinity   * viewer_author_affinity
```

One grouped PostgreSQL query loads engagement counts for every candidate, and a second grouped
query loads the viewer's author-affinity counts over the configured window. Weights, half-life,
window, and cold-start affinity are environment configuration. Ranking is deterministic:
score descending, then creation time descending, then post ID descending. Pagination uses a
versioned URL-safe base64 keyset cursor containing those same three sort fields, so page
boundaries do not depend on an unstable offset.

## Cache invalidation and warming (Phase 8)

Post delete is filter-on-read, not fanout-on-delete. Post Service deletes `post:{id}` and
`SADD`s the ID to the global `tombstones` set in the same Redis transaction, then publishes
`PostDeleted`. Feed Service drops tombstoned IDs during candidate load *and* hydration, so a
delete is visible on the next read even if Kafka has not been consumed yet and even if a stale
`post:{id}` value is still cached. The tombstone set carries a TTL (`POST_TOMBSTONE_TTL` /
`TOMBSTONE_TTL`, default 24h) refreshed on every delete so recently-deleted IDs outlast typical
timeline turnover. After expiry, `GetPosts` still omits `deleted_at IS NOT NULL` rows.

Two warming paths:

1. **Online new-follow backfill.** `FollowCreated` for a normal account writes that author's
   recent posts into the follower's timeline ZSET. Celebrity follows only update
   `following:celebrities:{followerId}`; historical celebrity posts are merged at read time.
   Unfollow removes the celebrity marker and does not rewrite historical normal timelines.
2. **Cold-start / deploy warming.** `make warm-cache` (`services/fanout-worker/cmd/warm-cache`)
   reads users, follows, and recent posts from Postgres and rebuilds `timeline:{userId}`,
   `following:celebrities:{userId}`, `celebrity_posts:global`, follower-count keys, and
   `post:{id}` content cache. Use this after a Redis restart with no persistence, or to reset
   caches between Phase 12 benchmark runs.

## API Gateway / BFF (Phase 9)

The Gateway is the only public HTTP surface. It terminates JSON from the frontend, calls Post
and Feed over gRPC, and calls Social Graph over REST. `X-User-Id` is the auth stub: required
for feed, post, and follow mutations; absent for creating/fetching simulated users. Missing or
non-positive values return 401. `GET /api/feed` aggregates author `username` / `displayName` /
celebrity flag from Social Graph's batch `GET /users?ids=` so Feed Service stays a lean hot
path. `POST /api/posts` returns the created IDs immediately so the author's client can
read-your-own-write by prepending, while follower feeds remain eventually consistent through
the Kafka fanout path.

## Frontend demo (Phase 10)

The Next.js app is the only user-facing client. It stores the simulated viewer in
`localStorage` and sends `X-User-Id` on every Gateway call. `/feed` infinite-scrolls ranked
items, prepends the author's own posts immediately, and keeps a "fanning out…" hint until that
post is observed in another user's feed. `/graph` follow/unfollows other seeded users.
`/admin` polls `GET /api/admin/metrics`.

## Observability (Phase 11)

Request IDs originate at the Gateway (`X-Request-Id`) and propagate to Post/Feed over gRPC
metadata and to Social Graph over HTTP. Each service logs JSON (Go `slog`, Spring Boot ECS)
including that ID. Prometheus scrapes Compose service names (`feed-service:9101`,
`post-service:9100`, `fanout-worker:9102`, `gateway:8080`, `social-graph-service:8081`).
Grafana on port 3001 loads a provisioned dashboard for feed req/sec, latency percentiles,
cache hit ratio, fanout lag, and Kafka consumer lag. Jaeger tracing is intentionally out of
scope.

## Load testing (Phase 12)

`loadtest/seed.py` builds a power-law follow graph with `COPY` (not REST). `--preset full` is
the 50k-user plan dataset; `--preset ci` scales celebrity follower counts and the celebrity
threshold so the hybrid path still exists on 500 users. Locust's `CascadeUser` calls
`GET /api/feed` and `POST /api/posts` at 100:1 using seeded `X-User-Id` values.

Feed Service `FEED_BYPASS_CACHE=true` swaps Redis timeline reads and Redis/Post hydration for
three candidate SQL queries plus one `posts` hydration query. Post Service
`POST_BYPASS_CACHE=true` makes `GetPosts` skip Redis. The comparison metric is
`feed_postgres_queries_total` / `post_postgres_queries_total` scraped around each Locust run
(`loadtest/benchmark.py`). Write-ups live under `docs/benchmarks/`.

## Docker Compose (Phase 13)

`deploy/docker-compose.yml` runs the data plane and every application process. App images are
multi-stage (Go toolchain + `protoc` for gitignored stubs; Maven Wrapper for Java; Next.js
`output: "standalone"`). `depends_on: condition: service_healthy` (and kafka-init
`service_completed_successfully`) keeps services from crash-looping on a cold Kafka/Postgres.
`scripts/smoke_test.py` (`make smoke`) is the one-command proof: two users, a follow, a post,
then poll the follower feed until fanout lands. `warm-cache` is a Compose `tools` profile so
`make up` does not run it as a long-lived service.

## Local Kubernetes (Phase 14)

`make kind-up` builds the Compose app images, `kind load`s them into a cluster named
`cascade`, and applies `deploy/k8s`. Gateway NodePort 30080 is mapped to host 8080 so the
existing smoke script does not change. Feed Service has a CPU HPA (1–4 replicas, 50% of a
100m request). metrics-server is installed with `--kubelet-insecure-tls` (kind only). There
are no cloud load-balancer annotations. See [ADR 0007](decisions/0007-local-kind-only.md).

## Testing strategy (Phase 16)

Unit tests cover the pure decisions (hybrid fanout threshold, heuristic reorder, cursor
codec, Zipf graph). Feed Service and Fanout Worker integration tests start real
Postgres/Redis/Kafka via testcontainers-go when Docker is available, and skip otherwise.
`scripts/smoke_test.py` is the whole-system check.
CI also runs a cheap perf guard: ci-preset graph generation < 1s, and `Rank(500)` < 50ms.
Live Locust remains a manual/kind exercise.

## ADRs

Recorded under [`docs/decisions/`](decisions/README.md). Phase 9.5 (Fanout Worker calls Social
Graph over REST instead of SQL) is explicitly deferred in
[ADR 0004](decisions/0004-fanout-direct-postgres.md).
