from __future__ import annotations

import time

from . import __version__
from .config import WorkerConfig
from .crypto import SecretStore
from .http_client import APIClient
from .models import DeviceInfo, PairingInfo, current_platform, hostname_hash, load_device, save_device


DEVICE_TOKEN_KEY = "device-token"
REFRESH_TOKEN_KEY = "refresh-token"


def create_pairing(client: APIClient, display_name: str | None = None) -> PairingInfo:
    payload = {
        "worker_version": __version__,
        "platform": current_platform(),
        "hostname_hash": hostname_hash(),
        "display_name": display_name,
    }
    data = client.create_pairing(payload)
    return PairingInfo(
        pairing_id=data["pairing_id"],
        pairing_code=data["pairing_code"],
        verification_uri=data["verification_uri"],
        expires_at=data["expires_at"],
        poll_interval_seconds=int(data.get("poll_interval_seconds") or 3),
    )


def poll_pairing_until_approved(
    client: APIClient,
    pairing: PairingInfo,
    config: WorkerConfig,
    secrets: SecretStore,
    timeout_seconds: int = 600,
) -> DeviceInfo:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        data = client.get_pairing(pairing.pairing_id)
        status = data["status"]
        if status == "approved":
            device = data["device"]
            info = DeviceInfo(
                device_id=device["id"],
                name=device["name"],
                platform=current_platform(),
                worker_version=__version__,
            )
            save_device(config.device_file, info)
            secrets.set_secret(DEVICE_TOKEN_KEY, data["device_token"])
            if data.get("refresh_token"):
                secrets.set_secret(REFRESH_TOKEN_KEY, data["refresh_token"])
            return info
        if status in {"expired", "rejected"}:
            raise RuntimeError(f"pairing {status}")
        time.sleep(pairing.poll_interval_seconds)
    raise TimeoutError("pairing approval timed out")


def require_device(config: WorkerConfig) -> DeviceInfo:
    device = load_device(config.device_file)
    if not device:
        raise RuntimeError("device not paired. Run `qiyuan-worker pair` first.")
    return device


def require_token(secrets: SecretStore) -> str:
    token = secrets.get_secret(DEVICE_TOKEN_KEY)
    if not token:
        raise RuntimeError("device token not found. Run `qiyuan-worker pair` first.")
    return token
