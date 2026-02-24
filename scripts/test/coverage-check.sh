#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

STAGE="${COVERAGE_STAGE:-65}"
THRESHOLD="${COVERAGE_THRESHOLD:-}"

if [[ -z "${THRESHOLD}" ]]; then
  case "${STAGE}" in
    65|stage1|Stage1) THRESHOLD="65" ;;
    80|stage2|Stage2) THRESHOLD="80" ;;
    90|stage3|Stage3) THRESHOLD="90" ;;
    *)
      echo "unsupported COVERAGE_STAGE=${STAGE}; use 65, 80, or 90" >&2
      exit 1
      ;;
  esac
fi

echo "Running test suite for coverage gate stage ${STAGE} (>=${THRESHOLD}%)"
go test ./... -covermode=atomic -coverprofile=coverage.out -count=1 | tee coverage-packages.txt

TOTAL="$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%","",$3); print $3}')"
echo "Total coverage: ${TOTAL}%"
awk -v total="${TOTAL}" -v threshold="${THRESHOLD}" 'BEGIN { if (total + 0 < threshold + 0) exit 1 }'

"${ROOT_DIR}/scripts/test/package-thresholds.sh" "${ROOT_DIR}/coverage-packages.txt"

echo "Coverage checks passed."
