#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INPUT_FILE="${1:-${ROOT_DIR}/coverage-packages.txt}"

if [[ ! -f "${INPUT_FILE}" ]]; then
  echo "coverage package file not found: ${INPUT_FILE}" >&2
  exit 1
fi

check_package() {
  local pkg="$1"
  local min="$2"
  local line
  line="$(grep -E "[[:space:]]github.com/.+/${pkg}[[:space:]].*coverage:" "${INPUT_FILE}" | tail -n 1 || true)"
  if [[ -z "${line}" ]]; then
    echo "missing package coverage line for ${pkg}" >&2
    return 1
  fi
  local value
  value="$(echo "${line}" | sed -E 's/.*coverage: ([0-9.]+)%.*/\1/')"
  echo "${pkg}: ${value}% (min ${min}%)"
  awk -v value="${value}" -v min="${min}" 'BEGIN { if (value + 0 < min + 0) exit 1 }'
}

check_package "internal/symlink" 75
check_package "internal/backup" 75
check_package "internal/repo" 70
check_package "internal/update" 70
check_package "internal/cli" 70

echo "Package thresholds passed."
