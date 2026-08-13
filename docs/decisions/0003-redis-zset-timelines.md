# ADR 0003: Redis sorted sets for timelines

## Context

Per-user feeds need "newest N post IDs, bounded, trim-on-write." Candidates were a Redis
LIST, a Redis ZSET scored by timestamp, and Postgres `ORDER BY created_at`.

## Decision

Store `timeline:{userId}` and `celebrity_posts:global` as ZSETs scored by created-at millis.
Fanout pipelines `ZADD` + `ZREMRANGEBYRANK` to cap `MAX_TIMELINE_LEN`. Feed Service reads
with `ZREVRANGE`.

## Consequences

- Pagination cursors can key off score + post ID without a second index.
- Deletes do not walk every follower ZSET; a global `tombstones` set hides IDs at read time.
- Memory is proportional to users × timeline length, which is the point of the celebrity split.
