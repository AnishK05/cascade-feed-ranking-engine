# ADR 0006: Heuristic ranking, not a trained model

## Context

A resume-friendly ranking story could mean an offline Python model writing `weights.json`.
The project's learning goals are serving infrastructure (fanout, cache, gRPC latency), not
ML training.

## Decision

Rank in-process with

```
score = recency_weight * exp(-age / half_life)
      + engagement_weight * log1p(likes + 2 * comments)
      + affinity_weight * viewer_author_affinity
```

Weights and half-life are environment variables / ConfigMap keys. The Python trainer under
`ranking/` stays a deprioritized stretch.

## Consequences

- Reordering is testable with table-driven Go tests and no model artifact.
- Changing weights does not require a deploy of a new binary if env is updated (Compose/kind
  restart the Feed pod).
- Affinity is a 30-day engagement count, not a learned embedding.
