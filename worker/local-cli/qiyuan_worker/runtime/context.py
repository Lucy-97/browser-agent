from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from qiyuan_worker.artifacts import ArtifactCollector
from qiyuan_worker.config import WorkerConfig
from qiyuan_worker.http_client import APIClient
from qiyuan_worker.protocols import AutomationJob


@dataclass(frozen=True)
class JobContext:
    job: AutomationJob
    config: WorkerConfig
    api_client: APIClient
    artifact_collector: ArtifactCollector
    work_dir: Path
