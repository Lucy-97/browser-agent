#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_PATH="${ROOT_DIR}/deploy-local/tools/run-gateway-host-local.sh"
RUN_DIR="${ROOT_DIR}/deploy-local/run"
LOG_DIR="${ROOT_DIR}/deploy-local/logs"
source "${ROOT_DIR}/deploy-local/tools/load-env.sh"
PREFIX="${COMPOSE_PROJECT_NAME:-browser_agent}"
LOG_FILE="${LOG_DIR}/${PREFIX}-backend-gateway.log"
SESSION_NAME="${PREFIX}-backend-gateway-local"
ACTION="${1:-start}"

mkdir -p "${RUN_DIR}" "${LOG_DIR}"

ensure_tmux() {
  if ! command -v tmux >/dev/null 2>&1; then
    echo "tmux is required. Install it first, for example: brew install tmux" >&2
    exit 1
  fi
}

session_exists() {
  tmux has-session -t "${SESSION_NAME}" 2>/dev/null
}

run_gateway() {
  export GATEWAY_PORT="${GATEWAY_PORT:-29000}"
  export API_SERVICE_URL="${API_SERVICE_URL:-http://127.0.0.1:29001}"
  export INTERNAL_API_SECRET="${INTERNAL_API_SECRET:-${INTERNAL_SECRET:-}}"
  cd "${ROOT_DIR}/backend-gateway"
  exec go run ./cmd/gateway >>"${LOG_FILE}" 2>&1
}

start_gateway() {
  ensure_tmux
  if session_exists; then
    echo "backend-gateway already running in tmux session: ${SESSION_NAME}"
    echo "URL: http://localhost:${GATEWAY_PORT:-29000}"
    return
  fi
  tmux new-session -d -s "${SESSION_NAME}" "BROWSER_AGENT_ENV_FILE='${BROWSER_AGENT_ENV_FILE:-}' BROWSER_AGENT_GATEWAY_TMUX_CHILD=1 bash '${SCRIPT_PATH}'"
  echo "backend-gateway started in tmux session: ${SESSION_NAME}"
  echo "URL: http://localhost:${GATEWAY_PORT:-29000}"
  echo "log: ${LOG_FILE}"
}

stop_gateway() {
  ensure_tmux
  if session_exists; then
    tmux kill-session -t "${SESSION_NAME}"
    echo "backend-gateway stopped: ${SESSION_NAME}"
  else
    echo "backend-gateway not running: ${SESSION_NAME}"
  fi
}

status_gateway() {
  ensure_tmux
  if session_exists; then
    echo "backend-gateway running in tmux session: ${SESSION_NAME}"
    echo "URL: http://localhost:${GATEWAY_PORT:-29000}"
    echo "log: ${LOG_FILE}"
  else
    echo "backend-gateway stopped"
  fi
}

if [[ "${BROWSER_AGENT_GATEWAY_TMUX_CHILD:-}" == "1" ]]; then
  run_gateway
  exit 0
fi

case "${ACTION}" in
  start)
    start_gateway
    ;;
  stop)
    stop_gateway
    ;;
  restart)
    stop_gateway
    start_gateway
    ;;
  status)
    status_gateway
    ;;
  logs)
    touch "${LOG_FILE}"
    tail -n "${2:-200}" "${LOG_FILE}"
    ;;
  *)
    echo "usage: $0 {start|stop|restart|status|logs [lines]}" >&2
    exit 2
    ;;
esac
