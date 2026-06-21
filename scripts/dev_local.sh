#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-3000}"
FRONTEND_PORT="${FRONTEND_PORT:-8080}"
SERVER_URL="${SERVER_URL:-ws://localhost:${PORT}}"
OPEN_BROWSER="${OPEN_BROWSER:-1}"
FRONTEND_URL="http://localhost:${FRONTEND_PORT}/?server=${SERVER_URL}"

backend_pid=""
frontend_pid=""

cleanup() {
  echo
  echo "Stopping local development servers..."
  if [[ -n "${backend_pid}" ]] && kill -0 "${backend_pid}" 2>/dev/null; then
    kill "${backend_pid}" 2>/dev/null || true
    wait "${backend_pid}" 2>/dev/null || true
  fi
  if [[ -n "${frontend_pid}" ]] && kill -0 "${frontend_pid}" 2>/dev/null; then
    kill "${frontend_pid}" 2>/dev/null || true
    wait "${frontend_pid}" 2>/dev/null || true
  fi
}

shutdown() {
  trap - EXIT INT TERM
  cleanup
  exit 0
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

trap cleanup EXIT
trap shutdown INT TERM

require_command go
require_command python3

# shellcheck source=port_check.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/port_check.sh"

cd "${ROOT_DIR}"

require_port_available "${PORT}" "backend" "./scripts/dev_local.sh"
require_port_available "${FRONTEND_PORT}" "frontend" "./scripts/dev_local.sh"

echo "Starting Yumbo backend on ${SERVER_URL}..."
PORT="${PORT}" go run ./backend &
backend_pid="$!"

echo "Starting frontend on http://localhost:${FRONTEND_PORT}..."
python3 -m http.server "${FRONTEND_PORT}" --directory frontend >/dev/null 2>&1 &
frontend_pid="$!"

sleep 1

if ! kill -0 "${backend_pid}" 2>/dev/null; then
  echo "Backend failed to start on port ${PORT}." >&2
  exit 1
fi

if ! kill -0 "${frontend_pid}" 2>/dev/null; then
  echo "Frontend failed to start on port ${FRONTEND_PORT}." >&2
  exit 1
fi

echo
echo "Local development is running:"
echo "  Backend:  ${SERVER_URL}"
echo "  Frontend: ${FRONTEND_URL}"
echo
echo "Press Ctrl-C to stop both servers."

if [[ "${OPEN_BROWSER}" == "1" ]] && command -v open >/dev/null 2>&1; then
  open "${FRONTEND_URL}" >/dev/null 2>&1 || true
fi

while true; do
  if ! kill -0 "${backend_pid}" 2>/dev/null; then
    echo "Backend stopped unexpectedly."
    exit 1
  fi
  if ! kill -0 "${frontend_pid}" 2>/dev/null; then
    echo "Frontend server stopped unexpectedly."
    exit 1
  fi
  sleep 1
done
