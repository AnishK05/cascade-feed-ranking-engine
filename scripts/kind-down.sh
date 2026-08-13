#!/usr/bin/env bash
set -euo pipefail
CLUSTER="${KIND_CLUSTER_NAME:-cascade}"
if command -v kind >/dev/null 2>&1; then
  kind delete cluster --name "${CLUSTER}" || true
else
  echo "kind-down: kind is not installed; nothing to delete" >&2
  exit 1
fi
