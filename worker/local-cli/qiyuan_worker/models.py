from __future__ import annotations

import hashlib
import json
import platform
import socket
from dataclasses import asdict, dataclass
from pathlib import Path


@dataclass(frozen=True)
class DeviceInfo:
    device_id: str
    name: str
    platform: str
    worker_version: str


@dataclass(frozen=True)
class PairingInfo:
    pairing_id: str
    pairing_code: str
    verification_uri: str
    expires_at: str
    poll_interval_seconds: int


@dataclass(frozen=True)
class Job:
    job_id: str
    run_id: str
    source: str
    query: str
    max_results: int
    cursor: dict | None
    crawler_config: dict
    upload_policy: dict


def current_platform() -> str:
    system = platform.system()
    machine = platform.machine().lower()
    arch = "arm64" if machine in {"arm64", "aarch64"} else "amd64"
    if system == "Darwin":
        return f"darwin-{arch}"
    if system == "Windows":
        return f"windows-{arch}"
    return f"linux-{arch}"


def hostname_hash() -> str:
    digest = hashlib.sha256(socket.gethostname().encode("utf-8")).hexdigest()
    return f"sha256:{digest}"


def save_device(path: Path, device: DeviceInfo) -> None:
    path.write_text(json.dumps(asdict(device), ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    path.chmod(0o600)


def load_device(path: Path) -> DeviceInfo | None:
    if not path.exists():
        return None
    data = json.loads(path.read_text(encoding="utf-8"))
    return DeviceInfo(
        device_id=data["device_id"],
        name=data["name"],
        platform=data["platform"],
        worker_version=data["worker_version"],
    )
