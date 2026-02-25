#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMAGE_TAG="${DOCKER_TEST_IMAGE:-paw-test-env:local}"

cd "${ROOT_DIR}"

docker build -f tests/docker/Dockerfile.test -t "${IMAGE_TAG}" .

docker run --rm \
  -e COVERAGE_STAGE="${COVERAGE_STAGE:-65}" \
  -e COVERAGE_THRESHOLD="${COVERAGE_THRESHOLD:-}" \
  -e PAW_DOCKER_REAL_DEPS=1 \
  -e RUN_VULNCHECK="${RUN_VULNCHECK:-1}" \
  -e SEVERITY_THRESHOLD="${SEVERITY_THRESHOLD:-high}" \
  -e SKIP_QUALITY="${SKIP_QUALITY:-0}" \
  -e TZ=UTC \
  -e LANG=C.UTF-8 \
  -e LC_ALL=C.UTF-8 \
  -e HOME=/tmp/paw-home \
  -v "${ROOT_DIR}:/workspace" \
  -w /workspace \
  "${IMAGE_TAG}" \
  bash -c 'if [[ "${SKIP_QUALITY:-0}" != "1" ]]; then ./scripts/test/quality-check.sh; fi && ./scripts/test/coverage-check.sh && go test -race ./internal/symlink ./internal/backup ./internal/repo ./internal/update ./internal/cli'

echo "Docker CI test run complete."
