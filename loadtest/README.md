# loadtest/

Python tooling for seeding a power-law follow graph and running the before/after Redis-cache
benchmark in `IMPLEMENTATION_PLAN.md` §13.

To bring the stack up first, see [`docs/running.md`](../docs/running.md). On Windows,
`cascade.cmd seed` / `cascade.cmd loadtest` wrap the same scripts.

## Setup

```bash
cd loadtest
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt -r requirements-dev.txt
```

## Seed

`seed.py` COPY-inserts users, follows, posts, and a sample of likes. It writes
`loadtest/data/user_ids.json` and `seed_meta.json` (gitignored).

| Preset | Users | Celebrities | Celebrity threshold |
|--------|-------|-------------|---------------------|
| `ci`   | 500   | 5           | 80                  |
| `full` | 50,000 | 50         | 10,000              |

```bash
python seed.py --preset ci --reset
# then, with services up:
make -C .. warm-cache
```

Match Fanout Worker / Social Graph to the printed threshold:

```text
CELEBRITY_FOLLOWER_THRESHOLD=80 CELEBRITY_THRESHOLD=80
```

## Locust

Read:write mix is **100:1** (`GET /api/feed` : `POST /api/posts`). User IDs come from the
seed sidecar, `LOADTEST_USERS_FILE`, or a comma list in `LOADTEST_USER_IDS`.

```bash
locust -f locustfile.py --headless --host http://localhost:8080 \
  -u 50 -r 10 -t 60s --csv reports/run --html reports/run.html
```

Ramp `-u` until GetFeed p99 crosses 200ms (or your chosen bound). That inflection, not a
single fixed-concurrency number, is “sustaining N req/s”.

## Before/after cache protocol

1. Seed + warm Redis.
2. **Baseline:** recreate Feed (and optionally Post) with `FEED_BYPASS_CACHE=true`
   (`POST_BYPASS_CACHE=true` if you want GetPosts to skip Redis too).
3. `python benchmark.py run --label baseline --users 50 --spawn-rate 10 --duration 60s`
4. **Cached:** `FEED_BYPASS_CACHE=false`, warm Redis again, `benchmark.py run --label cached …`
5. `python benchmark.py compare --baseline-csv reports/baseline_stats.csv \
     --cached-csv reports/cached_stats.csv \
     --baseline-metrics reports/baseline_metrics.json \
     --cached-metrics reports/cached_metrics.json`

Reduction is `1 − (cacheable Postgres QPS with cache ÷ baseline)`. Cacheable ops are Feed
`candidates` + `hydrate` and Post `get_posts`. Ranking `signals` queries run in both modes.

Do not paste the plan’s example 8k req/s or 80% reduction unless this run’s artifacts contain
those values. See `docs/benchmarks/YYYY-MM-DD-cache-comparison.md`.

## Tests

```bash
pytest
ruff check .
```

CI runs the unit suite only (graph construction, Locust task weights, CSV/Prometheus parsers).
It does not seed 50k users or talk to a live Gateway.
