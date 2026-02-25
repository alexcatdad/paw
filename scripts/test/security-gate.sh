#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

SEVERITY_THRESHOLD="${SEVERITY_THRESHOLD:-high}"
SUMMARY_FILE="${SECURITY_SUMMARY_FILE:-security-summary.json}"
TARGET="${SECURITY_TARGET:-./...}"
read -r -a security_targets <<< "${TARGET}"

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
  echo "${name} is required for security-gate.sh" >&2
  exit 1
}

jq_bin="$(resolve_bin jq /usr/bin/jq)"
govulncheck_bin="$(resolve_bin govulncheck /usr/local/bin/govulncheck)"
gosec_bin="$(resolve_bin gosec /usr/local/bin/gosec)"
path_override="$(dirname "${govulncheck_bin}"):$(dirname "${gosec_bin}"):${PATH:-/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin}"
export PATH="${path_override}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

govuln_json="${tmp_dir}/govuln.jsonl"
gosec_json="${tmp_dir}/gosec.json"

govuln_status=0
"${govulncheck_bin}" -json "${security_targets[@]}" >"${govuln_json}" 2>"${tmp_dir}/govuln.stderr" || govuln_status=$?
if [[ ${govuln_status} -ne 0 && ! -s "${govuln_json}" ]]; then
  echo "govulncheck failed to execute cleanly:" >&2
  cat "${tmp_dir}/govuln.stderr" >&2 || true
  exit 1
fi

gosec_status=0
"${gosec_bin}" -fmt=json -no-fail "${security_targets[@]}" >"${gosec_json}" 2>"${tmp_dir}/gosec.stderr" || gosec_status=$?
if [[ ${gosec_status} -ne 0 && ! -s "${gosec_json}" ]]; then
  echo "gosec failed to execute cleanly:" >&2
  cat "${tmp_dir}/gosec.stderr" >&2 || true
  exit 1
fi

govuln_critical=$(
  "${jq_bin}" -s '[.[] | select(.osv != null) | (.osv.severity // [])[]? | .score? | tonumber? | select(. >= 9)] | length' "${govuln_json}"
)
govuln_high=$(
  "${jq_bin}" -s '[.[] | select(.osv != null) | (.osv.severity // [])[]? | .score? | tonumber? | select(. >= 7 and . < 9)] | length' "${govuln_json}"
)
govuln_medium_low=$(
  "${jq_bin}" -s '[.[] | select(.osv != null) | (.osv.severity // [])[]? | .score? | tonumber? | select(. < 7)] | length' "${govuln_json}"
)
govuln_unknown=$(
  "${jq_bin}" -s '[.[] | select(.osv != null) | select((.osv.severity // []) | length == 0)] | length' "${govuln_json}"
)

gosec_critical=$(
  "${jq_bin}" '[.Issues[]? | select((.severity // "" | ascii_upcase) == "CRITICAL")] | length' "${gosec_json}"
)
gosec_high=$(
  "${jq_bin}" '[.Issues[]? | select((.severity // "" | ascii_upcase) == "HIGH")] | length' "${gosec_json}"
)
gosec_medium_low=$(
  "${jq_bin}" '[.Issues[]? | select((.severity // "" | ascii_upcase) == "MEDIUM" or (.severity // "" | ascii_upcase) == "LOW")] | length' "${gosec_json}"
)

"${jq_bin}" -n \
  --arg threshold "${SEVERITY_THRESHOLD}" \
  --argjson govuln_critical "${govuln_critical}" \
  --argjson govuln_high "${govuln_high}" \
  --argjson govuln_medium_low "${govuln_medium_low}" \
  --argjson govuln_unknown "${govuln_unknown}" \
  --argjson gosec_critical "${gosec_critical}" \
  --argjson gosec_high "${gosec_high}" \
  --argjson gosec_medium_low "${gosec_medium_low}" \
  '{
    threshold: $threshold,
    govulncheck: {
      critical: $govuln_critical,
      high: $govuln_high,
      medium_low: $govuln_medium_low,
      unknown_severity: $govuln_unknown
    },
    gosec: {
      critical: $gosec_critical,
      high: $gosec_high,
      medium_low: $gosec_medium_low
    }
  }' > "${SUMMARY_FILE}"

echo "Security summary:"
cat "${SUMMARY_FILE}"

if [[ "${SEVERITY_THRESHOLD}" == "high" ]]; then
  if (( govuln_critical + govuln_high + gosec_critical + gosec_high > 0 )); then
    echo "Blocking security findings detected at HIGH/CRITICAL severity." >&2
    exit 1
  fi
else
  if (( govuln_critical + gosec_critical > 0 )); then
    echo "Blocking security findings detected at CRITICAL severity." >&2
    exit 1
  fi
fi

if (( govuln_medium_low + govuln_unknown + gosec_medium_low > 0 )); then
  echo "Security findings below blocking threshold detected (warning only)." >&2
fi

echo "Security gate passed."
