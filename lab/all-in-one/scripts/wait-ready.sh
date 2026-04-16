#!/bin/sh

set -eu

for _ in $(seq 1 120); do
  if kubectl get nodes -o name 2>/dev/null | grep -q .; then
    kubectl wait --for=condition=Ready node --all --timeout=120s
    exit 0
  fi
  sleep 1
done

echo "k3s did not become ready in time" >&2
exit 1
