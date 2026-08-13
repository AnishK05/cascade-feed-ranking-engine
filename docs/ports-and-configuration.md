# Ports and configuration

Compose and kind both publish the **same host ports** so `make smoke` /
`cascade.cmd smoke` stay `http://localhost:8080`. Do not run both at once.

## Host ports

| Port | Process |
|------|---------|
| 3000 | Next.js UI |
| 3001 | Grafana |
| 5432 | Postgres |
| 6379 | Redis |
| 8080 | Gateway (BFF) |
| 8081 | Social Graph |
| 9090 | Post Service gRPC |
| 9091 | Feed Service gRPC |
| 9092 | Kafka (host / advertised `PLAINTEXT_HOST`) |
| 9095 | Prometheus (container 9090) |
| 9100 | Post Service `/metrics` |
| 9101 | Feed Service `/metrics` |
| 9102 | Fanout Worker `/metrics` |

Inside Compose, Kafka brokers for **containers** are `kafka:29092`. Host-native
clients use `localhost:9092`.

kind extra mappings: Gateway NodePort **30080 → 8080**, frontend **30300 → 3000**.

## Credentials (demo only)

Default user/password/database are `cascade` / `cascade` / `cascade`.
`DATABASE_URL=postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable`
on the host; inside Compose the hostname is `postgres`.

Do not expose this stack. `X-User-Id` is the auth model.

Copy [`.env.example`](../.env.example) to `.env` if you run services **on the host**.
`make up` / `cascade.cmd up` inject Compose `environment:` blocks and ignore `.env`
for most app settings unless you change the compose file.

## Environment variables (host-native)

| Variable | Default | Who |
|----------|---------|-----|
| `DATABASE_URL` | `postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable` | Go services, Fanout, Social Graph (also accepts JDBC-style) |
| `DB_USER` / `DB_PASSWORD` | `cascade` | Social Graph |
| `REDIS_ADDR` | `localhost:6379` | Post, Feed, Fanout |
| `KAFKA_BROKERS` / `KAFKA_BOOTSTRAP_SERVERS` | `localhost:9092` | Post, Fanout, Social Graph |
| `POST_SERVICE_ADDR` | `localhost:9090` | Gateway, Feed |
| `FEED_SERVICE_ADDR` | `localhost:9091` | Gateway |
| `SOCIAL_GRAPH_BASE_URL` | `http://localhost:8081` | Gateway |
| `GATEWAY_PORT` | `8080` | Gateway |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Gateway |
| `PROMETHEUS_URL` | `http://localhost:9095` | Gateway admin metrics |
| `CELEBRITY_THRESHOLD` | `10000` | Social Graph (`is_celebrity`) |
| `CELEBRITY_FOLLOWER_THRESHOLD` | `10000` | Fanout (must match seed preset: **80** for `ci`) |
| `FEED_*_WEIGHT` / `FEED_RECENCY_HALF_LIFE` | `1` / `12h` | Feed ranking |
| `FEED_BYPASS_CACHE` / `POST_BYPASS_CACHE` | `false` | Phase 12 baseline |
| `NEXT_PUBLIC_API_BASE` | `http://localhost:8080` | Frontend **build-time** |
| `GATEWAY_URL` | `http://localhost:8080` | `scripts/smoke_test.py` |
| `SMOKE_TIMEOUT_S` | `45` | Smoke poll budget |

Kind/Compose set in-cluster hosts (`postgres`, `redis`, `kafka:29092`,
`post-service:9090`, …) via ConfigMap/Secret or compose `environment`.

## Celebrity threshold vs seed

| Seed preset | Users | Threshold to set on Fanout + Social Graph |
|-------------|-------|-------------------------------------------|
| `ci` (default) | 500 | **80** |
| `full` | 50,000 | **10000** (Compose default) |

If you seed `ci` but leave the threshold at 10,000, nobody is a celebrity at
runtime and hybrid fanout will not match the seeded `is_celebrity` flags.
