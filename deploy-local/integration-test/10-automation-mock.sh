#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:28001}"
RUN_SCRIPT="${ROOT_DIR}/deploy-local/tools/run-api-host-local.sh"
STARTED_BY_TEST=0

cleanup() {
  if [[ "${STARTED_BY_TEST}" == "1" ]]; then
    bash "${RUN_SCRIPT}" stop >/dev/null || true
  fi
}
trap cleanup EXIT

if ! curl -fsS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
  bash "${RUN_SCRIPT}" start >/dev/null
  STARTED_BY_TEST=1
  for _ in $(seq 1 30); do
    if curl -fsS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

pairing_json="$(curl -fsS -X POST "${API_BASE_URL}/worker/devices/pairing" \
  -H 'Content-Type: application/json' \
  -d '{"worker_version":"0.1.0","platform":"darwin-arm64","hostname_hash":"sha256:test","display_name":"smoke-worker"}')"

pairing_id="$(printf '%s' "${pairing_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["pairing_id"])')"
approved_json="$(curl -fsS "${API_BASE_URL}/worker/devices/pairing/${pairing_id}")"
device_id="$(printf '%s' "${approved_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["device"]["id"])')"
device_token="$(printf '%s' "${approved_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["device_token"])')"

curl -fsS -X POST "${API_BASE_URL}/worker/devices/${device_id}/heartbeat" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"worker_version":"0.1.0","status":"idle","capabilities":["adapter.mock.echo"],"metrics":{"pending_upload_count":0}}' >/dev/null

curl -fsS -X POST "${API_BASE_URL}/admin/automation/jobs" \
  -H 'Content-Type: application/json' \
  -d '{"job_type":"generic.browser.script","adapter":"mock.echo","target":{"allowed_domains":["example.com"]},"input":{"message":"hello"},"policy":{},"priority":1}' >/dev/null

job_json="$(curl -fsS "${API_BASE_URL}/worker/automation/jobs/next" -H "Authorization: Bearer ${device_token}")"
run_id="$(printf '%s' "${job_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["run_id"])')"
job_id="$(printf '%s' "${job_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])')"

curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/checkpoint" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"status":"completed","summary":{"ok":true},"cursor":{"mock":"done"}}' >/dev/null

curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/artifacts" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d '{"artifact_type":"mock.summary","metadata":{"ok":true}}' >/dev/null

curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/complete" \
  -H "Authorization: Bearer ${device_token}" \
  -H 'Content-Type: application/json' \
  -d "{\"job_id\":\"${job_id}\",\"status\":\"completed\",\"summary\":{\"ok\":true},\"last_cursor\":{\"mock\":\"done\"}}" >/dev/null

curl -fsS "${API_BASE_URL}/admin/automation/jobs/${job_id}" >/dev/null
curl -fsS "${API_BASE_URL}/admin/automation/runs/${run_id}" >/dev/null
artifact_count="$(curl -fsS "${API_BASE_URL}/admin/automation/runs/${run_id}/artifacts" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["artifacts"]))')"
if [[ "${artifact_count}" != "1" ]]; then
  echo "expected 1 artifact, got ${artifact_count}" >&2
  exit 1
fi

upload_file="${ROOT_DIR}/deploy-local/run/mock-upload-${run_id}.txt"
mkdir -p "$(dirname "${upload_file}")"
printf 'qiyuan artifact upload smoke\n' >"${upload_file}"
uploaded_artifact_json="$(curl -fsS -X POST "${API_BASE_URL}/worker/automation/runs/${run_id}/artifact-files" \
  -H "Authorization: Bearer ${device_token}" \
  -F 'artifact_type=mock.file' \
  -F 'metadata={"source":"smoke"}' \
  -F "file=@${upload_file};type=text/plain")"
uploaded_artifact_id="$(printf '%s' "${uploaded_artifact_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["artifact_id"])')"
downloaded_content="$(curl -fsS "${API_BASE_URL}/admin/automation/artifacts/${uploaded_artifact_id}/download")"
if [[ "${downloaded_content}" != "qiyuan artifact upload smoke" ]]; then
  echo "downloaded artifact content mismatch: ${downloaded_content}" >&2
  exit 1
fi

echo "automation mock smoke test passed"
