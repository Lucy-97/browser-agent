from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from .config import WorkerConfig


def job_dir(config: WorkerConfig, job_id: str) -> Path:
    path = config.jobs_dir / job_id
    path.mkdir(parents=True, exist_ok=True)
    return path


def write_job(config: WorkerConfig, job: dict[str, Any]) -> None:
    path = job_dir(config, job["job_id"]) / "job.json"
    path.write_text(json.dumps(job, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def append_checkpoint(config: WorkerConfig, job_id: str, checkpoint: dict[str, Any]) -> None:
    path = job_dir(config, job_id) / "checkpoints.ndjson"
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(checkpoint, ensure_ascii=False) + "\n")


def write_upload_manifest(config: WorkerConfig, job_id: str, manifest: dict[str, Any]) -> None:
    path = job_dir(config, job_id) / "upload-manifest.json"
    path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
