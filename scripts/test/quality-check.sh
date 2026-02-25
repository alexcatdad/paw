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
golangci_lint_bin="$(resolve_bin golangci-lint /usr/local/bin/golangci-lint)"
shellcheck_bin="$(resolve_bin shellcheck /usr/bin/shellcheck)"
actionlint_bin="$(resolve_bin actionlint /usr/local/bin/actionlint)"

gofmt_bin="$(command -v gofmt || true)"
if [[ -z "${gofmt_bin}" ]]; then
  gofmt_bin="$("${go_bin}" env GOROOT)/bin/gofmt"
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
"${go_bin}" vet "${lint_targets[@]}"

echo "golangci-lint..."
lint_args=(run --timeout "${LINT_TIMEOUT}" "${lint_targets[@]}")
if [[ "${QUALITY_STAGE}" == "strict" ]]; then
  lint_args+=(-E errcheck -E gocritic -E revive -E unconvert -E unparam)
fi
"${golangci_lint_bin}" "${lint_args[@]}"

echo "shellcheck..."
mapfile -t shell_files < <(find scripts -type f -name "*.sh" | sort)
if [[ -f "install.sh" ]]; then
  shell_files+=("install.sh")
fi
if (( ${#shell_files[@]} > 0 )); then
  "${shellcheck_bin}" -S warning "${shell_files[@]}"
fi

echo "actionlint..."
"${actionlint_bin}"

if [[ "${RUN_VULNCHECK}" == "1" ]]; then
  echo "security gate..."
  SECURITY_TARGET="${LINT_TARGET}" SEVERITY_THRESHOLD="${SEVERITY_THRESHOLD}" SECURITY_SUMMARY_FILE="security-summary.json" \
    ./scripts/test/security-gate.sh
else
  echo "Skipping security gate (RUN_VULNCHECK=${RUN_VULNCHECK})"
fi

echo "Quality checks passed."
