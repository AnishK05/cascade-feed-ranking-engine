#!/usr/bin/env bash
# Create (or reuse) a local kind cluster, load Compose-built images, apply deploy/k8s,
# and wait until Gateway answers /api/ping. Local only — never targets EKS/GKE/AKS.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER="${KIND_CLUSTER_NAME:-cascade}"
NS=cascade

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "kind-up: missing $1. Install Docker, kind, and kubectl on your machine." >&2
    exit 1
  fi
}

need docker
need kind
need kubectl

if ! docker info >/dev/null 2>&1; then
  echo "kind-up: Docker daemon is not running." >&2
  exit 1
fi

if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER}"; then
  kind create cluster --config "${ROOT}/deploy/k8s/kind-config.yaml" --name "${CLUSTER}"
else
  echo "kind-up: cluster ${CLUSTER} already exists"
fi

echo "kind-up: building application images"
docker compose -f "${ROOT}/deploy/docker-compose.yml" build \
  post-service feed-service fanout-worker social-graph-service gateway frontend warm-cache

images=(
  cascade/post-service:local
  cascade/feed-service:local
  cascade/fanout-worker:local
  cascade/social-graph-service:local
  cascade/gateway:local
  cascade/frontend:local
  cascade/warm-cache:local
)
for image in "${images[@]}"; do
  kind load docker-image "${image}" --name "${CLUSTER}"
done

kubectl create namespace cascade --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${NS}" delete job kafka-init --ignore-not-found
kubectl apply -k "${ROOT}/deploy/k8s"
kubectl apply -f "${ROOT}/deploy/k8s/metrics-server.yaml"

echo "kind-up: waiting for data plane"
kubectl -n "${NS}" wait --for=condition=available --timeout=180s deploy/postgres deploy/redis deploy/kafka
if ! kubectl -n "${NS}" wait --for=condition=complete --timeout=180s job/kafka-init; then
  echo "kind-up: kafka-init did not complete" >&2
  kubectl -n "${NS}" logs job/kafka-init >&2 || true
  exit 1
fi

echo "kind-up: waiting for application deployments"
kubectl -n "${NS}" wait --for=condition=available --timeout=300s \
  deploy/post-service deploy/feed-service deploy/fanout-worker \
  deploy/social-graph-service deploy/gateway deploy/frontend

echo "kind-up: waiting for Gateway /api/ping on localhost:8080"
ok=0
for _ in $(seq 1 60); do
  if curl -fsS http://localhost:8080/api/ping >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 2
done
if [ "${ok}" -ne 1 ]; then
  echo "kind-up: Gateway did not become ready. Pods:" >&2
  kubectl -n "${NS}" get pods >&2
  exit 1
fi

echo "kind-up: cluster is ready"
echo "  UI:      http://localhost:3000"
echo "  Gateway: http://localhost:8080"
echo "  Smoke:   make k8s-smoke"
echo "  HPA:     kubectl -n cascade get hpa feed-service -w"
echo "  Chaos:   make k8s-chaos"
