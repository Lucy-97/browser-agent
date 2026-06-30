from __future__ import annotations

from pathlib import Path

from qiyuan_worker.protocols import Artifact


class ArtifactCollector:
    def __init__(self, work_dir: Path):
        self.work_dir = work_dir
        self.work_dir.mkdir(parents=True, exist_ok=True)
        self._artifacts: list[Artifact] = []

    def add_metadata(self, artifact_type: str, metadata: dict) -> Artifact:
        artifact = Artifact(artifact_type=artifact_type, metadata=metadata)
        self._artifacts.append(artifact)
        return artifact

    def add_file(self, artifact_type: str, path: Path, metadata: dict | None = None) -> Artifact:
        artifact = Artifact(
            artifact_type=artifact_type,
            local_path=path,
            metadata=metadata or {},
            size_bytes=path.stat().st_size if path.exists() else None,
        )
        self._artifacts.append(artifact)
        return artifact

    def collected(self) -> tuple[Artifact, ...]:
        return tuple(self._artifacts)
