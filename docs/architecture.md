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
