#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-start}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_PATH="$SCRIPT_DIR/$(basename "$0")"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$ROOT_DIR/deploy-local/tools/load-env.sh"
PREFIX="${COMPOSE_PROJECT_NAME:-qiyuan}"
WEB_DIR="$ROOT_DIR/frontend-web"
RUN_DIR="$ROOT_DIR/deploy-local/run"
PORT_FILE="$RUN_DIR/${PREFIX}-frontend-web.port"
SESSION_NAME="${PREFIX}-web-local"
DEFAULT_PORT="${WEB_PORT:-23001}"

usage() {
  echo "Usage: bash deploy-local/tools/run-web-host-local.sh [start|restart|stop|status]"
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

run_web() {
  cd "$WEB_DIR"
  export PORT="${QIYUAN_WEB_PORT:-$DEFAULT_PORT}"
  exec npm run dev -- -p "$PORT"
}

start_session() {
  ensure_tmux
  if session_exists; then
    echo "Web frontend is already running in tmux session: $SESSION_NAME"
    echo "URL: http://localhost:$(current_port)"
    echo "Attach logs: tmux attach -t $SESSION_NAME"
    return
  fi

  mkdir -p "$RUN_DIR"
  local port
  port="$(find_port "$DEFAULT_PORT")"
  echo "$port" >"$PORT_FILE"
  tmux new-session -d -s "$SESSION_NAME" "QIYUAN_ENV='${QIYUAN_ENV:-}' QIYUAN_TMUX_CHILD=1 QIYUAN_WEB_PORT='$port' bash '$SCRIPT_PATH'"
  echo "Web frontend started in tmux session: $SESSION_NAME"
  echo "URL: http://localhost:$port"
  echo "Attach logs: tmux attach -t $SESSION_NAME"
}

stop_session() {
  ensure_tmux
  if session_exists; then
    tmux kill-session -t "$SESSION_NAME"
    echo "Web frontend stopped: $SESSION_NAME"
  else
    echo "Web frontend is not running: $SESSION_NAME"
  fi
  rm -f "$PORT_FILE"
}

status_session() {
  ensure_tmux
  if session_exists; then
    echo "Web frontend is running in tmux session: $SESSION_NAME"
    echo "URL: http://localhost:$(current_port)"
    echo "Attach logs: tmux attach -t $SESSION_NAME"
  else
    echo "Web frontend is not running: $SESSION_NAME"
  fi
}

if [[ "${QIYUAN_TMUX_CHILD:-}" == "1" ]]; then
  run_web
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
