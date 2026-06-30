from __future__ import annotations

import asyncio
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Awaitable, Callable

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.protocols import AdapterResult


BrowserActCommandRunner = Callable[[list[str]], Awaitable[str]]


@dataclass(frozen=True)
class BrowserActState:
    title: str = ""
    url: str = ""
    text: str = ""
    raw: dict[str, Any] | None = None


class BrowserActAdapter(AutomationAdapter):
    name = "browser.act"
    supported_job_types = ("generic.browser.act",)
    required_capabilities = ("adapter.browser.act",)

    def __init__(self, command_runner: BrowserActCommandRunner | None = None):
        self.command_runner = command_runner or _run_browser_act_command

    async def run(self, context) -> AdapterResult:
        url = str(context.job.input.get("url") or "").strip()
        if not url:
            return AdapterResult.failed("BROWSER_ACT_URL_REQUIRED", "input.url is required", retryable=False)

        session_name = str(context.job.run_id or context.job.job_id)
        context.work_dir.mkdir(parents=True, exist_ok=True)
        try:
            await self._run_command(["browser-act", "--session", session_name, "browser", "open", url])
            state_raw = await self._run_command(["browser-act", "--session", session_name, "state"])
            screenshot_path = context.work_dir / "browser-act.png"
            screenshot_path.parent.mkdir(parents=True, exist_ok=True)
            await self._run_command(["browser-act", "--session", session_name, "screenshot", str(screenshot_path)])
        except Exception as exc:
            return AdapterResult.failed("BROWSER_ACT_RUNTIME_ERROR", str(exc), retryable=True)

        state = _parse_state(state_raw)
        summary = {
            "adapter": self.name,
            "mode": "cli",
            "url": state.url or url,
            "title": state.title,
            "text": state.text,
            "conclusion": "browser-act CLI prototype executed successfully.",
        }
        artifact_path = context.work_dir / "browser-act.png"
        if artifact_path.exists():
            context.artifact_collector.add_file(
                "screenshot",
                artifact_path,
                metadata={"url": summary["url"], "adapter": self.name},
            )
        context.artifact_collector.add_file(
            "agent_trace",
            _write_trace(context.work_dir, session_name, summary, state.raw),
            metadata={"url": summary["url"], "adapter": self.name},
        )
        return AdapterResult.completed(summary=summary, cursor={"source": self.name, "url": summary["url"]})

    async def _run_command(self, args: list[str]) -> str:
        result = self.command_runner(args)
        if asyncio.iscoroutine(result):
            return await result
        return result  # type: ignore[return-value]


async def _run_browser_act_command(args: list[str]) -> str:
    proc = await asyncio.create_subprocess_exec(
        *args,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )
    stdout, stderr = await proc.communicate()
    if proc.returncode != 0:
        raise RuntimeError((stderr or stdout).decode("utf-8", errors="replace").strip() or f"browser-act exited {proc.returncode}")
    return (stdout or b"").decode("utf-8", errors="replace").strip()


def _parse_state(raw: str) -> BrowserActState:
    text = raw.strip()
    if not text:
        return BrowserActState(raw={})
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        return BrowserActState(text=text, raw={"raw": text})
    return BrowserActState(
        title=str(payload.get("title") or ""),
        url=str(payload.get("url") or ""),
        text=str(payload.get("text") or payload.get("markdown") or ""),
        raw=payload if isinstance(payload, dict) else {"raw": payload},
    )


def _write_trace(work_dir: Path, session_name: str, summary: dict[str, Any], state: dict[str, Any] | None) -> Path:
    trace_path = work_dir / "browser-act-trace.json"
    trace_path.write_text(
        json.dumps({"session": session_name, "summary": summary, "state": state}, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    return trace_path
