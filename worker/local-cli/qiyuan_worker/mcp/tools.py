"""MCP tool definitions for browser automation.

Each tool is a thin wrapper around Playwright Page operations, reusing
the existing BrowserRuntime and BrowserPageSession from the worker
browser module.  All tools share a single lazily-initialized browser
session managed by the MCP server.
"""

from __future__ import annotations

import base64
import json
from dataclasses import dataclass, field
from typing import Any

# ---------------------------------------------------------------------------
# Tool metadata registry
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class ToolDef:
    """Schema for a single MCP tool."""

    name: str
    description: str
    input_schema: dict[str, Any]


TOOLS: tuple[ToolDef, ...] = (
    ToolDef(
        name="browser_navigate",
        description="Navigate the browser to a URL and return the page title and final URL.",
        input_schema={
            "type": "object",
            "properties": {
                "url": {"type": "string", "description": "The URL to navigate to."},
            },
            "required": ["url"],
        },
    ),
    ToolDef(
        name="browser_screenshot",
        description="Take a screenshot of the current page. Returns a base64-encoded PNG image.",
        input_schema={
            "type": "object",
            "properties": {
                "full_page": {
                    "type": "boolean",
                    "description": "Capture the full scrollable page. Defaults to false (viewport only).",
                    "default": False,
                },
            },
        },
    ),
    ToolDef(
        name="browser_get_content",
        description="Get the text content of the current page or a specific element.",
        input_schema={
            "type": "object",
            "properties": {
                "selector": {
                    "type": "string",
                    "description": "Optional CSS selector to scope content extraction. If omitted, returns the full page text.",
                },
            },
        },
    ),
    ToolDef(
        name="browser_click",
        description="Click an element on the page identified by a CSS selector.",
        input_schema={
            "type": "object",
            "properties": {
                "selector": {"type": "string", "description": "CSS selector of the element to click."},
            },
            "required": ["selector"],
        },
    ),
    ToolDef(
        name="browser_fill",
        description="Fill a form input element with a value.",
        input_schema={
            "type": "object",
            "properties": {
                "selector": {"type": "string", "description": "CSS selector of the input element."},
                "value": {"type": "string", "description": "The value to fill."},
            },
            "required": ["selector", "value"],
        },
    ),
    ToolDef(
        name="browser_evaluate",
        description="Execute a JavaScript expression in the page context and return the result.",
        input_schema={
            "type": "object",
            "properties": {
                "expression": {"type": "string", "description": "JavaScript expression to evaluate."},
            },
            "required": ["expression"],
        },
    ),
    ToolDef(
        name="browser_wait",
        description="Wait for an element to appear on the page, or wait for a fixed duration.",
        input_schema={
            "type": "object",
            "properties": {
                "selector": {
                    "type": "string",
                    "description": "CSS selector to wait for. If omitted, waits for timeout_ms.",
                },
                "timeout_ms": {
                    "type": "integer",
                    "description": "Maximum time to wait in milliseconds. Defaults to 5000.",
                    "default": 5000,
                },
            },
        },
    ),
    ToolDef(
        name="browser_tabs",
        description="List all open browser tabs with their titles and URLs.",
        input_schema={
            "type": "object",
            "properties": {},
        },
    ),
    ToolDef(
        name="browser_close",
        description="Close the browser session. A new session will be created on the next tool call.",
        input_schema={
            "type": "object",
            "properties": {},
        },
    ),
)


# ---------------------------------------------------------------------------
# Tool executor
# ---------------------------------------------------------------------------

@dataclass
class BrowserToolContext:
    """Holds the lazily-initialized browser session shared by all tools."""

    _session: Any = field(default=None, repr=False)
    _runtime: Any = field(default=None, repr=False)

    @property
    def has_session(self) -> bool:
        return self._session is not None

    async def get_page(self) -> Any:
        """Return the Playwright Page, creating a session if needed."""
        if self._session is None:
            if self._runtime is None:
                raise RuntimeError("BrowserToolContext.runtime not configured")
            self._session = await self._runtime.open_page()
        return self._session.page

    async def close(self) -> None:
        if self._session is not None:
            await self._session.close()
            self._session = None


async def execute_tool(ctx: BrowserToolContext, name: str, arguments: dict[str, Any]) -> list[dict[str, Any]]:
    """Execute a named tool and return MCP-compatible content blocks."""
    handler = _HANDLERS.get(name)
    if handler is None:
        return [{"type": "text", "text": f"Unknown tool: {name}"}]
    try:
        return await handler(ctx, arguments)
    except Exception as exc:
        return [{"type": "text", "text": f"Error executing {name}: {exc}"}]


# ---------------------------------------------------------------------------
# Individual tool handlers
# ---------------------------------------------------------------------------

async def _navigate(ctx: BrowserToolContext, args: dict[str, Any]) -> list[dict[str, Any]]:
    url = args.get("url", "")
    if not url:
        return [{"type": "text", "text": "Error: url is required"}]
    page = await ctx.get_page()
    response = await page.goto(url, wait_until="domcontentloaded", timeout=30_000)
    status = response.status if response else "unknown"
    title = await page.title()
    return [{"type": "text", "text": json.dumps({
        "title": title,
        "url": page.url,
        "status": status,
    }, ensure_ascii=False)}]


async def _screenshot(ctx: BrowserToolContext, args: dict[str, Any]) -> list[dict[str, Any]]:
    page = await ctx.get_page()
    full_page = args.get("full_page", False)
    screenshot_bytes = await page.screenshot(full_page=full_page, type="png")
    b64 = base64.b64encode(screenshot_bytes).decode("ascii")
    return [{"type": "image", "data": b64, "mimeType": "image/png"}]


async def _get_content(ctx: BrowserToolContext, args: dict[str, Any]) -> list[dict[str, Any]]:
    page = await ctx.get_page()
    selector = args.get("selector")
    if selector:
        element = await page.query_selector(selector)
        if element is None:
            return [{"type": "text", "text": f"No element found for selector: {selector}"}]
        text = await element.inner_text()
    else:
        text = await page.inner_text("body")
    # Truncate very long content to avoid overwhelming the LLM context.
    max_chars = 50_000
    if len(text) > max_chars:
        text = text[:max_chars] + f"\n\n... (truncated, total {len(text)} chars)"
    return [{"type": "text", "text": text}]


async def _click(ctx: BrowserToolContext, args: dict[str, Any]) -> list[dict[str, Any]]:
    selector = args.get("selector", "")
    if not selector:
        return [{"type": "text", "text": "Error: selector is required"}]
    page = await ctx.get_page()
    await page.click(selector, timeout=10_000)
    return [{"type": "text", "text": f"Clicked: {selector}"}]


async def _fill(ctx: BrowserToolContext, args: dict[str, Any]) -> list[dict[str, Any]]:
    selector = args.get("selector", "")
    value = args.get("value", "")
    if not selector:
        return [{"type": "text", "text": "Error: selector is required"}]
    page = await ctx.get_page()
    await page.fill(selector, value, timeout=10_000)
    return [{"type": "text", "text": f"Filled '{selector}' with value (length={len(value)})"}]


async def _evaluate(ctx: BrowserToolContext, args: dict[str, Any]) -> list[dict[str, Any]]:
    expression = args.get("expression", "")
    if not expression:
        return [{"type": "text", "text": "Error: expression is required"}]
    page = await ctx.get_page()
    result = await page.evaluate(expression)
    return [{"type": "text", "text": json.dumps(result, ensure_ascii=False, default=str)}]


async def _wait(ctx: BrowserToolContext, args: dict[str, Any]) -> list[dict[str, Any]]:
    page = await ctx.get_page()
    selector = args.get("selector")
    timeout_ms = args.get("timeout_ms", 5000)
    if selector:
        await page.wait_for_selector(selector, timeout=timeout_ms)
        return [{"type": "text", "text": f"Element appeared: {selector}"}]
    else:
        await page.wait_for_timeout(timeout_ms)
        return [{"type": "text", "text": f"Waited {timeout_ms}ms"}]


async def _tabs(ctx: BrowserToolContext, _args: dict[str, Any]) -> list[dict[str, Any]]:
    if not ctx.has_session or ctx._session is None:
        return [{"type": "text", "text": "No browser session active."}]
    pages = ctx._session.context.pages
    tabs = []
    for i, p in enumerate(pages):
        tabs.append({"index": i, "url": p.url, "title": await p.title()})
    return [{"type": "text", "text": json.dumps(tabs, ensure_ascii=False)}]


async def _close(ctx: BrowserToolContext, _args: dict[str, Any]) -> list[dict[str, Any]]:
    await ctx.close()
    return [{"type": "text", "text": "Browser session closed."}]


_HANDLERS: dict[str, Any] = {
    "browser_navigate": _navigate,
    "browser_screenshot": _screenshot,
    "browser_get_content": _get_content,
    "browser_click": _click,
    "browser_fill": _fill,
    "browser_evaluate": _evaluate,
    "browser_wait": _wait,
    "browser_tabs": _tabs,
    "browser_close": _close,
}
