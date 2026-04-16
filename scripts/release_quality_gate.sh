#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

GOCACHE_DIR="${GOCACHE:-/tmp/alfred-go-build}"
ARTIFACT_DIR="${ROOT_DIR}/artifacts/release-quality-gate"

rm -rf "${ARTIFACT_DIR}"
mkdir -p "${ARTIFACT_DIR}"

echo "[1/4] Running Go test suite"
go test ./...

echo "[2/4] Rendering replay review scaffolding"
GOCACHE="${GOCACHE_DIR}" go run ./cmd/alfred replay >"${ARTIFACT_DIR}/replay-review.txt"
echo "Replay review written to ${ARTIFACT_DIR}/replay-review.txt"

echo "[3/4] Validating runtime Kustomize overlays"
kubectl kustomize deploy/kustomize/overlays/staging >"${ARTIFACT_DIR}/alfred-staging.yaml"
kubectl kustomize deploy/kustomize/overlays/production >"${ARTIFACT_DIR}/alfred-production.yaml"
kubectl kustomize deploy/kustomize/overlays/production-external-secrets >"${ARTIFACT_DIR}/alfred-production-external-secrets.yaml"

echo "[4/4] Validating monitoring Kustomize overlays"
kubectl kustomize deploy/monitoring/kustomize/overlays/kube-prometheus-stack-servicemonitor >"${ARTIFACT_DIR}/alfred-monitoring-servicemonitor.yaml"
kubectl kustomize deploy/monitoring/kustomize/overlays/kube-prometheus-stack-podmonitor >"${ARTIFACT_DIR}/alfred-monitoring-podmonitor.yaml"

echo "Release quality gate baseline passed."
echo "Artifacts:"
echo "- ${ARTIFACT_DIR}/replay-review.txt"
echo "- ${ARTIFACT_DIR}/alfred-staging.yaml"
echo "- ${ARTIFACT_DIR}/alfred-production.yaml"
echo "- ${ARTIFACT_DIR}/alfred-production-external-secrets.yaml"
echo "- ${ARTIFACT_DIR}/alfred-monitoring-servicemonitor.yaml"
echo "- ${ARTIFACT_DIR}/alfred-monitoring-podmonitor.yaml"
