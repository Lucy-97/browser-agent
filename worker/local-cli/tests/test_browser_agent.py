from __future__ import annotations

from pathlib import Path
import tempfile
import unittest

from qiyuan_worker.agent import AgentRunCancelled, BrowserActionExecutor, BrowserAgentExecutor
from qiyuan_worker.agent.action_schema import validate_action_plan
from qiyuan_worker.agent.policy import AgentPolicyError, ensure_url_allowed
from qiyuan_worker.agent.trace import AgentTrace


class FakeKeyboard:
    def __init__(self, page: "FakeAgentPage") -> None:
        self.page = page

    async def press(self, key: str) -> None:
        if key == "Enter":
            self.page.submitted = True


class FakeLocator:
    def __init__(self, page: "FakeAgentPage", selector: str) -> None:
        self.page = page
        self.selector = selector

    async def inner_text(self, timeout: int | None = None) -> str:
        if self.selector == "body":
            if self.page.submitted:
                return f"Search\nResult: {self.page.query}\nSecond Result"
            return "Search page"
        return ""

    async def all_inner_texts(self) -> list[str]:
        if self.selector == ".result" and self.page.submitted:
            return [f"Result: {self.page.query}", "Second Result"]
        return []

    async def get_attribute(self, name: str) -> str | None:
        if self.selector == "#pdf" and name == "href":
            return "https://example.com/paper.pdf"
        return None


class FakeAgentPage:
    def __init__(self) -> None:
        self.url = "https://example.com/search"
        self.query = ""
        self.submitted = False
        self.keyboard = FakeKeyboard(self)

    async def title(self) -> str:
        return "Demo Search"

    def locator(self, selector: str) -> FakeLocator:
        return FakeLocator(self, selector)

    async def evaluate(self, script: str):
        if "const candidates" in script:
            return "#search"
        return [
            {
                "index": 0,
                "tag": "input",
                "type": "search",
                "role": "",
                "name": "search",
                "text": "",
                "selector": "#search",
            }
        ]

    async def fill(self, selector: str, query: str, timeout: int | None = None) -> None:
        if selector != "#search":
            raise RuntimeError(f"unexpected selector {selector}")
        self.query = query

    async def click(self, selector: str, timeout: int | None = None) -> None:
        if selector in {"#submit", '[data-qiyuan-agent-index="1"]'}:
            self.submitted = True

    async def wait_for_load_state(self, state: str, timeout: int) -> None:
        return None

    async def screenshot(self, path: str, full_page: bool) -> None:
        Path(path).write_bytes(b"fake-agent-screenshot")

    async def goto(self, url: str, wait_until: str, timeout: int) -> object | None:
        self.url = url
        if url.endswith(".pdf"):
            return FakeDownloadResponse(b"%PDF-1.7 fake")
        return None


class FakeDownloadResponse:
    def __init__(self, body: bytes):
        self._body = body

    async def body(self) -> bytes:
        return self._body


class BrowserAgentTest(unittest.IsolatedAsyncioTestCase):
    def test_policy_allows_exact_and_blocks_other_domains(self) -> None:
        ensure_url_allowed("https://example.com/search", ["example.com"])
        with self.assertRaises(AgentPolicyError):
            ensure_url_allowed("https://evil.example/search", ["example.com"])

    async def test_executor_searches_and_writes_trace(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            page = FakeAgentPage()
            trace = AgentTrace(Path(tmp))
            result = await BrowserAgentExecutor().run_search(
                page=page,
                trace=trace,
                query="LiFePO4",
                result_selector=".result",
            )

            self.assertEqual(result.summary["result_count"], 2)
            self.assertIn("LiFePO4", result.summary["results"][0])
            self.assertTrue(Path(result.trace_path).exists())
            self.assertTrue(result.screenshot_path)
            self.assertTrue(Path(result.screenshot_path).exists())

    async def test_executor_blocks_disallowed_action(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            page = FakeAgentPage()
            trace = AgentTrace(Path(tmp))

            with self.assertRaises(AgentPolicyError) as ctx:
                await BrowserAgentExecutor().run_search(
                    page=page,
                    trace=trace,
                    query="LiFePO4",
                    policy={"allowed_actions": ["observe_page"]},
                )

            self.assertEqual(ctx.exception.code, "AGENT_ACTION_BLOCKED")

    async def test_action_executor_runs_llm_plan_actions(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            page = FakeAgentPage()
            trace = AgentTrace(tmp_path / "trace")
            actions = validate_action_plan(
                {
                    "actions": [
                        {"action": "observe_page"},
                        {"action": "fill", "selector": "#search", "value": "LiFePO4"},
                        {"action": "click_element", "index": 1},
                        {"action": "extract", "selector": ".result"},
                        {"action": "screenshot", "name": "overlay", "overlay": True},
                    ]
                },
                policy={"allowed_actions": ["observe_page", "fill", "click_element", "extract", "screenshot"]},
            )

            result = await BrowserActionExecutor(downloads_dir=tmp_path / "downloads").execute(
                page,
                trace,
                actions,
                policy={"allowed_actions": ["observe_page", "fill", "click_element", "extract", "screenshot"]},
            )

            self.assertEqual(result.summary["action_count"], 5)
            self.assertEqual(result.summary["extract_count"], 1)
            self.assertTrue(result.screenshots[0].exists())

    async def test_action_executor_downloads_pdf(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            page = FakeAgentPage()
            trace = AgentTrace(tmp_path / "trace")
            actions = validate_action_plan(
                {"actions": [{"action": "download", "url": "https://example.com/paper.pdf"}]},
                policy={"allowed_actions": ["download"], "allowed_domains": ["example.com"]},
            )

            result = await BrowserActionExecutor(downloads_dir=tmp_path / "downloads").execute(
                page,
                trace,
                actions,
                policy={"allowed_actions": ["download"], "allowed_domains": ["example.com"]},
            )

            self.assertEqual(result.summary["download_count"], 1)
            self.assertTrue(Path(result.downloads[0]["path"]).exists())

    async def test_action_executor_records_error_instead_of_raising(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            page = FakeAgentPage()
            trace = AgentTrace(tmp_path / "trace")
            # FakeAgentPage.fill raises for any selector other than "#search".
            actions = validate_action_plan(
                {
                    "actions": [
                        {"action": "fill", "selector": "#missing", "value": "x"},
                        {"action": "extract", "selector": ".result"},
                    ]
                },
                policy={"allowed_actions": ["fill", "extract"]},
            )

            result = await BrowserActionExecutor(downloads_dir=tmp_path / "downloads").execute(
                page,
                trace,
                actions,
                policy={"allowed_actions": ["fill", "extract"]},
            )

            # The loop must not crash; the failure is reported in the summary so the
            # planner can recover on the next iteration.
            self.assertEqual(result.summary["failed_action"], "fill")
            self.assertEqual(result.summary["failed_index"], 0)
            self.assertIn("error", result.summary)
            # The action after the failure is skipped (batch stops at the error).
            self.assertEqual(result.summary["extract_count"], 0)

    def test_stop_action_allowed_even_when_not_in_policy(self) -> None:
        # `stop` is the mandatory completion signal; it must validate even when the job
        # policy's allowed_actions omits it (regression: AGENT_ACTION_BLOCKED on completion).
        actions = validate_action_plan(
            {"actions": [{"action": "stop", "reason": "done"}]},
            policy={"allowed_actions": ["observe_page", "extract"]},
        )
        self.assertEqual(actions[0].action, "stop")

    async def test_action_executor_stops_when_cancelled(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            page = FakeAgentPage()
            trace = AgentTrace(tmp_path / "trace")
            actions = validate_action_plan(
                {"actions": [{"action": "observe_page"}]},
                policy={"allowed_actions": ["observe_page"]},
            )

            with self.assertRaises(AgentRunCancelled):
                await BrowserActionExecutor(
                    downloads_dir=tmp_path / "downloads",
                    should_cancel=lambda: True,
                ).execute(page, trace, actions, policy={"allowed_actions": ["observe_page"]})


if __name__ == "__main__":
    unittest.main()
