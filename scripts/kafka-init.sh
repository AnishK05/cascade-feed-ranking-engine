#!/usr/bin/env bash
set -euo pipefail

bootstrap_servers="${KAFKA_BOOTSTRAP_SERVERS:-kafka:29092}"
partitions="${KAFKA_TOPIC_PARTITIONS:-6}"
replication_factor="${KAFKA_REPLICATION_FACTOR:-1}"
kafka_bin="${KAFKA_BIN_DIR:-/opt/kafka/bin}"

topics=(
  post-events
  follow-events
  engagement-events
  post-events.dlq
  follow-events.dlq
)

for topic in "${topics[@]}"; do
  "${kafka_bin}/kafka-topics.sh" \
    --bootstrap-server "${bootstrap_servers}" \
    --create \
    --if-not-exists \
    --topic "${topic}" \
    --partitions "${partitions}" \
    --replication-factor "${replication_factor}"
done

echo "Kafka topics ready:"
"${kafka_bin}/kafka-topics.sh" --bootstrap-server "${bootstrap_servers}" --list
