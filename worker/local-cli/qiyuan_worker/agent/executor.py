from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import Any, Awaitable, TypeVar

from .observer import observe_page
from .policy import action_timeout_seconds, ensure_action_allowed
from .tools import extract_results, fill_search, submit_search
from .trace import AgentTrace

T = TypeVar("T")


@dataclass(frozen=True)
class AgentExecutionResult:
    summary: dict[str, Any]
    trace_path: str
    screenshot_path: str | None


class BrowserAgentExecutor:
    async def run_search(
        self,
        page: Any,
        trace: AgentTrace,
        query: str,
        input_selector: str | None = None,
        submit_selector: str | None = None,
        result_selector: str | None = None,
        policy: dict[str, object] | None = None,
    ) -> AgentExecutionResult:
        resolved_policy = policy or {}
        before = await _run_action("observe_page", resolved_policy, observe_page(page))
        trace.add("observe.before", before.to_dict())

        filled_selector = await _run_action(
            "fill",
            resolved_policy,
            fill_search(page, query, input_selector=input_selector),
        )
        trace.add("act.fill", {"selector": filled_selector, "query": query})

        submit_action = "click" if submit_selector else "press"
        submitted_by = await _run_action(
            submit_action,
            resolved_policy,
            submit_search(page, submit_selector=submit_selector),
        )
        trace.add("act.submit", {"selector": submitted_by})

        await _run_action("wait_for", resolved_policy, _wait_after_submit(page))
        after = await _run_action("observe_page", resolved_policy, observe_page(page))
        results = await _run_action(
            "extract",
            resolved_policy,
            extract_results(page, result_selector=result_selector),
        )
        trace.add("observe.after", after.to_dict())
        trace.add("extract.results", {"count": len(results), "items": results[:20]})

        screenshot = await _run_action("screenshot", resolved_policy, trace.screenshot(page, "agent-final"))
        trace_path = trace.write()
        return AgentExecutionResult(
            summary={
                "url": after.url,
                "title": after.title,
                "query": query,
                "result_count": len(results),
                "results": results[:10],
            },
            trace_path=str(trace_path),
            screenshot_path=str(screenshot) if screenshot else None,
        )


async def _wait_after_submit(page: Any) -> None:
    if hasattr(page, "wait_for_load_state"):
        try:
            await page.wait_for_load_state("networkidle", timeout=5000)
            return
        except Exception:
            pass
    if hasattr(page, "wait_for_timeout"):
        await page.wait_for_timeout(500)


async def _run_action(action: str, policy: dict[str, object], awaitable: Awaitable[T]) -> T:
    try:
        ensure_action_allowed(action, policy)
    except Exception:
        close = getattr(awaitable, "close", None)
        if callable(close):
            close()
        raise
    return await asyncio.wait_for(awaitable, timeout=action_timeout_seconds(policy))
