#!/usr/bin/env bash
# Delete a Feed Service pod and wait until Gateway /api/ping succeeds again.
# Demonstrates that the Service keeps routing while the Deployment recreates the pod.
set -euo pipefail
NS=cascade
kubectl -n "${NS}" delete pod -l app=feed-service --wait=false
ok=0
for _ in $(seq 1 60); do
  if curl -fsS http://localhost:8080/api/ping >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 1
done
if [ "${ok}" -ne 1 ]; then
  echo "kind-chaos: Gateway did not recover" >&2
  kubectl -n "${NS}" get pods >&2
  exit 1
fi
echo "kind-chaos: Gateway still serving /api/ping after feed-service pod delete"
kubectl -n "${NS}" get pods -l app=feed-service
