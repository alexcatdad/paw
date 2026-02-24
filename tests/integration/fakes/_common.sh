#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="${FAKE_TOOL_LOG:-/tmp/paw-fake-tools.log}"
mkdir -p "$(dirname "${LOG_FILE}")"
echo "${0##*/} $*" >> "${LOG_FILE}"
