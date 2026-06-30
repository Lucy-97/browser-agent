from __future__ import annotations

from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Literal


RunStatus = Literal["completed", "failed", "needs_manual_action"]


@dataclass(frozen=True)
class AutomationJob:
    job_id: str
    run_id: str
    job_type: str
    adapter: str
    target: dict[str, Any] = field(default_factory=dict)
    input: dict[str, Any] = field(default_factory=dict)
    policy: dict[str, Any] = field(default_factory=dict)
    cursor: dict[str, Any] | None = None

    @classmethod
    def from_payload(cls, payload: dict[str, Any]) -> "AutomationJob":
        if "job_type" in payload:
            return cls(
                job_id=payload["job_id"],
                run_id=payload["run_id"],
                job_type=payload["job_type"],
                adapter=payload["adapter"],
                target=dict(payload.get("target") or {}),
                input=dict(payload.get("input") or {}),
                policy=dict(payload.get("policy") or {}),
                cursor=payload.get("cursor"),
            )
        raise ValueError("job_type is missing from payload")

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


@dataclass(frozen=True)
class Artifact:
    artifact_type: str
    local_path: Path | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
    sha256: str | None = None
    size_bytes: int | None = None

    def to_payload(self) -> dict[str, Any]:
        return {
            "artifact_type": self.artifact_type,
            "local_path": str(self.local_path) if self.local_path else None,
            "metadata": self.metadata,
            "sha256": self.sha256,
            "size_bytes": self.size_bytes,
        }


@dataclass(frozen=True)
class ManualAction:
    action_type: str
    message: str
    payload: dict[str, Any] = field(default_factory=dict)

    def to_payload(self) -> dict[str, Any]:
        return {
            "action_type": self.action_type,
            "message": self.message,
            "payload": self.payload,
        }


@dataclass(frozen=True)
class AdapterResult:
    status: RunStatus
    summary: dict[str, Any] = field(default_factory=dict)
    cursor: dict[str, Any] | None = None
    artifacts: tuple[Artifact, ...] = ()
    manual_action: ManualAction | None = None
    error_code: str | None = None
    error_message: str | None = None
    retryable: bool = False

    @classmethod
    def completed(
        cls,
        summary: dict[str, Any] | None = None,
        cursor: dict[str, Any] | None = None,
        artifacts: tuple[Artifact, ...] = (),
    ) -> "AdapterResult":
        return cls(status="completed", summary=summary or {}, cursor=cursor, artifacts=artifacts)

    @classmethod
    def failed(cls, code: str, message: str, retryable: bool = False) -> "AdapterResult":
        return cls(
            status="failed",
            error_code=code,
            error_message=message,
            retryable=retryable,
        )
