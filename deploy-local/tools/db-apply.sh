#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${ROOT_DIR}/deploy-local/tools/load-env.sh"
COMPOSE_FILE="$ROOT_DIR/deploy-local/docker-compose-infra.yaml"
MYSQL_SERVICE="mysql"
DATABASE="${QIYUAN_DB_NAME:-qiyuan}"
USER="${QIYUAN_DB_USER:-qiyuan}"
PASSWORD="${QIYUAN_DB_PASSWORD:-qiyuan}"

usage() {
  cat <<'USAGE'
Usage: bash deploy-local/tools/db-apply.sh [init|migrations|all]

Commands:
  init        Apply database/init.sql.
  migrations  Apply every database/migrations/*.sql in filename order.
  all         Apply init.sql, then migrations.
USAGE
}

wait_mysql() {
  for _ in $(seq 1 60); do
    if docker compose -f "$COMPOSE_FILE" exec -T "$MYSQL_SERVICE" mysqladmin ping -h 127.0.0.1 -u"$USER" -p"$PASSWORD" --silent >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "MySQL is not ready" >&2
  return 1
}

apply_file() {
  local file="$1"
  echo "Applying ${file#$ROOT_DIR/}"
  docker compose -f "$COMPOSE_FILE" exec -T "$MYSQL_SERVICE" mysql -u"$USER" -p"$PASSWORD" "$DATABASE" < "$file"
}

cmd="${1:-all}"
wait_mysql

case "$cmd" in
  init)
    apply_file "$ROOT_DIR/database/init.sql"
    ;;
  migrations)
    while IFS= read -r file; do
      apply_file "$file"
    done < <(find "$ROOT_DIR/database/migrations" -maxdepth 1 -type f -name '*.sql' | sort)
    ;;
  all)
    apply_file "$ROOT_DIR/database/init.sql"
    while IFS= read -r file; do
      apply_file "$file"
    done < <(find "$ROOT_DIR/database/migrations" -maxdepth 1 -type f -name '*.sql' | sort)
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
