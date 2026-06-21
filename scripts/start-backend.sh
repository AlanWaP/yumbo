#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PORT="${1:-${PORT:-3000}}"
PAGES_URL="${PAGES_URL:-https://AlanWaP.github.io/yumbo/}"
TUNNEL_LOG="$(mktemp "${TMPDIR:-/tmp}/yumbo-cloudflared.XXXXXX.log")"
BACKEND_PID=""
TUNNEL_PID=""

cd "$PROJECT_ROOT"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed or is not on PATH." >&2
  exit 1
fi

if ! command -v cloudflared >/dev/null 2>&1; then
  echo "cloudflared is not installed or is not on PATH." >&2
  echo "Install it with: brew install cloudflare/cloudflare/cloudflared" >&2
  exit 1
fi

# shellcheck source=port_check.sh
source "${SCRIPT_DIR}/port_check.sh"

cleanup() {
  if [[ -n "$TUNNEL_PID" ]] && kill -0 "$TUNNEL_PID" 2>/dev/null; then
    kill "$TUNNEL_PID" 2>/dev/null || true
  fi

  if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" 2>/dev/null; then
    kill "$BACKEND_PID" 2>/dev/null || true
  fi

  rm -f "$TUNNEL_LOG"
}

trap cleanup EXIT INT TERM

require_port_available "${PORT}" "backend" "./scripts/start-backend.sh"

echo "Starting Yumbo backend on ws://localhost:${PORT}"
PORT="$PORT" go run ./backend &
BACKEND_PID="$!"

echo "Starting Cloudflare Tunnel for http://localhost:${PORT}"
cloudflared tunnel --url "http://localhost:${PORT}" 2>&1 | tee "$TUNNEL_LOG" &
TUNNEL_PID="$!"

echo "Waiting for Cloudflare Tunnel URL..."

TUNNEL_URL=""
for _ in {1..60}; do
  TUNNEL_URL="$(grep -Eo 'https://[^ ]+\.trycloudflare\.com' "$TUNNEL_LOG" | tail -n 1 || true)"

  if [[ -n "$TUNNEL_URL" ]]; then
    break
  fi

  if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "Backend stopped before tunnel was ready." >&2
    if port_in_use "$PORT"; then
      echo "Port ${PORT} is already in use." >&2
      print_port_usage "$PORT"
      echo "Or choose another port: PORT=3001 ./scripts/start-backend.sh" >&2
    fi
    exit 1
  fi

  if ! kill -0 "$TUNNEL_PID" 2>/dev/null; then
    echo "Cloudflare Tunnel stopped before publishing a URL." >&2
    exit 1
  fi

  sleep 1
done

if [[ -z "$TUNNEL_URL" ]]; then
  echo "Timed out waiting for Cloudflare Tunnel URL." >&2
  exit 1
fi

WS_URL="${TUNNEL_URL/https:\/\//wss://}"

echo
echo "Open this URL to play:"
echo "${PAGES_URL}?server=${WS_URL}"
echo
echo "Backend URL: ws://localhost:${PORT}"
echo "Tunnel URL:  ${TUNNEL_URL}"
echo "Press Ctrl+C to stop the backend and tunnel."

wait -n "$BACKEND_PID" "$TUNNEL_PID"
