from __future__ import annotations

import argparse
import sys

from . import __version__
from .browser import BrowserRuntime, BrowserRuntimeConfig
from .mcp import run_mcp_server
from .config import default_config_path, load_config, write_default_config
from .crypto import build_secret_store
from .device import create_pairing, poll_pairing_until_approved, require_device, require_token
from .errors import APIError, ConfigError
from .http_client import APIClient
from .job_loop import run_forever
from .models import current_platform, load_device


def main(argv: list[str] | None = None) -> None:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        args.func(args)
    except (APIError, ConfigError, RuntimeError, TimeoutError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="qiyuan-worker")
    parser.add_argument("--version", action="version", version=f"qiyuan-worker {__version__}")
    sub = parser.add_subparsers(dest="command", required=True)

    init_parser = sub.add_parser("init", help="initialize local worker config")
    init_parser.add_argument("--server", default="http://localhost:29001")
    init_parser.set_defaults(func=cmd_init)

    pair_parser = sub.add_parser("pair", help="pair this machine with the platform")
    pair_parser.add_argument("--display-name", default=None)
    pair_parser.add_argument("--timeout-seconds", type=int, default=600)
    pair_parser.set_defaults(func=cmd_pair)

    run_parser = sub.add_parser("run", help="run worker job loop")
    run_parser.add_argument("--source", default=None)
    run_parser.add_argument("--job", default=None, help="run a specific job id when direct-claim API is available")
    run_parser.add_argument("--once", action="store_true", help="run one poll cycle and exit")
    run_parser.set_defaults(func=cmd_run)

    status_parser = sub.add_parser("status", help="show local worker status")
    status_parser.set_defaults(func=cmd_status)

    doctor_parser = sub.add_parser("doctor", help="check local worker prerequisites")
    doctor_parser.set_defaults(func=cmd_doctor)

    logs_parser = sub.add_parser("logs", help="show local worker logs")
    logs_parser.add_argument("--tail", type=int, default=200)
    logs_parser.set_defaults(func=cmd_logs)

    clear_parser = sub.add_parser("clear-session", help="clear local source session placeholder")
    clear_parser.add_argument("--source", required=True)
    clear_parser.set_defaults(func=cmd_clear_session)

    mcp_parser = sub.add_parser("mcp", help="start MCP server for browser automation tools")
    mcp_parser.add_argument("--profile", default="mcp", help="browser profile name (isolated from run)")
    mcp_parser.add_argument("--headed", action="store_true", default=True, help="show browser window")
    mcp_parser.add_argument("--headless", action="store_true", help="run browser headlessly")
    mcp_parser.set_defaults(func=cmd_mcp)

    return parser


def cmd_init(args: argparse.Namespace) -> None:
    config = write_default_config(server=args.server)
    print(f"initialized config: {default_config_path()}")
    print(f"data dir: {config.data_dir}")


def cmd_pair(args: argparse.Namespace) -> None:
    config = load_config()
    secrets = build_secret_store(config.secrets_dir)
    client = APIClient(config.server)
    pairing = create_pairing(client, display_name=args.display_name)
    print("Pair this device in the Browser Agent platform:")
    print(f"  code: {pairing.pairing_code}")
    print(f"  url:  {pairing.verification_uri}")
    print(f"  expires_at: {pairing.expires_at}")
    device = poll_pairing_until_approved(
        client,
        pairing,
        config,
        secrets,
        timeout_seconds=args.timeout_seconds,
    )
    print(f"paired device: {device.device_id} ({device.name})")


def cmd_run(args: argparse.Namespace) -> None:
    if args.job:
        raise RuntimeError("run --job is reserved for the direct job claim API; use `run --once` for M2 mock flow.")
    config = load_config()
    secrets = build_secret_store(config.secrets_dir)
    device = require_device(config)
    token = require_token(secrets)
    client = APIClient(config.server, token=token)
    run_forever(client, config, device, source=args.source, once=args.once)


def cmd_status(args: argparse.Namespace) -> None:
    config = load_config()
    device = load_device(config.device_file)
    print(f"server: {config.server}")
    print(f"data_dir: {config.data_dir}")
    print(f"enabled_products: {','.join(config.enabled_products)}")
    print(f"llm_provider: {config.llm_provider}")
    if config.llm_model:
        print(f"llm_model: {config.llm_model}")
    print(f"platform: {current_platform()}")
    if device:
        print(f"device_id: {device.device_id}")
        print(f"device_name: {device.name}")
        print(f"worker_version: {device.worker_version}")
    else:
        print("device: not paired")


def cmd_doctor(args: argparse.Namespace) -> None:
    print(f"python: {sys.version.split()[0]}")
    print(f"worker: {__version__}")
    print(f"platform: {current_platform()}")
    try:
        config = load_config()
        print(f"config: ok ({default_config_path()})")
        print(f"data_dir: ok ({config.data_dir})")
        print(f"enabled_products: {','.join(config.enabled_products)}")
        print(f"llm_provider: {config.llm_provider}")
        browser_runtime = BrowserRuntime(
            BrowserRuntimeConfig(
                profile_dir=config.secrets_dir / "browser-profiles" / "default",
                downloads_dir=config.data_dir / "downloads",
            )
        )
        browser_doctor = browser_runtime.doctor()
        print(f"browser_profile: {'ok' if browser_doctor.profile_dir_ready else 'missing'}")
        print(f"browser_downloads: {'ok' if browser_doctor.downloads_dir_ready else 'missing'}")
        print(f"playwright: {'ok' if browser_doctor.playwright_installed else 'missing'}")
        print(f"chromium_cli: {'available' if browser_doctor.chromium_cli_available else 'not-found'}")
        print(f"browser_runtime: {browser_doctor.message}")
        try:
            build_secret_store(config.secrets_dir)
            print("secret_store: ok")
        except ConfigError as exc:
            print(f"secret_store: unavailable ({exc})")
    except ConfigError as exc:
        print(f"config: missing ({exc})")


def cmd_clear_session(args: argparse.Namespace) -> None:
    config = load_config()
    session_file = config.secrets_dir / "storage-state" / f"{args.source}.enc"
    if session_file.exists():
        session_file.unlink()
        print(f"cleared session: {args.source}")
    else:
        print(f"session not found: {args.source}")


def cmd_logs(args: argparse.Namespace) -> None:
    config = load_config()
    log_file = config.logs_dir / "worker.log"
    if not log_file.exists():
        print("log file not found")
        return
    lines = log_file.read_text(encoding="utf-8").splitlines()
    for line in lines[-args.tail :]:
        print(line)


def cmd_mcp(args: argparse.Namespace) -> None:
    headed = not args.headless
    run_mcp_server(profile_name=args.profile, headed=headed)
