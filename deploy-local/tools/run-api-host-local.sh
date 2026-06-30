#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_PATH="${ROOT_DIR}/deploy-local/tools/run-api-host-local.sh"
RUN_DIR="${ROOT_DIR}/deploy-local/run"
LOG_DIR="${ROOT_DIR}/deploy-local/logs"
ENV_FILE="${ROOT_DIR}/deploy-local/.env"
source "${ROOT_DIR}/deploy-local/tools/load-env.sh"
PREFIX="${COMPOSE_PROJECT_NAME:-qiyuan}"
PID_FILE="${RUN_DIR}/${PREFIX}-backend-api.pid"
LOG_FILE="${LOG_DIR}/${PREFIX}-backend-api.log"
SESSION_NAME="${PREFIX}-backend-api-local"
ACTION="${1:-start}"

mkdir -p "${RUN_DIR}" "${LOG_DIR}"

load_env() {
  export API_ADDR="${API_ADDR:-:28001}"
  export ARTIFACT_DIR="${ARTIFACT_DIR:-${ROOT_DIR}/deploy-local/artifacts}"
  if [[ "${ARTIFACT_DIR}" != /* ]]; then
    export ARTIFACT_DIR="${ROOT_DIR}/${ARTIFACT_DIR}"
  fi
}

ensure_tmux() {
  if ! command -v tmux >/dev/null 2>&1; then
    echo "tmux is required. Install it first, for example: brew install tmux" >&2
    exit 1
  fi
}

session_exists() {
  tmux has-session -t "${SESSION_NAME}" 2>/dev/null
}

run_api() {
  load_env
  cd "${ROOT_DIR}/backend-api"
  exec go run ./cmd/api >>"${LOG_FILE}" 2>&1
}

start_api() {
  ensure_tmux
  if session_exists; then
    echo "backend-api already running in tmux session: ${SESSION_NAME}"
    echo "log: ${LOG_FILE}"
    return
  fi
  rm -f "${PID_FILE}"
  tmux new-session -d -s "${SESSION_NAME}" "QIYUAN_ENV='${QIYUAN_ENV:-}' QIYUAN_API_TMUX_CHILD=1 bash '${SCRIPT_PATH}'"
  echo "backend-api started in tmux session: ${SESSION_NAME}"
  echo "log: ${LOG_FILE}"
}

stop_api() {
  ensure_tmux
  if session_exists; then
    tmux kill-session -t "${SESSION_NAME}"
    echo "backend-api stopped: ${SESSION_NAME}"
  else
    echo "backend-api not running: ${SESSION_NAME}"
  fi
  rm -f "${PID_FILE}"
}

status_api() {
  ensure_tmux
  if session_exists; then
    echo "backend-api running in tmux session: ${SESSION_NAME}"
    echo "log: ${LOG_FILE}"
  else
    echo "backend-api stopped"
  fi
}

if [[ "${QIYUAN_API_TMUX_CHILD:-}" == "1" ]]; then
  run_api
  exit 0
fi

case "${ACTION}" in
  start)
    start_api
    ;;
  stop)
    stop_api
    ;;
  restart)
    stop_api
    start_api
    ;;
  status)
    status_api
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
