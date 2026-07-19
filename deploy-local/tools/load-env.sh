#!/usr/bin/env bash

# Load the single Browser Agent local environment and export it to child commands.

if [[ -n "${BROWSER_AGENT_ENV_FILE:-}" && -f "${BROWSER_AGENT_ENV_FILE}" ]]; then
  ENV_FILE="${BROWSER_AGENT_ENV_FILE}"
elif [[ -f "${ROOT_DIR}/deploy-local/.env.browser-agent" ]]; then
  ENV_FILE="${ROOT_DIR}/deploy-local/.env.browser-agent"
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

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-browser_agent}"
