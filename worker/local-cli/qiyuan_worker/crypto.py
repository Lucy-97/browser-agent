from __future__ import annotations

import os
import platform
import subprocess
from pathlib import Path

from .errors import ConfigError


env_suffix = os.environ.get("BROWSER_AGENT_ENV") or os.environ.get("QIYUAN_ENV", "default")
SERVICE_NAME = f"qiyuan-worker-{env_suffix}"


class SecretStore:
    def set_secret(self, key: str, value: str) -> None:
        raise NotImplementedError

    def get_secret(self, key: str) -> str | None:
        raise NotImplementedError

    def delete_secret(self, key: str) -> None:
        raise NotImplementedError


class MacOSKeychainSecretStore(SecretStore):
    def set_secret(self, key: str, value: str) -> None:
        self.delete_secret(key)
        subprocess.run(
            ["security", "add-generic-password", "-a", key, "-s", SERVICE_NAME, "-w", value],
            check=True,
            capture_output=True,
            text=True,
        )

    def get_secret(self, key: str) -> str | None:
        result = subprocess.run(
            ["security", "find-generic-password", "-a", key, "-s", SERVICE_NAME, "-w"],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            return None
        return result.stdout.rstrip("\n")

    def delete_secret(self, key: str) -> None:
        subprocess.run(
            ["security", "delete-generic-password", "-a", key, "-s", SERVICE_NAME],
            capture_output=True,
            text=True,
        )


class DevFileSecretStore(SecretStore):
    """Development-only secret store enabled explicitly by env var."""

    def __init__(self, secrets_dir: Path):
        self.secrets_dir = secrets_dir
        self.secrets_dir.mkdir(parents=True, exist_ok=True)

    def _path(self, key: str) -> Path:
        safe = key.replace("/", "_").replace(":", "_")
        return self.secrets_dir / f"{safe}.secret"

    def set_secret(self, key: str, value: str) -> None:
        path = self._path(key)
        path.write_text(value, encoding="utf-8")
        path.chmod(0o600)

    def get_secret(self, key: str) -> str | None:
        path = self._path(key)
        if not path.exists():
            return None
        return path.read_text(encoding="utf-8")

    def delete_secret(self, key: str) -> None:
        path = self._path(key)
        if path.exists():
            path.unlink()


def build_secret_store(secrets_dir: Path) -> SecretStore:
    allow_insecure = os.environ.get("BROWSER_AGENT_WORKER_ALLOW_INSECURE_FILE_SECRETS")
    if allow_insecure is None:
        allow_insecure = os.environ.get("QIYUAN_WORKER_ALLOW_INSECURE_FILE_SECRETS")
    if allow_insecure == "1":
        return DevFileSecretStore(secrets_dir)
    if platform.system() == "Darwin":
        return MacOSKeychainSecretStore()
    raise ConfigError(
        "no secure secret store available. On non-macOS dev machines, set "
        "BROWSER_AGENT_WORKER_ALLOW_INSECURE_FILE_SECRETS=1 only for local testing."
    )
