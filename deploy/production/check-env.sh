#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${PRODUCTION_ENV_FILE:-${ROOT_DIR}/deploy/production/.env}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "production environment file not found: ${ENV_FILE}" >&2
  exit 2
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

required=(
  IMAGE_REPOSITORY_PREFIX IMAGE_TAG PUBLIC_WEB_BASE_URL
  DB_HOST DB_PORT DB_USER DB_NAME DB_SSL_MODE
  REDIS_ADDR REDIS_HOST REDIS_PORT REDIS_REQUIRED REDIS_TLS_ENABLED REDIS_TLS_SERVER_NAME
  R2_ACCOUNT_ID R2_BUCKET R2_PREFIX
  JWT_SECRET_FILE INTERNAL_API_SECRET_FILE MYSQL_DSN_FILE DB_PASSWORD_FILE REDIS_PASSWORD_FILE
  R2_ACCESS_KEY_ID_FILE R2_SECRET_ACCESS_KEY_FILE
)
for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required production variable: ${name}" >&2
    exit 2
  fi
done

if [[ "${IMAGE_TAG}" == "latest" || "${IMAGE_TAG}" == "main" || ! "${IMAGE_TAG}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "IMAGE_TAG must be a full immutable 40-character commit SHA" >&2
  exit 2
fi
if [[ ! "${PUBLIC_WEB_BASE_URL}" =~ ^https:// ]]; then
  echo "PUBLIC_WEB_BASE_URL must use https://" >&2
  exit 2
fi
if [[ "${DB_SSL_MODE}" != "VERIFY_CA" && "${DB_SSL_MODE}" != "VERIFY_IDENTITY" ]]; then
  echo "production DB_SSL_MODE must be VERIFY_CA or VERIFY_IDENTITY" >&2
  exit 2
fi
if [[ "${REDIS_REQUIRED,,}" != "true" || "${REDIS_TLS_ENABLED,,}" != "true" ]]; then
  echo "production Redis must be required and use TLS" >&2
  exit 2
fi
if [[ ! "${R2_ACCOUNT_ID}" =~ ^[0-9A-Fa-f]{32}$ ]]; then
	echo "R2_ACCOUNT_ID must be 32 hexadecimal characters" >&2
	exit 2
fi
if [[ "${R2_ACCOUNT_ID,,}" == "0123456789abcdef0123456789abcdef" ]]; then
	echo "R2_ACCOUNT_ID still contains the template value" >&2
	exit 2
fi
if [[ ! "${R2_BUCKET}" =~ ^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$ ]]; then
  echo "R2_BUCKET must be 3-63 lowercase letters, numbers, or hyphens and cannot start or end with a hyphen" >&2
  exit 2
fi
if [[ "${R2_PREFIX}" == /* || "${R2_PREFIX}" == *".."* || "${R2_PREFIX}" == *\\* ]]; then
  echo "R2_PREFIX must be a relative object key prefix without '..' or backslashes" >&2
  exit 2
fi

resolve_path() {
  local path="$1"
  if [[ "${path}" = /* ]]; then
    printf '%s' "${path}"
  else
    printf '%s' "$(cd "$(dirname "${ENV_FILE}")" && pwd)/${path#./}"
  fi
}

for name in JWT_SECRET_FILE INTERNAL_API_SECRET_FILE MYSQL_DSN_FILE DB_PASSWORD_FILE REDIS_PASSWORD_FILE R2_ACCESS_KEY_ID_FILE R2_SECRET_ACCESS_KEY_FILE; do
  path="$(resolve_path "${!name}")"
  if [[ ! -f "${path}" ]]; then
    echo "secret file does not exist for ${name}: ${path}" >&2
    exit 2
  fi
done

jwt_file="$(resolve_path "${JWT_SECRET_FILE}")"
internal_file="$(resolve_path "${INTERNAL_API_SECRET_FILE}")"
mysql_dsn_file="$(resolve_path "${MYSQL_DSN_FILE}")"
redis_password_file="$(resolve_path "${REDIS_PASSWORD_FILE}")"
r2_access_key_id_file="$(resolve_path "${R2_ACCESS_KEY_ID_FILE}")"
r2_secret_access_key_file="$(resolve_path "${R2_SECRET_ACCESS_KEY_FILE}")"
if (( $(wc -c < "${jwt_file}") < 32 )); then
  echo "JWT secret must contain at least 32 bytes" >&2
  exit 2
fi
if (( $(wc -c < "${internal_file}") < 32 )); then
  echo "internal API secret must contain at least 32 bytes" >&2
  exit 2
fi
if [[ ! "$(tr -d '\r\n' < "${mysql_dsn_file}")" =~ [\?\&]tls=true([\&].*)?$ ]]; then
  echo "MYSQL_DSN must enable verified TLS with tls=true" >&2
  exit 2
fi
if [[ ! -s "${redis_password_file}" ]]; then
  echo "Redis password must not be empty in production" >&2
  exit 2
fi
if (( $(wc -c < "${r2_access_key_id_file}") < 16 )); then
  echo "R2 access key ID must contain at least 16 bytes" >&2
  exit 2
fi
if (( $(wc -c < "${r2_secret_access_key_file}") < 32 )); then
  echo "R2 secret access key must contain at least 32 bytes" >&2
  exit 2
fi

if grep -Eqi 'replace|change[-_ ]?me|example|prod-secret' "${jwt_file}" "${internal_file}" "${r2_access_key_id_file}" "${r2_secret_access_key_file}"; then
  echo "placeholder secret detected" >&2
  exit 2
fi

echo "production environment validation passed"
