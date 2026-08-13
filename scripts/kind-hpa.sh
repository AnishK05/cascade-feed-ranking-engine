#!/usr/bin/env bash
# Print Feed Service HPA status. Run Locust in another terminal to watch replicas climb:
#   make loadtest USERS=80 DURATION=2m HOST=http://localhost:8080
set -euo pipefail
kubectl -n cascade get hpa feed-service
echo "---"
kubectl -n cascade get deploy feed-service
echo "Watch with: kubectl -n cascade get hpa feed-service -w"
