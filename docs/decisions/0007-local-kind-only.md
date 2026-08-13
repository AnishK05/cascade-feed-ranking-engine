# ADR 0007: Local kind only — no cloud Kubernetes

## Context

Deploying to EKS/GKE/AKS would add IAM, load balancers, and a bill. The goal is to learn
Deployments, Services, Secrets vs ConfigMaps, and HPA against the same smoke test that
Compose already runs.

## Decision

`deploy/k8s/` targets a local [kind](https://kind.sigs.k8s.io/) cluster. Images are built by
Compose, `kind load docker-image`'d, and referenced with `imagePullPolicy: Never`. Feed
Service has a CPU HorizontalPodAutoscaler (1–4 replicas). There is no Ingress, no cloud load
balancer annotation, and no managed Kubernetes.

## Consequences

- `make kind-up` then `make k8s-smoke` is the DoD; CI validates manifests rather than
  provisioning a cluster (kind + image builds are too heavy for the default PR checks).
- HPA needs metrics-server with `--kubelet-insecure-tls`, which is kind-specific.
- Credentials live in a Kubernetes Secret (`cascade-db`), not a ConfigMap.
