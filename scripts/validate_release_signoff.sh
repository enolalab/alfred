#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <signoff-file>" >&2
  exit 1
fi

signoff_file="$1"

if [[ ! -f "${signoff_file}" ]]; then
  echo "sign-off file not found: ${signoff_file}" >&2
  exit 1
fi

if [[ "${signoff_file}" != docs/release-signoffs/*.md ]]; then
  echo "sign-off file must live under docs/release-signoffs/: ${signoff_file}" >&2
  exit 1
fi

signoff_basename="$(basename "${signoff_file}")"

case "${signoff_basename}" in
  README.md)
    echo "sign-off file must be a real release record, not the directory README: ${signoff_file}" >&2
    exit 1
    ;;
  release-signoff.template.md)
    echo "sign-off file must not use the shared template path directly: ${signoff_file}" >&2
    exit 1
    ;;
  replace-with-real-signoff.md)
    echo "sign-off file must not use the placeholder workflow path: ${signoff_file}" >&2
    exit 1
    ;;
esac

if grep -qi "replace-me" "${signoff_file}"; then
  echo "sign-off file still contains placeholder values: ${signoff_file}" >&2
  exit 1
fi

required_fields=(
  "image_tag:"
  "fixture_set:"
  "reviewers:"
  "decision:"
)

for field in "${required_fields[@]}"; do
  if ! grep -q "^${field}[[:space:]]*[^[:space:]].*$" "${signoff_file}"; then
    echo "missing or empty required field '${field}' in ${signoff_file}" >&2
    exit 1
  fi
done

if ! grep -q "^decision:[[:space:]]*pass[[:space:]]*$" "${signoff_file}"; then
  echo "release sign-off decision must be 'pass' in ${signoff_file}" >&2
  exit 1
fi

echo "Validated release sign-off: ${signoff_file}"
