# Kubernetes (local kind only)

Phase 14. These manifests are for a laptop `kind` cluster — not EKS, GKE, or AKS.

Stop Compose first (`make down` / `cascade.cmd down`) so ports 8080 and 3000 are free.

Full runbook: [`docs/running.md`](../../docs/running.md). Windows: `cascade.cmd kind-up`.

```bash
# Docker + kind + kubectl on the host
make kind-up      # build images, kind load, apply, wait for Gateway
make k8s-smoke    # same create-post → fanout → GetFeed script as Compose
make k8s-hpa      # kubectl get hpa feed-service
make k8s-chaos    # delete a feed-service pod, wait for /api/ping
make kind-down
```

Gateway is NodePort `30080` mapped to host `8080`; the UI is `30300` → host `3000`.
`make smoke` therefore works unchanged.

Feed Service CPU HPA: 1–4 replicas at 50% of a 100m request. Generate load with
`make loadtest USERS=80 DURATION=2m` and `kubectl -n cascade get hpa feed-service -w`.

DB credentials are a Secret (`cascade-db`); ranking weights and the celebrity threshold
are a ConfigMap. App images use `imagePullPolicy: Never` so they must be `kind load`'d.

`make k8s-validate` checks this directory without a cluster (CI runs kubeconform).

`init/` holds copies of `migrations/*.up.sql` and `scripts/kafka-init.sh` because kustomize
will not load files from outside this directory. A pytest guard fails if the copies drift.
