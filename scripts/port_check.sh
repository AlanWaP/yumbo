#!/usr/bin/env bash

port_in_use() {
  local port="$1"

  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
    return
  fi

  python3 - "$port" <<'PY'
import socket
import sys

port = int(sys.argv[1])
with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    try:
        sock.bind(("127.0.0.1", port))
    except OSError:
        raise SystemExit(0)
raise SystemExit(1)
PY
}

print_port_usage() {
  local port="$1"

  if ! command -v lsof >/dev/null 2>&1; then
    return
  fi

  lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | tail -n +2 | while read -r line; do
    local pid command_name
    command_name="$(echo "$line" | awk '{print $1}')"
    pid="$(echo "$line" | awk '{print $2}')"
    echo "  kill ${pid}    # ${command_name}" >&2
  done
}

require_port_available() {
  local port="$1"
  local label="$2"
  local script_name="$3"

  if port_in_use "$port"; then
    echo "Port ${port} is already in use (${label})." >&2
    print_port_usage "$port"
    echo "Or choose another port: PORT=3001 ${script_name}" >&2
    exit 1
  fi
}
