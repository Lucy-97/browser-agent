from __future__ import annotations

import json
import mimetypes
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

from .errors import APIError


@dataclass
class APIResponse:
    status: int
    data: Any | None


class APIClient:
    def __init__(self, base_url: str, token: str | None = None, timeout: float = 20.0):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout

    def create_pairing(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json("POST", "/worker/devices/pairing", payload=payload, auth=False).data

    def get_pairing(self, pairing_id: str) -> dict[str, Any]:
        return self._json("GET", f"/worker/devices/pairing/{pairing_id}", auth=False).data

    def device_heartbeat(self, device_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json("POST", f"/worker/devices/{device_id}/heartbeat", payload=payload).data

    def next_job(self, source: str | None = None) -> dict[str, Any] | None:
        path = "/worker/jobs/next"
        if source:
            path += "?" + urlencode({"source": source})
        response = self._json("GET", path)
        return response.data

    def next_automation_job(self, source: str | None = None) -> dict[str, Any] | None:
        path = "/worker/automation/jobs/next"
        if source:
            path += "?" + urlencode({"source": source})
        response = self._json("GET", path)
        return response.data

    def job_heartbeat(self, job_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json("POST", f"/worker/jobs/{job_id}/heartbeat", payload=payload).data

    def checkpoint(self, job_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json(
            "POST",
            f"/worker/jobs/{job_id}/checkpoint",
            payload=payload,
            idempotency_key=str(uuid.uuid4()),
        ).data

    def complete_job(self, job_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json(
            "POST",
            f"/worker/jobs/{job_id}/complete",
            payload=payload,
            idempotency_key=str(uuid.uuid4()),
        ).data

    def run_heartbeat(self, run_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json("POST", f"/worker/automation/runs/{run_id}/heartbeat", payload=payload).data

    def run_checkpoint(self, run_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json(
            "POST",
            f"/worker/automation/runs/{run_id}/checkpoint",
            payload=payload,
            idempotency_key=str(uuid.uuid4()),
        ).data

    def run_status(self, run_id: str) -> dict[str, Any]:
        return self._json("GET", f"/worker/automation/runs/{run_id}").data

    def create_manual_action(self, run_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json(
            "POST",
            f"/worker/automation/runs/{run_id}/manual-actions",
            payload=payload,
            idempotency_key=str(uuid.uuid4()),
        ).data

    def get_manual_action(self, manual_action_id: str) -> dict[str, Any]:
        return self._json("GET", f"/worker/automation/manual-actions/{manual_action_id}").data

    def create_run_artifact(self, run_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json(
            "POST",
            f"/worker/automation/runs/{run_id}/artifacts",
            payload=payload,
            idempotency_key=str(uuid.uuid4()),
        ).data

    def upload_run_artifact_file(
        self,
        run_id: str,
        artifact_type: str,
        path: Path,
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        boundary = "----qiyuan-worker-" + uuid.uuid4().hex
        content_type = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
        fields = {
            "artifact_type": artifact_type,
            "metadata": json.dumps(metadata or {}),
        }
        body_parts: list[bytes] = []
        for name, value in fields.items():
            body_parts.extend(
                [
                    f"--{boundary}\r\n".encode("utf-8"),
                    f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode("utf-8"),
                    str(value).encode("utf-8"),
                    b"\r\n",
                ]
            )
        body_parts.extend(
            [
                f"--{boundary}\r\n".encode("utf-8"),
                (
                    f'Content-Disposition: form-data; name="file"; filename="{path.name}"\r\n'
                    f"Content-Type: {content_type}\r\n\r\n"
                ).encode("utf-8"),
                path.read_bytes(),
                b"\r\n",
                f"--{boundary}--\r\n".encode("utf-8"),
            ]
        )
        return self._json(
            "POST",
            f"/worker/automation/runs/{run_id}/artifact-files",
            raw_body=b"".join(body_parts),
            content_type=f"multipart/form-data; boundary={boundary}",
            idempotency_key=str(uuid.uuid4()),
        ).data

    def complete_run(self, run_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._json(
            "POST",
            f"/worker/automation/runs/{run_id}/complete",
            payload=payload,
            idempotency_key=str(uuid.uuid4()),
        ).data

    def _json(
        self,
        method: str,
        path: str,
        payload: dict[str, Any] | None = None,
        raw_body: bytes | None = None,
        content_type: str | None = None,
        auth: bool = True,
        idempotency_key: str | None = None,
    ) -> APIResponse:
        body = None
        headers = {"Accept": "application/json"}
        if raw_body is not None:
            body = raw_body
            headers["Content-Type"] = content_type or "application/octet-stream"
        elif payload is not None:
            body = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if auth and self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        if idempotency_key:
            headers["Idempotency-Key"] = idempotency_key

        request = Request(f"{self.base_url}{path}", data=body, headers=headers, method=method)
        last_error: Exception | None = None
        for attempt in range(3):
            try:
                with urlopen(request, timeout=self.timeout) as response:
                    if response.status == 204:
                        return APIResponse(status=204, data=None)
                    raw = response.read()
                    data = json.loads(raw.decode("utf-8")) if raw else None
                    return APIResponse(status=response.status, data=data)
            except HTTPError as exc:
                raise self._api_error(exc) from exc
            except URLError as exc:
                last_error = exc
                if attempt < 2:
                    time.sleep(0.5 * (2**attempt))
                    continue
                raise APIError("NETWORK_ERROR", str(exc), retryable=True) from exc
        raise APIError("NETWORK_ERROR", str(last_error), retryable=True)

    @staticmethod
    def _api_error(exc: HTTPError) -> APIError:
        try:
            raw = exc.read()
            data = json.loads(raw.decode("utf-8")) if raw else {}
            error = data.get("error") or {}
            return APIError(
                code=error.get("code") or f"HTTP_{exc.code}",
                message=error.get("message") or exc.reason,
                retryable=bool(error.get("retryable")),
                status=exc.code,
            )
        except Exception:
            return APIError(code=f"HTTP_{exc.code}", message=exc.reason, status=exc.code)
