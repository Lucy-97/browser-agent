from __future__ import annotations

import os
import platform
from dataclasses import dataclass
from pathlib import Path

from .errors import ConfigError


env_suffix = os.environ.get("QIYUAN_ENV", "default")
APP_DIR_NAME = f"QIYUAN Worker {env_suffix}"
DEFAULT_SERVER = "http://localhost:28080"
DEFAULT_POLL_INTERVAL_SECONDS = 10
DEFAULT_HEARTBEAT_INTERVAL_SECONDS = 30
DEFAULT_ENABLED_PRODUCTS = ("core", "browser_agent", "literature")


@dataclass(frozen=True)
class WorkerConfig:
    server: str
    data_dir: Path
    log_level: str
    poll_interval_seconds: int
    heartbeat_interval_seconds: int
    enabled_products: tuple[str, ...]
    llm_provider: str
    llm_model: str

    @property
    def device_file(self) -> Path:
        return self.data_dir / "device.json"

    @property
    def jobs_dir(self) -> Path:
        return self.data_dir / "jobs"

    @property
    def logs_dir(self) -> Path:
        return self.data_dir / "logs"

    @property
    def secrets_dir(self) -> Path:
        return self.data_dir / "secrets"


def default_data_dir() -> Path:
    if platform.system() == "Darwin":
        return Path.home() / "Library" / "Application Support" / APP_DIR_NAME
    if platform.system() == "Windows":
        base = os.environ.get("APPDATA")
        if base:
            return Path(base) / APP_DIR_NAME
    return Path(os.environ.get("XDG_DATA_HOME", Path.home() / ".local" / "share")) / f"qiyuan-worker-{env_suffix}"


def default_config_path() -> Path:
    return default_data_dir() / "config.yaml"


def ensure_config_dirs(config: WorkerConfig) -> None:
    config.data_dir.mkdir(parents=True, exist_ok=True)
    config.jobs_dir.mkdir(parents=True, exist_ok=True)
    config.logs_dir.mkdir(parents=True, exist_ok=True)
    config.secrets_dir.mkdir(parents=True, exist_ok=True)


def write_default_config(
    server: str = DEFAULT_SERVER,
    config_path: Path | None = None,
    data_dir: Path | None = None,
) -> WorkerConfig:
    resolved_data_dir = data_dir or default_data_dir()
    config = WorkerConfig(
        server=server.rstrip("/"),
        data_dir=resolved_data_dir,
        log_level="info",
        poll_interval_seconds=DEFAULT_POLL_INTERVAL_SECONDS,
        heartbeat_interval_seconds=DEFAULT_HEARTBEAT_INTERVAL_SECONDS,
        enabled_products=DEFAULT_ENABLED_PRODUCTS,
        llm_provider="disabled",
        llm_model="",
    )
    ensure_config_dirs(config)
    path = config_path or default_config_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        "\n".join(
            [
                f"server: {config.server}",
                f"data_dir: {config.data_dir}",
                f"log_level: {config.log_level}",
                f"poll_interval_seconds: {config.poll_interval_seconds}",
                f"heartbeat_interval_seconds: {config.heartbeat_interval_seconds}",
                f"enabled_products: {','.join(config.enabled_products)}",
                f"llm_provider: {config.llm_provider}",
                f"llm_model: {config.llm_model}",
                "",
            ]
        ),
        encoding="utf-8",
    )
    return config


def load_config(config_path: Path | None = None) -> WorkerConfig:
    path = config_path or default_config_path()
    if not path.exists():
        raise ConfigError(f"config not found: {path}. Run `qiyuan-worker init` first.")

    values: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if ":" not in stripped:
            raise ConfigError(f"invalid config line: {line}")
        key, value = stripped.split(":", 1)
        values[key.strip()] = value.strip()

    try:
        config = WorkerConfig(
            server=values.get("server", DEFAULT_SERVER).rstrip("/"),
            data_dir=Path(values.get("data_dir") or default_data_dir()).expanduser(),
            log_level=values.get("log_level", "info"),
            poll_interval_seconds=int(values.get("poll_interval_seconds", DEFAULT_POLL_INTERVAL_SECONDS)),
            heartbeat_interval_seconds=int(
                values.get("heartbeat_interval_seconds", DEFAULT_HEARTBEAT_INTERVAL_SECONDS)
            ),
            enabled_products=_parse_csv(values.get("enabled_products"), DEFAULT_ENABLED_PRODUCTS),
            llm_provider=os.environ.get("LLM_PROVIDER") or values.get("llm_provider") or "disabled",
            llm_model=os.environ.get("LLM_MODEL") or values.get("llm_model") or "",
        )
    except ValueError as exc:
        raise ConfigError(f"invalid integer in config: {exc}") from exc

    ensure_config_dirs(config)
    return config


def _parse_csv(value: str | None, default: tuple[str, ...]) -> tuple[str, ...]:
    if value is None or not value.strip():
        return default
    parsed = tuple(item.strip() for item in value.split(",") if item.strip())
    return parsed or default
