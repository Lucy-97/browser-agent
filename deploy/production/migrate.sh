#!/usr/bin/env bash
set -euo pipefail

required=(DB_HOST DB_PORT DB_USER DB_NAME DB_PASSWORD_FILE)
for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required migration variable: ${name}" >&2
    exit 2
  fi
done

if [[ ! -r "${DB_PASSWORD_FILE}" ]]; then
  echo "database password file is not readable" >&2
  exit 2
fi

export MYSQL_PWD
MYSQL_PWD="$(tr -d '\r\n' < "${DB_PASSWORD_FILE}")"
ssl_mode="${DB_SSL_MODE:-VERIFY_IDENTITY}"
mysql_args=(
  --host="${DB_HOST}"
  --port="${DB_PORT}"
  --user="${DB_USER}"
  --database="${DB_NAME}"
  --ssl-mode="${ssl_mode}"
  --default-character-set=utf8mb4
  --connect-timeout=10
)

for attempt in $(seq 1 30); do
  if mysql "${mysql_args[@]}" --execute="SELECT 1" >/dev/null 2>&1; then
    break
  fi
  if [[ "${attempt}" == "30" ]]; then
    echo "database did not become ready" >&2
    exit 1
  fi
  sleep 2
done

echo "applying database/init.sql"
mysql "${mysql_args[@]}" < /schema/init.sql

while IFS= read -r migration; do
  echo "applying ${migration#/schema/}"
  mysql "${mysql_args[@]}" < "${migration}"
done < <(find /schema/migrations -maxdepth 1 -type f -name '*.sql' | sort)

echo "database migrations completed"
