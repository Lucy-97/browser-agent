#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:38001}"
LOG_DIR="${ROOT_DIR}/deploy-local/logs"
LOG_FILE="${LOG_DIR}/backend-api-mysql-smoke.log"
PID_FILE="${ROOT_DIR}/deploy-local/run/backend-api-mysql-smoke.pid"

mkdir -p "${LOG_DIR}" "$(dirname "${PID_FILE}")"

cleanup() {
  if [[ -f "${PID_FILE}" ]] && kill -0 "$(cat "${PID_FILE}")" 2>/dev/null; then
    kill "$(cat "${PID_FILE}")" || true
  fi
  rm -f "${PID_FILE}"
}
trap cleanup EXIT

bash "${ROOT_DIR}/deploy-local/tools/run-infra-local.sh" start >/dev/null
bash "${ROOT_DIR}/deploy-local/tools/db-apply.sh" all >/dev/null

(
  cd "${ROOT_DIR}/backend-api"
  export API_ADDR=":38001"
  export MYSQL_DSN="qiyuan:qiyuan@tcp(127.0.0.1:23307)/qiyuan?parseTime=true&charset=utf8mb4&loc=Local"
  export MYSQL_MAX_OPEN_CONNS="10"
  export MYSQL_MAX_IDLE_CONNS="5"
  export REDIS_ADDR="127.0.0.1:26380"
  export REDIS_DB="0"
  export ARTIFACT_DIR="${ROOT_DIR}/deploy-local/artifacts"
  export INTERNAL_SECRET="mysql-smoke-internal-secret"
  exec go run ./cmd/api
) >>"${LOG_FILE}" 2>&1 &
echo "$!" >"${PID_FILE}"

for _ in $(seq 1 60); do
  if curl -fsS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "${API_BASE_URL}/healthz" >/dev/null

pairing_json="$(curl -fsS -X POST "${API_BASE_URL}/worker/devices/pairing" \
  -H 'Content-Type: application/json' \
  -d '{"worker_version":"0.1.0","platform":"darwin-arm64","hostname_hash":"sha256:mysql-smoke","display_name":"mysql-smoke-worker"}')"

pairing_id="$(printf '%s' "${pairing_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["pairing_id"])')"
approved_json="$(curl -fsS "${API_BASE_URL}/worker/devices/pairing/${pairing_id}")"
device_id="$(printf '%s' "${approved_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["device"]["id"])')"
device_token="$(printf '%s' "${approved_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["device_token"])')"

curl -fsS -X POST "${API_BASE_URL}/worker/devices/${device_id}/heartbeat" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"worker_version":"0.1.0","status":"idle","capabilities":["adapter.mock.echo"],"metrics":{"pending_upload_count":0}}' >/dev/null

priority="$(date +%s)"
job_create_json="$(curl -fsS -X POST "${API_BASE_URL}/admin/automation/jobs" \
  -H 'Content-Type: application/json' \
  -d "{\"job_type\":\"generic.browser.script\",\"adapter\":\"mock.echo\",\"target\":{\"allowed_domains\":[\"example.com\"]},\"input\":{\"message\":\"mysql\"},\"policy\":{},\"priority\":${priority}}")"
created_job_id="$(printf '%s' "${job_create_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])')"

job_json="$(curl -fsS "${API_BASE_URL}/worker/automation/jobs/next" -H "Authorization: Bearer ${device_token}")"
run_id="$(printf '%s' "${job_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["run_id"])')"
job_id="$(printf '%s' "${job_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])')"

if [[ -z "${job_id}" || -z "${run_id}" ]]; then
  echo "claimed job is missing job_id or run_id: created=${created_job_id}" >&2
  exit 1
fi

curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/checkpoint" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"status":"completed","summary":{"ok":true,"store":"mysql"},"cursor":{"mock":"done"}}' >/dev/null

curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/artifacts" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"artifact_type":"mock.summary","metadata":{"ok":true,"store":"mysql"}}' >/dev/null

upload_file="${ROOT_DIR}/deploy-local/run/mysql-upload-${run_id}.txt"
mkdir -p "$(dirname "${upload_file}")"
printf 'qiyuan mysql artifact upload smoke\n' >"${upload_file}"
uploaded_artifact_json="$(curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/artifact-files" \
  -H "Authorization: Bearer ${device_token}" \
  -F 'artifact_type=mock.file' \
  -F 'metadata={"source":"mysql-smoke"}' \
  -F "file=@${upload_file};type=text/plain")"
uploaded_artifact_id="$(printf '%s' "${uploaded_artifact_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["artifact_id"])')"
downloaded_content="$(curl -fsS "${API_BASE_URL}/admin/automation/artifacts/${uploaded_artifact_id}/download")"
if [[ "${downloaded_content}" != "qiyuan mysql artifact upload smoke" ]]; then
  echo "downloaded artifact content mismatch: ${downloaded_content}" >&2
  exit 1
fi

pdf_file="${ROOT_DIR}/deploy-local/run/mysql-upload-${run_id}.pdf"
printf '%%PDF-1.4 qiyuan mysql pdf smoke\n' >"${pdf_file}"
uploaded_pdf_json="$(curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/artifact-files" \
  -H "Authorization: Bearer ${device_token}" \
  -F 'artifact_type=pdf' \
  -F 'metadata={"title":"Smoke PDF","source_url":"https://example.com/paper","pdf_url":"https://example.com/paper.pdf"}' \
  -F "file=@${pdf_file};type=application/pdf")"
uploaded_pdf_id="$(printf '%s' "${uploaded_pdf_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["artifact_id"])')"
pending_literature_count="$(curl -fsS "${API_BASE_URL}/admin/literature/results?parse_status=pending&limit=10" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["results"]))')"
if [[ "${pending_literature_count}" -lt "1" ]]; then
  echo "expected pending literature result for pdf artifact ${uploaded_pdf_id}" >&2
  exit 1
fi
parse_tasks_json="$(curl -fsS "${API_BASE_URL}/internal/literature/parse-tasks/next?limit=1" \
  -H 'X-Internal-Secret: mysql-smoke-internal-secret')"
parse_task_id="$(printf '%s' "${parse_tasks_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["tasks"][0]["result_id"])')"
curl -fsS -X POST "${API_BASE_URL}/internal/literature/results/${parse_task_id}/parse-result" \
  -H 'X-Internal-Secret: mysql-smoke-internal-secret' \
  -H 'Content-Type: application/json' \
  -d '{"status":"parsed","title":"Smoke PDF Parsed","authors":["QIYUAN"],"year":2026,"doi":"10.0000/qiyuan-smoke","venue":"Smoke Test","abstract":"parsed by smoke","extracted":{"materials":["LiFePO4"],"dft_u":[{"element":"Fe","u_ev":4.0}]}}' >/dev/null
parsed_status="$(curl -fsS "${API_BASE_URL}/admin/literature/results/${parse_task_id}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["parse_status"])')"
if [[ "${parsed_status}" != "parsed" ]]; then
  echo "expected parsed literature status, got ${parsed_status}" >&2
  exit 1
fi

manual_action_json="$(curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/manual-actions" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"action_type":"confirmation","message":"mysql smoke confirmation","payload":{"reason":"test"}}')"
manual_action_id="$(printf '%s' "${manual_action_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["manual_action_id"])')"

curl -fsS -X POST "${API_BASE_URL}/admin/automation/manual-actions/${manual_action_id}/resolve" \
  -H 'Content-Type: application/json' \
  -d '{"status":"resolved","payload":{"resolved_by":"smoke"}}' >/dev/null

curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/complete" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d "{\"job_id\":\"${job_id}\",\"status\":\"completed\",\"summary\":{\"ok\":true},\"last_cursor\":{\"mock\":\"done\"}}" >/dev/null

job_count="$(curl -fsS "${API_BASE_URL}/admin/automation/jobs?status=completed&limit=10" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["jobs"]))')"
run_count="$(curl -fsS "${API_BASE_URL}/admin/automation/runs?status=completed&limit=10" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["runs"]))')"
device_count="$(curl -fsS "${API_BASE_URL}/admin/worker/devices?status=active&limit=10" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["devices"]))')"
artifact_count="$(curl -fsS "${API_BASE_URL}/admin/automation/runs/${run_id}/artifacts" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["artifacts"]))')"
resolved_count="$(curl -fsS "${API_BASE_URL}/admin/automation/manual-actions?status=resolved&limit=10" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["manual_actions"]))')"

if [[ "${job_count}" -lt "1" || "${run_count}" -lt "1" || "${device_count}" -lt "1" || "${artifact_count}" != "3" || "${resolved_count}" -lt "1" ]]; then
  echo "unexpected persisted counts: jobs=${job_count} runs=${run_count} devices=${device_count} artifacts=${artifact_count} resolved=${resolved_count}" >&2
  exit 1
fi

curl -fsS -X POST "${API_BASE_URL}/admin/worker/devices/${device_id}/revoke" >/dev/null
revoke_status="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "${API_BASE_URL}/worker/devices/${device_id}/heartbeat" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"worker_version":"0.1.0","status":"idle","capabilities":["adapter.mock.echo"],"metrics":{}}')"
if [[ "${revoke_status}" != "401" ]]; then
  echo "expected revoked device heartbeat to be unauthorized, got ${revoke_status}" >&2
  exit 1
fi

echo "automation mysql smoke test passed"
