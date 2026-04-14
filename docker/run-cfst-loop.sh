#!/bin/sh
set -eu

INTERVAL="${CFST_INTERVAL_SECONDS:-300}"
if ! echo "${INTERVAL}" | grep -Eq '^[0-9]+$'; then
  echo "[cfst-loop] CFST_INTERVAL_SECONDS must be an integer, got: ${INTERVAL}" >&2
  exit 1
fi
if [ "${INTERVAL}" -le 0 ]; then
  echo "[cfst-loop] CFST_INTERVAL_SECONDS must be > 0, got: ${INTERVAL}" >&2
  exit 1
fi

CFST_ARGS_VALUE="${CFST_ARGS:--f ip.txt}"

echo "[cfst-loop] start interval=${INTERVAL}s args=${CFST_ARGS_VALUE}"

while true; do
  echo "[cfst-loop] running at $(date -Iseconds)"
  # shellcheck disable=SC2086
  /usr/local/bin/cfst ${CFST_ARGS_VALUE} || echo "[cfst-loop] cfst exited with non-zero status, continue next round"
  echo "[cfst-loop] sleep ${INTERVAL}s"
  sleep "${INTERVAL}"
done
