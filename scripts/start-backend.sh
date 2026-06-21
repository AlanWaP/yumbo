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
FRONTEND_URL="${TUNNEL_URL%/}/"

echo
echo "Open this URL to play:"
echo "${FRONTEND_URL}?server=${WS_URL}"
echo
if [[ -n "$PAGES_URL" ]]; then
  echo "GitHub Pages URL (when Pages is enabled):"
  echo "${PAGES_URL}?server=${WS_URL}"
  echo
fi
echo "Backend URL: ws://localhost:${PORT}"
echo "Tunnel URL:  ${TUNNEL_URL}"
echo "Press Ctrl+C to stop the backend and tunnel."

wait -n "$BACKEND_PID" "$TUNNEL_PID"
