#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

LINT_TIMEOUT="${LINT_TIMEOUT:-5m}"
LINT_TARGET="${LINT_TARGET:-./...}"
read -r -a lint_targets <<< "${LINT_TARGET}"

resolve_bin() {
  local name="$1"
  local fallback="$2"
  local found
  found="$(command -v "${name}" || true)"
  if [[ -n "${found}" ]]; then
    printf "%s" "${found}"
    return 0
  fi
  if [[ -x "${fallback}" ]]; then
    printf "%s" "${fallback}"
    return 0
  fi
  echo "${name} is required" >&2
  exit 1
}

go_bin="$(resolve_bin go /usr/local/go/bin/go)"
goimports_bin="$(resolve_bin goimports /usr/local/bin/goimports)"
golangci_lint_bin="$(resolve_bin golangci-lint /usr/local/bin/golangci-lint)"

gofmt_bin="$(command -v gofmt || true)"
if [[ -z "${gofmt_bin}" ]]; then
  gofmt_bin="$("${go_bin}" env GOROOT)/bin/gofmt"
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
  "${goimports_bin}" -w "${go_files[@]}"
fi

echo "Applying safe golangci-lint autofixes..."
"${golangci_lint_bin}" run --fix --timeout "${LINT_TIMEOUT}" --disable-all -E misspell "${lint_targets[@]}"

echo "Quality autofix complete."
