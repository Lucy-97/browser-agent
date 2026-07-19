#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-start}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_PATH="$SCRIPT_DIR/$(basename "$0")"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$ROOT_DIR/deploy-local/tools/load-env.sh"
PREFIX="${COMPOSE_PROJECT_NAME:-browser_agent}"
ADMIN_DIR="$ROOT_DIR/frontend-admin"
RUN_DIR="$ROOT_DIR/deploy-local/run"
PORT_FILE="$RUN_DIR/${PREFIX}-frontend-admin.port"
SESSION_NAME="${PREFIX}-admin-local"
DEFAULT_PORT="${ADMIN_PORT:-26174}"

usage() {
  echo "Usage: bash deploy-local/tools/run-admin-host-local.sh [start|restart|stop|status]"
}

ensure_tmux() {
  if ! command -v tmux >/dev/null 2>&1; then
    echo "tmux is required. Install it first, for example: brew install tmux" >&2
    exit 1
  fi
}

session_exists() {
  tmux has-session -t "$SESSION_NAME" 2>/dev/null
}

port_available() {
  local port="$1"
  ! lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
}

find_port() {
  local port="$1"
  while ! port_available "$port"; do
    port=$((port + 1))
  done
  echo "$port"
}

current_port() {
  if [[ -f "$PORT_FILE" ]]; then
    cat "$PORT_FILE"
  else
    echo "$DEFAULT_PORT"
  fi
}

run_dev() {

  export VITE_API_BASE_URL="${ADMIN_API_BASE_URL:-http://127.0.0.1:29001}"
  export VITE_API_PREFIX="/api"
  export VITE_ADMIN_API_TOKEN="${ADMIN_API_TOKEN:-}"
  local port="${BROWSER_AGENT_ADMIN_PORT:-$DEFAULT_PORT}"

  cd "$ADMIN_DIR"

  if [[ ! -x node_modules/.bin/next ]] || [[ package-lock.json -nt node_modules/.package-lock.local ]]; then
    npm install
    cp package-lock.json node_modules/.package-lock.local
  fi

  npm run dev -- -H 0.0.0.0 -p "$port"
}

start_session() {
  ensure_tmux
  if session_exists; then
    echo "Admin frontend is already running in tmux session: $SESSION_NAME"
    echo "URL: http://localhost:$(current_port)"
    echo "Attach logs: tmux attach -t $SESSION_NAME"
    return
  fi

  mkdir -p "$RUN_DIR"
  local port
  port="$(find_port "$DEFAULT_PORT")"
  echo "$port" >"$PORT_FILE"
  tmux new-session -d -s "$SESSION_NAME" "BROWSER_AGENT_ENV_FILE='${BROWSER_AGENT_ENV_FILE:-}' BROWSER_AGENT_TMUX_CHILD=1 BROWSER_AGENT_ADMIN_PORT='$port' bash '$SCRIPT_PATH'"
  echo "Admin frontend started in tmux session: $SESSION_NAME"
  echo "URL: http://localhost:$port"
  echo "Attach logs: tmux attach -t $SESSION_NAME"
}

stop_session() {
  ensure_tmux
  if session_exists; then
    tmux kill-session -t "$SESSION_NAME"
    echo "Admin frontend stopped: $SESSION_NAME"
  else
    echo "Admin frontend is not running: $SESSION_NAME"
  fi
  rm -f "$PORT_FILE"
}

status_session() {
  ensure_tmux
  if session_exists; then
    echo "Admin frontend is running in tmux session: $SESSION_NAME"
    echo "URL: http://localhost:$(current_port)"
    echo "Attach logs: tmux attach -t $SESSION_NAME"
  else
    echo "Admin frontend is not running: $SESSION_NAME"
  fi
}

if [[ "${BROWSER_AGENT_TMUX_CHILD:-}" == "1" ]]; then
  run_dev
  exit 0
fi

case "$ACTION" in
  start)
    start_session
    ;;
  restart)
    stop_session
    start_session
    ;;
  stop)
    stop_session
    ;;
  status)
    status_session
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
