#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${ROOT_DIR}/deploy-local/tools/load-env.sh"
COMPOSE_FILE="$ROOT_DIR/deploy-local/docker-compose-infra.yaml"

usage() {
  cat <<'USAGE'
Usage: bash deploy-local/tools/run-infra-local.sh [start|stop|restart|status|logs]

Commands:
  start    Start local MySQL and Redis.
  stop     Stop local MySQL and Redis.
  restart  Restart local MySQL and Redis.
  status   Show container status.
  logs     Follow infra logs.
USAGE
}

cmd="${1:-start}"

case "$cmd" in
  start)
    docker compose -f "$COMPOSE_FILE" up -d mysql redis
    docker compose -f "$COMPOSE_FILE" ps
    ;;
  stop)
    docker compose -f "$COMPOSE_FILE" stop mysql redis
    ;;
  restart)
    docker compose -f "$COMPOSE_FILE" restart mysql redis
    docker compose -f "$COMPOSE_FILE" ps
    ;;
  status)
    docker compose -f "$COMPOSE_FILE" ps
    ;;
  logs)
    docker compose -f "$COMPOSE_FILE" logs -f mysql redis
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
