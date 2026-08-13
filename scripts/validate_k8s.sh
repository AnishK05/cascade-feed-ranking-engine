#!/usr/bin/env bash
# Validate Kubernetes manifests without a cluster. kubeconform is used when present
# (CI installs it); otherwise this script still checks local-only constraints.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
K8S="${ROOT}/deploy/k8s"
fail=0

required=(
  kind-config.yaml
  kustomization.yaml
  namespace.yaml
  postgres.yaml
  redis.yaml
  kafka.yaml
  kafka-init-job.yaml
  post-service.yaml
  feed-service.yaml
  feed-hpa.yaml
  fanout-worker.yaml
  social-graph.yaml
  gateway.yaml
  frontend.yaml
  metrics-server.yaml
  warm-cache-job.yaml
  init/001-users.sql
  init/002-follows.sql
  init/003-posts.sql
  init/004-engagements.sql
  init/kafka-init.sh
)
for f in "${required[@]}"; do
  if [ ! -f "${K8S}/${f}" ]; then
    echo "missing ${f}" >&2
    fail=1
  fi
done

combined="$(cat "${K8S}"/*.yaml)"
for needle in eks.amazonaws.com gke.io cloud.google.com kubernetes.azure.com elasticloadbalancing; do
  if grep -F -q "${needle}" "${K8S}"/*.yaml; then
    echo "cloud-provider annotation ${needle} is not allowed (Phase 14 is local kind only)" >&2
    fail=1
  fi
done

echo "${combined}" | grep -q "kind: HorizontalPodAutoscaler" || {
  echo "feed HPA manifest missing" >&2
  fail=1
}
echo "${combined}" | grep -q "secretGenerator:" || {
  echo "DB credentials must come from a Secret generator, not a ConfigMap" >&2
  fail=1
}
grep -q "hostPort: 8080" "${K8S}/kind-config.yaml" || {
  echo "kind-config must map host 8080 so make smoke works unchanged" >&2
  fail=1
}
grep -q "imagePullPolicy: Never" "${K8S}/feed-service.yaml" || {
  echo "app images must use imagePullPolicy Never (kind load)" >&2
  fail=1
}

if command -v kustomize >/dev/null 2>&1; then
  rendered="$(kustomize build "${K8S}")"
elif command -v kubectl >/dev/null 2>&1; then
  rendered="$(kubectl kustomize "${K8S}")"
else
  rendered=""
  echo "kustomize/kubectl not installed; skipping render"
fi

if [ -n "${rendered}" ]; then
  echo "${rendered}" | grep -q "kind: Secret" || {
    echo "kustomize output is missing a Secret" >&2
    fail=1
  }
  if command -v kubeconform >/dev/null 2>&1; then
    echo "${rendered}" | kubeconform -strict -ignore-missing-schemas -summary
    kubeconform -strict -ignore-missing-schemas -summary \
      "${K8S}/namespace.yaml" "${K8S}/metrics-server.yaml" "${K8S}/warm-cache-job.yaml"
  else
    echo "kubeconform not installed; skipping schema check"
  fi
fi

if [ "${fail}" -ne 0 ]; then
  exit 1
fi
echo "k8s manifests look valid (local kind only)"
