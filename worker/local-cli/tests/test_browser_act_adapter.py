from __future__ import annotations

from pathlib import Path
import tempfile
import unittest

from qiyuan_worker.adapters.browser_act import BrowserActAdapter
from qiyuan_worker.artifacts import ArtifactCollector
from qiyuan_worker.config import write_default_config
from qiyuan_worker.protocols import AutomationJob
from qiyuan_worker.runtime.context import JobContext


class FakeBrowserActRunner:
    def __init__(self) -> None:
        self.commands: list[list[str]] = []

    async def __call__(self, args: list[str]) -> str:
        self.commands.append(args)
        if args[:3] == ["browser-act", "--session", "run_1"] and args[3:] == ["browser", "open", "https://example.com"]:
            return "opened"
        if args[:3] == ["browser-act", "--session", "run_1"] and args[3:] == ["state"]:
            return '{"title":"Example Domain","url":"https://example.com","elements":[{"index":1,"tag":"a","text":"More information"}]}'
        if args[:3] == ["browser-act", "--session", "run_1"] and args[3] == "screenshot":
            Path(args[-1]).write_text("fake image", encoding="utf-8")
            return "saved screenshot"
        raise AssertionError(f"unexpected command: {args}")


class BrowserActAdapterTest(unittest.IsolatedAsyncioTestCase):
    async def test_run_opens_page_collects_state_and_screenshot(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            config = config.__class__(
                server=config.server,
                data_dir=config.data_dir,
                log_level=config.log_level,
                poll_interval_seconds=config.poll_interval_seconds,
                heartbeat_interval_seconds=config.heartbeat_interval_seconds,
                enabled_products=config.enabled_products,
                llm_provider="disabled",
                llm_model="",
            )
            runner = FakeBrowserActRunner()
            adapter = BrowserActAdapter(command_runner=runner)
            job = AutomationJob(
                job_id="job_1",
                run_id="run_1",
                job_type="generic.browser.act",
                adapter="browser.act",
                input={"url": "https://example.com", "task": "open example"},
                policy={},
            )
            context = JobContext(
                job=job,
                config=config,
                api_client=object(),
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            result = await adapter.run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(result.summary["title"], "Example Domain")
            self.assertEqual(result.summary["url"], "https://example.com")
            self.assertEqual(result.summary["adapter"], "browser.act")
            self.assertGreaterEqual(len(runner.commands), 3)


if __name__ == "__main__":
    unittest.main()
