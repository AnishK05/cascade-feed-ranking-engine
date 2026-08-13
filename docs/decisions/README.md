# Architecture Decision Records

Short records of the non-obvious calls in Cascade. Format: Context → Decision → Consequences.
The plan's decisions log (`IMPLEMENTATION_PLAN.md` §19) is the source of truth for *what* was
decided; these files are the interview-facing *why*.

| ADR | Title |
|-----|--------|
| [0001](0001-kafka-kraft-not-redpanda.md) | Real Apache Kafka (KRaft), not Redpanda |
| [0002](0002-hybrid-fanout-threshold.md) | Hybrid fanout at 10,000 followers |
| [0003](0003-redis-zset-timelines.md) | Redis ZSETs for per-user timelines |
| [0004](0004-fanout-direct-postgres.md) | Fanout Worker reads `follows` from Postgres (Phase 9.5 deferred) |
| [0005](0005-auth-stub.md) | Spoofable `X-User-Id` instead of real auth |
| [0006](0006-heuristic-ranking.md) | Heuristic ranking, not a trained model |
| [0007](0007-local-kind-only.md) | Local `kind` only — no cloud Kubernetes |
| [0008](0008-json-kafka-payloads.md) | JSON Kafka payloads, no Schema Registry |
