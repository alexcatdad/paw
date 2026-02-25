#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

QUALITY_STAGE="${QUALITY_STAGE:-baseline}"
LINT_TIMEOUT="${LINT_TIMEOUT:-5m}"
LINT_TARGET="${LINT_TARGET:-./...}"
RUN_VULNCHECK="${RUN_VULNCHECK:-1}"
SEVERITY_THRESHOLD="${SEVERITY_THRESHOLD:-high}"
read -r -a lint_targets <<< "${LINT_TARGET}"

command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 1; }
command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required" >&2; exit 1; }
command -v shellcheck >/dev/null 2>&1 || { echo "shellcheck is required" >&2; exit 1; }
command -v actionlint >/dev/null 2>&1 || { echo "actionlint is required" >&2; exit 1; }

gofmt_bin="$(command -v gofmt || true)"
if [[ -z "${gofmt_bin}" ]]; then
  gofmt_bin="$(go env GOROOT)/bin/gofmt"
fi
if [[ ! -x "${gofmt_bin}" ]]; then
  echo "gofmt is required" >&2
  exit 1
fi

echo "Running quality checks (stage=${QUALITY_STAGE})"

unformatted="$("${gofmt_bin}" -s -l . | grep -E '\.go$' | grep -v '^dist/' || true)"
if [[ -n "${unformatted}" ]]; then
  echo "Unformatted Go files found:" >&2
  echo "${unformatted}" >&2
  exit 1
fi

echo "go vet..."
go vet "${lint_targets[@]}"

echo "golangci-lint..."
lint_args=(run --timeout "${LINT_TIMEOUT}" "${lint_targets[@]}")
if [[ "${QUALITY_STAGE}" == "strict" ]]; then
  lint_args+=(-E errcheck -E gocritic -E revive -E unconvert -E unparam)
fi
golangci-lint "${lint_args[@]}"

echo "shellcheck..."
mapfile -t shell_files < <(find scripts -type f -name "*.sh" | sort)
if [[ -f "install.sh" ]]; then
  shell_files+=("install.sh")
fi
if (( ${#shell_files[@]} > 0 )); then
  shellcheck -S warning "${shell_files[@]}"
fi

echo "actionlint..."
actionlint

if [[ "${RUN_VULNCHECK}" == "1" ]]; then
  echo "security gate..."
  SECURITY_TARGET="${LINT_TARGET}" SEVERITY_THRESHOLD="${SEVERITY_THRESHOLD}" SECURITY_SUMMARY_FILE="security-summary.json" \
    ./scripts/test/security-gate.sh
else
  echo "Skipping security gate (RUN_VULNCHECK=${RUN_VULNCHECK})"
fi

echo "Quality checks passed."
