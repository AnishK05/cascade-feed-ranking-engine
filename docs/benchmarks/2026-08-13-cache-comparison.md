# Cache comparison — 2026-08-13

Before/after Redis cache benchmark for Cascade GetFeed, following
`IMPLEMENTATION_PLAN.md` §13.3–13.4. Numbers below are **measured**, not targets.
Do not copy the plan's example "8,000+ req/s" or "80% reduction" unless this
file's Locust/Prometheus tables actually contain those values.

## Setup

- **Machine / environment:** Cloud agent VM (hostname cursor): 4 vCPU Intel Xeon, 16 GB RAM (13,542 MB available at measurement time). Docker is not installed in this environment, so Compose/Gateway/Locust did not run here.
- **Feed Service instances:** 1 Feed Service replica (intended; not started in this environment)
- **Latency threshold used to interpret throughput:** p99 < 200ms
- **Read:write mix:** 100:1 (`GET /api/feed` : `POST /api/posts`)
- **Baseline config:** `FEED_BYPASS_CACHE=true` (candidates + hydration from PostgreSQL)
- **Cached config:** `FEED_BYPASS_CACHE=false` (Redis timelines + post cache, Post Service on miss)
- **Comparison metric:** `1 − (Postgres QPS with cache ÷ Postgres QPS baseline)` on
  `feed_postgres_queries_total{op="candidates|hydrate"}` + `post_postgres_queries_total{op="get_posts"}`

## Results

**Measured cacheable-Postgres load reduction: not measured**

Locust throughput and the Redis vs Postgres query comparison require the full Compose stack
(Gateway + Feed + Post + Kafka). Those processes were not running here. What *was* measured
on this VM:

| Measurement | Result |
|---|---|
| Graph `--preset ci` (in-process, seed 42) | 500 users, 5 celebrities, 7,922 follow edges in **0.007s** |
| Graph `--preset full` (in-process, seed 42) | 50,000 users, 50 celebrities, 7,840,109 follow edges in **3.594s** |
| Postgres `COPY` `--preset ci` | 500 users, 7,922 follows, 3,688 posts, 2,212 likes in **0.262s** |
| Locust `GET /api/feed` req/s at p99 < 200ms | not measured (Gateway down) |
| Cacheable Postgres QPS reduction | not measured (Feed/Post down) |

Do not treat the plan's example 8,000 req/s or 80% reduction as results from this run.

### Baseline (cache bypassed)

Not measured in this environment.

### Baseline Postgres counters

Not measured in this environment.

### With Redis cache

Not measured in this environment.

### Cached Postgres counters

Not measured in this environment.


## How to reproduce

```bash
make up && make smoke
make seed PRESET=ci
make warm-cache-compose

FEED_BYPASS_CACHE=true docker compose -f deploy/docker-compose.yml up -d --force-recreate feed-service
make benchmark LABEL=baseline USERS=50 DURATION=60s

FEED_BYPASS_CACHE=false docker compose -f deploy/docker-compose.yml up -d --force-recreate feed-service
make warm-cache-compose
make benchmark LABEL=cached USERS=50 DURATION=60s

cd loadtest && python benchmark.py compare \
  --baseline-csv reports/baseline_stats.csv --cached-csv reports/cached_stats.csv \
  --baseline-metrics reports/baseline_metrics.json --cached-metrics reports/cached_metrics.json
```

Ramp `-u` until GetFeed p99 crosses the latency threshold above; the last
concurrency that stayed under the threshold is the "sustaining N req/s" figure.

## Notes

- Docker Engine is not available on this VM, so make up, make smoke, and Locust-against-Gateway could not be executed here.
- In-process graph generation (seed 42): --preset ci produced 500 users / 5 celebrities / 7,922 follow edges in 0.007s (assign 0.001s + edges 0.006s). --preset full produced 50,000 users / 50 celebrities / 7,840,109 follow edges in 3.594s (assign 0.094s + edges 3.500s).
- Postgres COPY seed --preset ci against localhost:5432 completed in 0.262s (500 users, 7,922 follows, 3,688 posts, 2,212 like engagements). This truncated the local cascade database.
- Cacheable Postgres QPS before/after Redis was not measured: Post and Feed were not running (Post Service requires Kafka at startup; Kafka is not on the host).
