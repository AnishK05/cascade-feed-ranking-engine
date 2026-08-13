# ADR 0002: Hybrid fanout at 10,000 followers

## Context

Fanout-on-write makes GetFeed a Redis ZREVRANGE, but write amplification is O(followers).
Celebrity accounts (tens of thousands of followers) make that write path unbounded.
Fanout-on-read is cheap to write and expensive to read.

## Decision

Authors with `follower_count < 10_000` are fanned out on write into each follower's
`timeline:{id}` ZSET. At or above the threshold, a post is written once to
`celebrity_posts:global` and merged at read time for users who follow that author
(`following:celebrities:{userId}`).

The 10,000 figure is a ConfigMap/env value (`CELEBRITY_FOLLOWER_THRESHOLD`), not a constant
baked into ranking. The `--preset ci` seeder lowers it so a 500-user graph still has
celebrities.

## Consequences

- GetFeed stays one pipeline of Redis reads for the common case.
- Celebrity posts are eventually consistent for everyone, not just their followers' ZSETs.
- Load tests must use a Zipf graph; a uniform random graph would never hit this branch.
