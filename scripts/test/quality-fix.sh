#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

LINT_TIMEOUT="${LINT_TIMEOUT:-5m}"
LINT_TARGET="${LINT_TARGET:-./...}"
read -r -a lint_targets <<< "${LINT_TARGET}"

command -v goimports >/dev/null 2>&1 || { echo "goimports is required" >&2; exit 1; }
command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required" >&2; exit 1; }

gofmt_bin="$(command -v gofmt || true)"
if [[ -z "${gofmt_bin}" ]]; then
  gofmt_bin="$(go env GOROOT)/bin/gofmt"
fi
if [[ ! -x "${gofmt_bin}" ]]; then
  echo "gofmt is required" >&2
  exit 1
fi

echo "Applying gofmt..."
"${gofmt_bin}" -s -w .

echo "Applying goimports..."
mapfile -t go_files < <(git ls-files "*.go")
if (( ${#go_files[@]} > 0 )); then
  goimports -w "${go_files[@]}"
fi

echo "Applying safe golangci-lint autofixes..."
golangci-lint run --fix --timeout "${LINT_TIMEOUT}" --disable-all -E misspell "${lint_targets[@]}"

echo "Quality autofix complete."
