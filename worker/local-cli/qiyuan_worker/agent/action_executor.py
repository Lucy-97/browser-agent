from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Awaitable, Callable, TypeVar

from .action_schema import AgentAction
from .observer import PageObservation, observe_page
from .policy import action_timeout_seconds, ensure_action_allowed, ensure_url_allowed
from .redaction import redact_value
from .tools import (
    click_element,
    click_selector,
    download_file,
    extract_text,
    fill_selector,
    press_key,
    screenshot_page,
    wait_for_condition,
)
from .trace import AgentTrace


T = TypeVar("T")
CheckpointCallback = Callable[[dict[str, Any]], None]
CancelCheckCallback = Callable[[], bool]


class AgentRunCancelled(RuntimeError):
    def __init__(self, message: str = "run was cancelled"):
        super().__init__(message)
        self.code = "RUN_CANCELLED"
        self.message = message


@dataclass(frozen=True)
class AgentActionExecutionResult:
    summary: dict[str, Any]
    screenshots: tuple[Path, ...] = ()
    downloads: tuple[dict[str, Any], ...] = ()
    last_observation: PageObservation | None = None


@dataclass
class _ExecutionState:
    observations: list[PageObservation] = field(default_factory=list)
    screenshots: list[Path] = field(default_factory=list)
    downloads: list[dict[str, Any]] = field(default_factory=list)
    extracts: list[dict[str, Any]] = field(default_factory=list)


class BrowserActionExecutor:
    def __init__(
        self,
        downloads_dir: Path,
        checkpoint: CheckpointCallback | None = None,
        should_cancel: CancelCheckCallback | None = None,
    ):
        self.downloads_dir = downloads_dir
        self.checkpoint = checkpoint
        self.should_cancel = should_cancel

    async def execute(
        self,
        page: Any,
        trace: AgentTrace,
        actions: tuple[AgentAction, ...],
        policy: dict[str, object],
    ) -> AgentActionExecutionResult:
        state = _ExecutionState()
        action_error: dict[str, Any] | None = None
        for index, action in enumerate(actions):
            self._raise_if_cancelled()
            trace.add(
                "action.start",
                {"index": index, "action": action.action, "params": redact_value(action.params)},
            )
            try:
                result = await _run_action(
                    action.action,
                    policy,
                    self._execute_one(page, trace, action, policy, state, index),
                )
            except AgentRunCancelled:
                # Cancellation must abort the whole run, not be fed back to the planner.
                raise
            except Exception as exc:
                trace.add(
                    "action.error",
                    {"index": index, "action": action.action, "error": str(exc), "error_type": type(exc).__name__},
                )
                self._checkpoint({"step": "action.error", "index": index, "action": action.action, "error": str(exc)})
                # Record the failure and stop this batch, but return a partial result so the
                # agent loop can feed the error back to the planner and recover next iteration.
                action_error = {
                    "error": str(exc),
                    "error_type": type(exc).__name__,
                    "failed_action": action.action,
                    "failed_index": index,
                }
                break
            trace.add("action.result", {"index": index, "action": action.action, "result": redact_value(result)})
            self._checkpoint({"step": "action.completed", "index": index, "action": action.action, "result": result})

        last_observation = state.observations[-1] if state.observations else None
        summary: dict[str, Any] = {
            "action_count": len(actions),
            "extract_count": len(state.extracts),
            "download_count": len(state.downloads),
            "screenshot_count": len(state.screenshots),
            "url": last_observation.url if last_observation else str(getattr(page, "url", "")),
            "title": last_observation.title if last_observation else "",
            "extracts": state.extracts[-5:],
        }
        if action_error is not None:
            summary.update(action_error)
        return AgentActionExecutionResult(
            summary=summary,
            screenshots=tuple(state.screenshots),
            downloads=tuple(state.downloads),
            last_observation=last_observation,
        )

    async def _execute_one(
        self,
        page: Any,
        trace: AgentTrace,
        action: AgentAction,
        policy: dict[str, object],
        state: _ExecutionState,
        index: int,
    ) -> dict[str, Any]:
        params = action.params
        if action.action == "observe_page":
            observation = await observe_page(page)
            state.observations.append(observation)
            return observation.to_dict()
        if action.action == "click":
            return await click_selector(page, str(params["selector"]))
        if action.action == "click_element":
            return await click_element(page, int(params["index"]))
        if action.action == "fill":
            return await fill_selector(page, str(params["selector"]), str(params["value"]))
        if action.action == "press":
            return await press_key(page, str(params["key"]))
        if action.action == "extract":
            fields = params.get("fields")
            if isinstance(fields, dict) and fields:
                result = {"fields": fields}
            else:
                result = await extract_text(page, selector=params.get("selector"))
            state.extracts.append(result)
            return result
        if action.action == "screenshot":
            name = str(params.get("name") or f"agent-action-{index}")
            path = trace.work_dir / f"{_safe_name(name)}.png"
            result = await screenshot_page(page, path, overlay=bool(params.get("overlay")))
            state.screenshots.append(path)
            return result
        if action.action == "wait_for":
            return await wait_for_condition(
                page,
                condition=params.get("condition"),
                timeout_ms=params.get("timeout_ms"),
            )
        if action.action == "download":
            url = params.get("url")
            if url:
                allowed_domains = policy.get("allowed_domains") or []
                if isinstance(allowed_domains, list) and all(isinstance(item, str) for item in allowed_domains):
                    ensure_url_allowed(str(url), allowed_domains)
            max_bytes = int(policy.get("max_download_bytes") or 25 * 1024 * 1024)
            result = await download_file(
                page=page,
                downloads_dir=self.downloads_dir,
                selector=params.get("selector"),
                url=str(url) if url else None,
                max_bytes=max_bytes,
            )
            state.downloads.append(result)
            return result
        if action.action == "stop":
            return {"reason": params.get("reason", "task finished")}
        raise RuntimeError(f"unsupported action {action.action}")

    def _checkpoint(self, payload: dict[str, Any]) -> None:
        if self.checkpoint:
            self.checkpoint(redact_value(payload))

    def _raise_if_cancelled(self) -> None:
        if self.should_cancel and self.should_cancel():
            raise AgentRunCancelled()


async def _run_action(action: str, policy: dict[str, object], awaitable: Awaitable[T]) -> T:
    try:
        ensure_action_allowed(action, policy)
    except Exception:
        close = getattr(awaitable, "close", None)
        if callable(close):
            close()
        raise
    return await asyncio.wait_for(awaitable, timeout=action_timeout_seconds(policy))


def _safe_name(value: str) -> str:
    safe = "".join(ch for ch in value if ch.isalnum() or ch in {"-", "_"}).strip()
    return safe or "screenshot"
