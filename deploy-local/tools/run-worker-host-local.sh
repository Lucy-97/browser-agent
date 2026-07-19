#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-start}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_PATH="$SCRIPT_DIR/$(basename "$0")"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$ROOT_DIR/deploy-local/tools/load-env.sh"
PREFIX="${COMPOSE_PROJECT_NAME:-browser_agent}"
WORKER_DIR="$ROOT_DIR/worker/local-cli"
RUN_DIR="$ROOT_DIR/deploy-local/run"
LOG_DIR="$ROOT_DIR/deploy-local/logs"
LOG_FILE="$LOG_DIR/${PREFIX}-worker-local.log"
SESSION_NAME="${PREFIX}-worker-local"

mkdir -p "$RUN_DIR" "$LOG_DIR"

usage() {
  cat <<'USAGE'
Usage: bash deploy-local/tools/run-worker-host-local.sh [init|pair|doctor|start|restart|stop|status|logs|once]

Commands:
  init     Create local Worker config. Defaults to http://127.0.0.1:29001.
  pair     Pair this machine with the platform and store the device token locally.
  doctor   Check Python, config, Playwright and browser directories.
  start    Run Worker loop in tmux.
  once     Run one poll cycle in the current shell.
  restart  Restart tmux Worker loop.
  stop     Stop tmux Worker loop.
  status   Show tmux session and Worker local status.
  logs     Show Worker local logs.
USAGE
}

ensure_tmux() {
  if ! command -v tmux >/dev/null 2>&1; then
    echo "tmux is required. Install it first, for example: brew install tmux" >&2
    exit 1
  fi
}

load_env() {

  export PYTHONPATH="$WORKER_DIR"
  export BROWSER_AGENT_WORKER_SERVER="${WORKER_SERVER_URL:-${ADMIN_API_BASE_URL:-http://127.0.0.1:29001}}"
  export BROWSER_AGENT_WORKER_DISPLAY_NAME="${WORKER_DISPLAY_NAME:-Browser Agent Local Worker}"
  export BROWSER_AGENT_WORKER_ENABLED_PRODUCTS="${BROWSER_AGENT_WORKER_ENABLED_PRODUCTS:-core,browser_agent,social,weixin}"
}

worker_cli() {
  load_env
  python3 -m qiyuan_worker "$@"
}

session_exists() {
  tmux has-session -t "$SESSION_NAME" 2>/dev/null
}

run_loop() {
  load_env
  exec python3 -u -m qiyuan_worker run >>"$LOG_FILE" 2>&1
}

start_session() {
  ensure_tmux
  if session_exists; then
    echo "Worker is already running in tmux session: $SESSION_NAME"
    echo "Attach logs: tmux attach -t $SESSION_NAME"
    echo "log: $LOG_FILE"
    return
  fi
  tmux new-session -d -s "$SESSION_NAME" "BROWSER_AGENT_ENV_FILE='${BROWSER_AGENT_ENV_FILE:-}' BROWSER_AGENT_WORKER_TMUX_CHILD=1 bash '$SCRIPT_PATH'"
  echo "Worker started in tmux session: $SESSION_NAME"
  echo "Attach logs: tmux attach -t $SESSION_NAME"
  echo "log: $LOG_FILE"
}

stop_session() {
  ensure_tmux
  if session_exists; then
    tmux kill-session -t "$SESSION_NAME"
    echo "Worker stopped: $SESSION_NAME"
  else
    echo "Worker is not running: $SESSION_NAME"
  fi
}

status_session() {
  ensure_tmux
  if session_exists; then
    echo "Worker is running in tmux session: $SESSION_NAME"
    echo "Attach logs: tmux attach -t $SESSION_NAME"
    echo "log: $LOG_FILE"
  else
    echo "Worker is not running: $SESSION_NAME"
  fi
  worker_cli status || true
}

if [[ "${BROWSER_AGENT_WORKER_TMUX_CHILD:-}" == "1" ]]; then
  run_loop
  exit 0
fi

case "$ACTION" in
  init)
    load_env
    worker_cli init --server "$BROWSER_AGENT_WORKER_SERVER"
    ;;
  pair)
    load_env
    worker_cli pair --display-name "$BROWSER_AGENT_WORKER_DISPLAY_NAME"
    ;;
  doctor)
    worker_cli doctor
    ;;
  start)
    start_session
    ;;
  once)
    worker_cli run --once
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
  logs)
    touch "$LOG_FILE"
    tail -n "${2:-200}" "$LOG_FILE"
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
