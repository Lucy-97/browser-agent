#!/usr/bin/env bash

# This script loads the appropriate .env file based on QIYUAN_ENV or QIYUAN_ENV_FILE
# It exports the variables so they are available to subsequent commands in the shell.

if [[ -n "${QIYUAN_ENV_FILE:-}" && -f "${QIYUAN_ENV_FILE}" ]]; then
  ENV_FILE="${QIYUAN_ENV_FILE}"
elif [[ -n "${QIYUAN_ENV:-}" && -f "${ROOT_DIR}/deploy-local/.env.${QIYUAN_ENV}" ]]; then
  ENV_FILE="${ROOT_DIR}/deploy-local/.env.${QIYUAN_ENV}"
elif [[ -f "${ROOT_DIR}/deploy-local/.env" ]]; then
  ENV_FILE="${ROOT_DIR}/deploy-local/.env"
else
  # Fallback: if no .env exists, just try to proceed without breaking
  ENV_FILE=""
fi

if [[ -n "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
  echo "Loaded environment from: ${ENV_FILE}"
fi

# Ensure default variables are set if not present.
# Keep feature/browser-agent and feature/qiyuan isolated by defaulting the
# compose prefix from QIYUAN_ENV when the environment file does not specify one.
if [[ -z "${COMPOSE_PROJECT_NAME:-}" ]]; then
  case "${QIYUAN_ENV:-}" in
    browser-agent|browser_agent)
      export COMPOSE_PROJECT_NAME="browser_agent"
      ;;
    qiyuan)
      export COMPOSE_PROJECT_NAME="qiyuan"
      ;;
    *)
      export COMPOSE_PROJECT_NAME="qiyuan"
      ;;
  esac
fi
