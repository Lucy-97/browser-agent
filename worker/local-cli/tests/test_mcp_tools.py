"""Unit tests for MCP browser tools.

All tests mock the Playwright Page to avoid a real browser dependency.
"""

from __future__ import annotations

import asyncio
import json
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

from qiyuan_worker.mcp.tools import (
    TOOLS,
    BrowserToolContext,
    execute_tool,
)


def run(coro):
    """Helper to run a coroutine synchronously."""
    return asyncio.get_event_loop().run_until_complete(coro)


class ToolRegistryTest(unittest.TestCase):
    """Verify the static TOOLS tuple."""

    def test_all_tools_have_unique_names(self) -> None:
        names = [t.name for t in TOOLS]
        self.assertEqual(len(names), len(set(names)))

    def test_expected_tool_count(self) -> None:
        self.assertEqual(len(TOOLS), 9)

    def test_all_tools_have_input_schema(self) -> None:
        for tool in TOOLS:
            self.assertIn("type", tool.input_schema)
            self.assertEqual(tool.input_schema["type"], "object")


class BrowserToolContextTest(unittest.TestCase):
    """Verify lazy session management."""

    def test_has_session_initially_false(self) -> None:
        ctx = BrowserToolContext()
        self.assertFalse(ctx.has_session)

    def test_get_page_raises_without_runtime(self) -> None:
        ctx = BrowserToolContext()
        with self.assertRaises(RuntimeError):
            run(ctx.get_page())

    def test_close_is_idempotent(self) -> None:
        ctx = BrowserToolContext()
        # Closing without a session should not raise.
        run(ctx.close())


def _make_ctx_with_mock_page():
    """Create a BrowserToolContext with a mock page and runtime."""
    mock_page = AsyncMock()
    mock_page.url = "https://example.com"
    mock_page.title = AsyncMock(return_value="Example")
    mock_page.inner_text = AsyncMock(return_value="Hello World")

    mock_session = AsyncMock()
    mock_session.page = mock_page
    mock_session.context = MagicMock()
    mock_session.context.pages = [mock_page]

    mock_runtime = MagicMock()
    mock_runtime.open_page = AsyncMock(return_value=mock_session)

    ctx = BrowserToolContext()
    ctx._runtime = mock_runtime
    return ctx, mock_page


class NavigateToolTest(unittest.TestCase):
    def test_navigate_returns_title_and_url(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_response = MagicMock()
        mock_response.status = 200
        mock_page.goto = AsyncMock(return_value=mock_response)

        result = run(execute_tool(ctx, "browser_navigate", {"url": "https://example.com"}))

        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["type"], "text")
        data = json.loads(result[0]["text"])
        self.assertEqual(data["title"], "Example")
        self.assertEqual(data["status"], 200)

    def test_navigate_missing_url_returns_error(self) -> None:
        ctx, _ = _make_ctx_with_mock_page()
        result = run(execute_tool(ctx, "browser_navigate", {}))
        self.assertIn("Error", result[0]["text"])


class ScreenshotToolTest(unittest.TestCase):
    def test_screenshot_returns_image(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_page.screenshot = AsyncMock(return_value=b"\x89PNG\r\n")

        result = run(execute_tool(ctx, "browser_screenshot", {}))

        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["type"], "image")
        self.assertEqual(result[0]["mimeType"], "image/png")
        self.assertIn("data", result[0])


class GetContentToolTest(unittest.TestCase):
    def test_get_content_full_page(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        result = run(execute_tool(ctx, "browser_get_content", {}))

        self.assertEqual(result[0]["type"], "text")
        self.assertIn("Hello World", result[0]["text"])

    def test_get_content_with_selector(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_element = AsyncMock()
        mock_element.inner_text = AsyncMock(return_value="Scoped Content")
        mock_page.query_selector = AsyncMock(return_value=mock_element)

        result = run(execute_tool(ctx, "browser_get_content", {"selector": "#main"}))

        self.assertIn("Scoped Content", result[0]["text"])

    def test_get_content_selector_not_found(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_page.query_selector = AsyncMock(return_value=None)

        result = run(execute_tool(ctx, "browser_get_content", {"selector": "#missing"}))

        self.assertIn("No element found", result[0]["text"])


class ClickToolTest(unittest.TestCase):
    def test_click_success(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_page.click = AsyncMock()

        result = run(execute_tool(ctx, "browser_click", {"selector": "button.submit"}))

        self.assertIn("Clicked", result[0]["text"])
        mock_page.click.assert_awaited_once()

    def test_click_missing_selector(self) -> None:
        ctx, _ = _make_ctx_with_mock_page()
        result = run(execute_tool(ctx, "browser_click", {}))
        self.assertIn("Error", result[0]["text"])


class FillToolTest(unittest.TestCase):
    def test_fill_success(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_page.fill = AsyncMock()

        result = run(execute_tool(ctx, "browser_fill", {"selector": "input#name", "value": "Alice"}))

        self.assertIn("Filled", result[0]["text"])
        mock_page.fill.assert_awaited_once_with("input#name", "Alice", timeout=10_000)


class EvaluateToolTest(unittest.TestCase):
    def test_evaluate_returns_result(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_page.evaluate = AsyncMock(return_value=42)

        result = run(execute_tool(ctx, "browser_evaluate", {"expression": "1 + 1"}))

        self.assertEqual(result[0]["text"], "42")


class WaitToolTest(unittest.TestCase):
    def test_wait_for_selector(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_page.wait_for_selector = AsyncMock()

        result = run(execute_tool(ctx, "browser_wait", {"selector": "#loaded", "timeout_ms": 3000}))

        self.assertIn("appeared", result[0]["text"])
        mock_page.wait_for_selector.assert_awaited_once_with("#loaded", timeout=3000)

    def test_wait_for_timeout(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        mock_page.wait_for_timeout = AsyncMock()

        result = run(execute_tool(ctx, "browser_wait", {"timeout_ms": 1000}))

        self.assertIn("Waited", result[0]["text"])


class TabsToolTest(unittest.TestCase):
    def test_tabs_returns_list(self) -> None:
        ctx, mock_page = _make_ctx_with_mock_page()
        # Force session to be initialized
        run(ctx.get_page())

        result = run(execute_tool(ctx, "browser_tabs", {}))

        tabs = json.loads(result[0]["text"])
        self.assertIsInstance(tabs, list)
        self.assertEqual(len(tabs), 1)

    def test_tabs_no_session(self) -> None:
        ctx = BrowserToolContext()
        result = run(execute_tool(ctx, "browser_tabs", {}))
        self.assertIn("No browser session", result[0]["text"])


class CloseToolTest(unittest.TestCase):
    def test_close_clears_session(self) -> None:
        ctx, _ = _make_ctx_with_mock_page()
        run(ctx.get_page())
        self.assertTrue(ctx.has_session)

        result = run(execute_tool(ctx, "browser_close", {}))

        self.assertIn("closed", result[0]["text"])
        self.assertFalse(ctx.has_session)


class UnknownToolTest(unittest.TestCase):
    def test_unknown_tool_returns_error(self) -> None:
        ctx = BrowserToolContext()
        result = run(execute_tool(ctx, "nonexistent_tool", {}))
        self.assertIn("Unknown tool", result[0]["text"])


if __name__ == "__main__":
    unittest.main()
