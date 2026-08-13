# ADR 0001: Real Apache Kafka in KRaft mode, not Redpanda

## Context

The fanout path needs a durable, ordered log of `PostCreated` / `FollowCreated` events.
Local-dev options were a single Apache Kafka container (KRaft, no ZooKeeper) versus Redpanda
as a Kafka-API-compatible stand-in with a smaller footprint.

## Decision

Use real Apache Kafka 4.x in combined broker/controller KRaft mode, one node, topic
auto-create disabled. Compose and kind both run `apache/kafka:4.3.1`.

## Consequences

- Interview story matches production vocabulary (partitions, consumer groups, DLQs, lag).
- Topic creation is explicit (`scripts/kafka-init.sh`), so misspelled topic names fail loudly.
- Single-node KRaft is not highly available; that is acceptable because this is a local
  learning cluster, not a cloud deployment.
