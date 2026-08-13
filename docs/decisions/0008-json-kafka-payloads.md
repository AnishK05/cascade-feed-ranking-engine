# ADR 0008: JSON Kafka payloads, no Schema Registry

## Context

A binary schema (Protobuf/Avro + Schema Registry) would make Post Service and Fanout Worker
share a contract the way gRPC already does for GetFeed. It also adds a registry process and
compatibility rules that are not the focus of the serving path.

## Decision

`PostCreated`, `PostDeleted`, `FollowCreated`, and `FollowDeleted` are JSON objects keyed by
entity ID. Malformed payloads go to a DLQ. Protobuf remains the RPC contract (Post/Feed
gRPC), not the log format.

## Consequences

- Fanout Worker and Social Graph can evolve fields with optional JSON keys.
- There is no compatibility check beyond "can we unmarshal this map."
- A Schema Registry remains a stretch goal only.
