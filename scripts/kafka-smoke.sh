#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${repo_root}/deploy/docker-compose.yml"
topic="cascade-smoke-$(date +%s)-$$"
message="cascade-kafka-smoke-${topic}"

kafka() {
  local command="$1"
  shift
  if [[ -n "${KAFKA_BIN_DIR:-}" ]]; then
    "${KAFKA_BIN_DIR}/${command}" "$@"
  else
    docker compose -f "${compose_file}" exec -T kafka "/opt/kafka/bin/${command}" "$@"
  fi
}

cleanup() {
  kafka kafka-topics.sh \
    --bootstrap-server "${bootstrap_servers}" \
    --delete \
    --if-exists \
    --topic "${topic}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

if [[ -n "${KAFKA_BIN_DIR:-}" ]]; then
  bootstrap_servers="${KAFKA_BOOTSTRAP_SERVERS:-localhost:9092}"
else
  bootstrap_servers="localhost:29092"
fi

kafka kafka-topics.sh \
  --bootstrap-server "${bootstrap_servers}" \
  --create \
  --topic "${topic}" \
  --partitions 1 \
  --replication-factor 1 >/dev/null

printf '%s\n' "${message}" |
  kafka kafka-console-producer.sh \
    --bootstrap-server "${bootstrap_servers}" \
    --topic "${topic}" >/dev/null

received="$(
  kafka kafka-console-consumer.sh \
    --bootstrap-server "${bootstrap_servers}" \
    --topic "${topic}" \
    --from-beginning \
    --max-messages 1 \
    --timeout-ms 10000 2>/dev/null
)"

if [[ "${received}" != "${message}" ]]; then
  echo "Kafka smoke test failed: expected '${message}', received '${received}'" >&2
  exit 1
fi

echo "Kafka smoke test passed: produced and consumed '${received}'"
