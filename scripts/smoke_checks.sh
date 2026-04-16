#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${ALFRED_BASE_URL:-${1:-}}"

if [[ -z "${BASE_URL}" ]]; then
  echo "usage: ALFRED_BASE_URL=http://host:port $0"
  echo "   or: $0 http://host:port"
  exit 1
fi

HEALTH_URL="${BASE_URL%/}/healthz"
METRICS_URL="${BASE_URL%/}/metrics"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need_cmd curl

json_check() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -m json.tool >/dev/null 2>&1
  else
    return 0
  fi
}

section() {
  echo
  echo "== $1 =="
}

fetch_with_status() {
  local url="$1"
  local tmp_body
  tmp_body="$(mktemp)"
  local status
  status="$(curl -sS -o "${tmp_body}" -w "%{http_code}" "${url}")"
  echo "${status}:${tmp_body}"
}

assert_contains() {
  local body_file="$1"
  local pattern="$2"
  if ! grep -q "${pattern}" "${body_file}"; then
    echo "expected pattern not found: ${pattern}" >&2
    cat "${body_file}" >&2
    exit 1
  fi
}

section "Health"
health_result="$(fetch_with_status "${HEALTH_URL}")"
health_status="${health_result%%:*}"
health_body="${health_result#*:}"

echo "GET ${HEALTH_URL} -> ${health_status}"
if [[ "${health_status}" != "200" ]]; then
  cat "${health_body}" >&2
  rm -f "${health_body}"
  exit 1
fi
if ! json_check < "${health_body}"; then
  echo "healthz did not return valid JSON" >&2
  cat "${health_body}" >&2
  rm -f "${health_body}"
  exit 1
fi
assert_contains "${health_body}" '"status":"ok"'
assert_contains "${health_body}" '"queue"'
assert_contains "${health_body}" '"features"'
cat "${health_body}"
rm -f "${health_body}"

section "Metrics"
metrics_result="$(fetch_with_status "${METRICS_URL}")"
metrics_status="${metrics_result%%:*}"
metrics_body="${metrics_result#*:}"

echo "GET ${METRICS_URL} -> ${metrics_status}"
if [[ "${metrics_status}" != "200" ]]; then
  cat "${metrics_body}" >&2
  rm -f "${metrics_body}"
  exit 1
fi
if ! json_check < "${metrics_body}"; then
  echo "metrics did not return valid JSON" >&2
  cat "${metrics_body}" >&2
  rm -f "${metrics_body}"
  exit 1
fi
assert_contains "${metrics_body}" '"counters"'
assert_contains "${metrics_body}" '"timings"'
cat "${metrics_body}"
rm -f "${metrics_body}"

section "Result"
echo "basic smoke checks passed for ${BASE_URL}"
