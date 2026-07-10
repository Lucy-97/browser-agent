from __future__ import annotations

import asyncio
import time
from collections.abc import Callable
from datetime import datetime, timezone

from . import __version__
from .adapters import build_default_registry, load_worker_extensions
from .browser import BrowserRuntime, BrowserRuntimeConfig
from .config import WorkerConfig
from .errors import APIError
from .http_client import APIClient
from .manifest import write_job
from .models import DeviceInfo
from .protocols import AutomationJob
from .runtime import JobRunner
from .sdk import WorkerExtension, detect_extension_capabilities


DEFAULT_CAPABILITIES = {"automation.runtime"}


def detect_runtime_capabilities(config: WorkerConfig) -> set[str]:
    capabilities = {"automation.runtime"}
    browser_doctor = BrowserRuntime(
        BrowserRuntimeConfig(
            profile_dir=config.secrets_dir / "browser-profiles" / "default",
            downloads_dir=config.data_dir / "downloads",
        )
    ).doctor()
    if browser_doctor.playwright_installed:
        capabilities.update(
            {
                "browser.playwright.chromium",
                "browser.profile.persistent",
            }
        )
    return capabilities


def detect_capabilities(
    config: WorkerConfig,
    extensions: tuple[WorkerExtension, ...] | None = None,
) -> set[str]:
    resolved_extensions = extensions or load_worker_extensions(enabled_products=config.enabled_products)
    return detect_extension_capabilities(resolved_extensions, detect_runtime_capabilities(config))


def log_event(message: str, **fields: object) -> None:
    timestamp = datetime.now(timezone.utc).isoformat(timespec="seconds")
    suffix = " ".join(f"{key}={value}" for key, value in fields.items() if value is not None)
    print(f"{timestamp} {message}{(' ' + suffix) if suffix else ''}", flush=True)


def send_device_heartbeat(
    client: APIClient,
    device: DeviceInfo,
    status: str,
    current_job: dict | AutomationJob | None = None,
    capabilities: set[str] | None = None,
) -> None:
    job_id = None
    run_id = None
    if isinstance(current_job, AutomationJob):
        job_id = current_job.job_id
        run_id = current_job.run_id
    elif current_job:
        job_id = current_job["job_id"]
        run_id = current_job["run_id"]

    client.device_heartbeat(
        device.device_id,
        {
            "worker_version": __version__,
            "status": status,
            "current_job_id": job_id,
            "current_run_id": run_id,
            "capabilities": sorted(capabilities or DEFAULT_CAPABILITIES),
            "metrics": {"pending_upload_count": 0},
        },
    )


def run_once(client: APIClient, config: WorkerConfig, device: DeviceInfo, source: str | None = None) -> bool:
    extensions = load_worker_extensions(enabled_products=config.enabled_products)
    capabilities = detect_capabilities(config, extensions)
    log_event("worker.poll", device_id=device.device_id, capabilities=",".join(sorted(capabilities)))
    send_device_heartbeat(client, device, "idle", capabilities=capabilities)
    job_payload = client.next_automation_job(source=source)
    if not job_payload:
        log_event("worker.no_job", device_id=device.device_id)
        return False

    job = AutomationJob.from_payload(job_payload)
    log_event("worker.job_claimed", job_id=job.job_id, run_id=job.run_id, adapter=job.adapter, job_type=job.job_type)
    write_job(config, job.to_dict())
    send_device_heartbeat(client, device, "running", current_job=job, capabilities=capabilities)
    runner = JobRunner(build_default_registry(extensions=extensions), capabilities)
    result = asyncio.run(runner.run(client, config, job))
    log_event(
        "worker.job_finished",
        job_id=job.job_id,
        run_id=job.run_id,
        status=result.status,
        error_code=result.error_code,
    )
    send_device_heartbeat(client, device, "idle", capabilities=capabilities)
    return True


def run_forever(
    client: APIClient,
    config: WorkerConfig,
    device: DeviceInfo,
    source: str | None = None,
    once: bool = False,
    sleep: Callable[[float], None] = time.sleep,
) -> None:
    while True:
        try:
            had_job = run_once(client, config, device, source=source)
        except APIError as exc:
            if once or not exc.retryable:
                raise
            log_event(
                "worker.poll_retryable_error",
                device_id=device.device_id,
                error_code=exc.code,
                error_message=exc.message,
            )
            sleep(config.poll_interval_seconds)
            continue
        if once:
            return
        if not had_job:
            sleep(config.poll_interval_seconds)
