#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_DIR="${ROOT_DIR}/deploy/production"
ENV_FILE="${PRODUCTION_ENV_FILE:-${DEPLOY_DIR}/.env}"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${DEPLOY_DIR}/compose.yaml")
ACTION="${1:-deploy}"

validate() {
  PRODUCTION_ENV_FILE="${ENV_FILE}" bash "${DEPLOY_DIR}/check-env.sh"
  "${COMPOSE[@]}" config --quiet
}

case "${ACTION}" in
  validate)
    validate
    ;;
  migrate)
    validate
    "${COMPOSE[@]}" --profile tools run --rm migration
    ;;
  deploy)
    validate
    "${COMPOSE[@]}" pull api gateway web
    "${COMPOSE[@]}" --profile tools run --rm migration
    "${COMPOSE[@]}" up -d --remove-orphans api gateway web
    "${COMPOSE[@]}" ps
    ;;
  status)
    "${COMPOSE[@]}" ps
    ;;
  logs)
    shift
    if (( $# == 0 )); then
      set -- api gateway web
    fi
    "${COMPOSE[@]}" logs --tail="${LOG_LINES:-200}" "$@"
    ;;
  *)
    echo "usage: $0 {validate|migrate|deploy|status|logs [service...]}" >&2
    exit 2
    ;;
esac
