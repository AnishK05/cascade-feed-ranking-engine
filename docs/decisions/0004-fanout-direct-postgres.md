# ADR 0004: Fanout Worker reads follows from Postgres (Phase 9.5 deferred)

## Context

The Fanout Worker needs the follower list of a post's author. Clean service boundaries would
have it call Social Graph's paginated `GET /users/{id}/followers`. Direct SQL against
`public.follows` is faster and simpler for the first fanout implementation.

The roadmap schedules an explicit refactor (Phase 9.5) rather than leaving the coupling as
an accidental forever-state.

## Decision

Phase 5–14 keep a Postgres connection on the Fanout Worker and query `follows` / `users`
directly. Phase 9.5 (REST boundary + before/after fanout throughput) is **not** done in this
tree; it remains a scheduled follow-up, not a silent skip.

## Consequences

- Fanout stays one network hop (Kafka → worker → Redis) plus a local SQL read.
- The worker still holds DB credentials for the social tables.
- An ADR exists so the next change has a recorded baseline to compare against.
