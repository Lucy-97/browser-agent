from __future__ import annotations

import importlib.util
import shutil
from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class BrowserRuntimeConfig:
    profile_dir: Path
    downloads_dir: Path
    headed: bool = True
    browser_name: str = "chromium"
    clear_profile: bool = False
    channel: str = "chrome"


@dataclass(frozen=True)
class BrowserDoctorResult:
    playwright_installed: bool
    chromium_cli_available: bool
    profile_dir_ready: bool
    downloads_dir_ready: bool
    message: str

    @property
    def ok(self) -> bool:
        return self.playwright_installed and self.profile_dir_ready and self.downloads_dir_ready


class BrowserRuntime:
    def __init__(self, config: BrowserRuntimeConfig):
        self.config = config

    def doctor(self) -> BrowserDoctorResult:
        self.config.profile_dir.mkdir(parents=True, exist_ok=True)
        self.config.downloads_dir.mkdir(parents=True, exist_ok=True)

        playwright_installed = importlib.util.find_spec("playwright") is not None
        chromium_cli_available = any(
            shutil.which(name)
            for name in (
                "chromium",
                "chromium-browser",
                "google-chrome",
                "Google Chrome",
            )
        )
        profile_dir_ready = self.config.profile_dir.exists() and self.config.profile_dir.is_dir()
        downloads_dir_ready = self.config.downloads_dir.exists() and self.config.downloads_dir.is_dir()

        if playwright_installed:
            message = "playwright package available"
        else:
            message = "playwright package missing; install browser extras before running real adapters"

        return BrowserDoctorResult(
            playwright_installed=playwright_installed,
            chromium_cli_available=bool(chromium_cli_available),
            profile_dir_ready=profile_dir_ready,
            downloads_dir_ready=downloads_dir_ready,
            message=message,
        )

    async def open_page(self) -> "BrowserPageSession":
        try:
            from playwright.async_api import async_playwright
        except ImportError as exc:
            raise BrowserRuntimeError("PLAYWRIGHT_NOT_INSTALLED", "playwright package is not installed") from exc

        # Optionally clear the persistent profile to start with a clean session.
        # This is useful when a previous session was flagged by anti-bot systems
        # (e.g. Google Scholar CAPTCHA), so the accumulated cookies/fingerprint are
        # discarded and a fresh session is established.
        if self.config.clear_profile and self.config.profile_dir.exists():
            shutil.rmtree(self.config.profile_dir, ignore_errors=True)

        self.config.profile_dir.mkdir(parents=True, exist_ok=True)
        self.config.downloads_dir.mkdir(parents=True, exist_ok=True)

        playwright = await async_playwright().start()
        try:
            browser_type = getattr(playwright, self.config.browser_name)
            # Use system-installed Chrome (channel="chrome") instead of Playwright's
            # bundled "Google Chrome for Testing" to reduce bot-detection signals.
            # Falls back to bundled Chromium if system Chrome is not available.
            launch_kwargs: dict[str, Any] = dict(
                user_data_dir=str(self.config.profile_dir),
                headless=not self.config.headed,
                accept_downloads=True,
                downloads_path=str(self.config.downloads_dir),
                # Strip --no-sandbox from Playwright defaults: it is unnecessary
                # on macOS (headed mode), triggers Chrome's "unsupported flag"
                # banner, and is a strong automation-detection signal.
                ignore_default_args=["--no-sandbox"],
            )
            if self.config.channel:
                launch_kwargs["channel"] = self.config.channel
            try:
                context = await browser_type.launch_persistent_context(**launch_kwargs)
            except Exception as channel_exc:
                if self.config.channel:
                    # System channel browser not available; retry without channel.
                    launch_kwargs.pop("channel", None)
                    context = await browser_type.launch_persistent_context(**launch_kwargs)
                else:
                    raise channel_exc
            # Inject anti-fingerprint scripts before any page navigation.
            # This hides common Playwright automation signals that sites like
            # Google Scholar use to detect automated browsers.
            await context.add_init_script("""() => {
                // Hide webdriver flag
                Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
                // Provide realistic plugins array
                Object.defineProperty(navigator, 'plugins', {
                    get: () => [1, 2, 3, 4, 5],
                });
                // Provide realistic languages
                Object.defineProperty(navigator, 'languages', {
                    get: () => ['en-US', 'en'],
                });
                // Mask chrome runtime to avoid detection
                window.chrome = { runtime: {} };
                // Override permissions query to avoid detection
                const origQuery = window.navigator.permissions.query;
                window.navigator.permissions.query = (params) => (
                    params.name === 'notifications'
                        ? Promise.resolve({ state: Notification.permission })
                        : origQuery(params)
                );
            }""")
            page = context.pages[0] if context.pages else await context.new_page()
            return BrowserPageSession(playwright=playwright, context=context, page=page)
        except Exception as exc:
            await playwright.stop()
            raise BrowserRuntimeError("BROWSER_LAUNCH_FAILED", str(exc)) from exc


class BrowserRuntimeError(RuntimeError):
    def __init__(self, code: str, message: str):
        super().__init__(f"{code}: {message}")
        self.code = code
        self.message = message


class BrowserPageSession:
    def __init__(self, playwright: Any, context: Any, page: Any):
        self.playwright = playwright
        self.context = context
        self.page = page

    async def __aenter__(self) -> Any:
        return self.page

    async def __aexit__(self, exc_type: object, exc: object, traceback: object) -> None:
        await self.close()

    async def close(self) -> None:
        try:
            await self.context.close()
        finally:
            await self.playwright.stop()
