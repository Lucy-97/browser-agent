from __future__ import annotations

from pathlib import Path
import tempfile
import unittest

from qiyuan_worker.adapters.generic.browser_agent import GenericBrowserAgentAdapter, _build_task_summary
from qiyuan_worker.artifacts import ArtifactCollector
from qiyuan_worker.config import write_default_config
from qiyuan_worker.protocols import AutomationJob
from qiyuan_worker.runtime.context import JobContext

from test_browser_agent import FakeAgentPage


class FakeRuntimeSession:
    def __init__(self, page: FakeAgentPage):
        self.page = page

    async def __aenter__(self) -> FakeAgentPage:
        return self.page

    async def __aexit__(self, exc_type: object, exc: object, traceback: object) -> None:
        return None


class FakeRuntime:
    def __init__(self, page: FakeAgentPage):
        self.page = page

    async def open_page(self) -> FakeRuntimeSession:
        return FakeRuntimeSession(self.page)


class FakeProvider:
    def complete_json(self, request: dict):
        return {
            "actions": [
                {"action": "observe_page"},
                {"action": "fill", "selector": "#search", "value": "LiFePO4"},
                {"action": "press", "key": "Enter"},
                {"action": "extract", "selector": ".result"},
                {"action": "screenshot", "name": "final", "overlay": True},
            ]
        }


class FakeClient:
    def __init__(self) -> None:
        self.checkpoints: list[dict] = []

    def run_checkpoint(self, run_id: str, payload: dict) -> dict:
        self.checkpoints.append(payload)
        return {"checkpoint_id": f"chk_{len(self.checkpoints)}"}

    def run_status(self, run_id: str) -> dict:
        return {"run_id": run_id, "status": "running"}


class GenericBrowserAgentAdapterTest(unittest.IsolatedAsyncioTestCase):
    def test_social_ops_summary_does_not_use_copyright_fields(self) -> None:
        summary = _build_task_summary(
            task="前往该社交平台，搜索主题 'xdd'。提取最新热点",
            stopped_reason="stop_action",
            last_error=None,
            extracts=[{"fields": {"hot_topics": ["Xdd", "Delusion xdd"]}}],
            confirmed_findings=[],
            candidate_findings=["Xdd"],
        )

        self.assertEqual(summary["conclusion"], "本次任务已完成，并生成结构化抽取结果。")
        self.assertNotIn("detected", summary)
        self.assertNotIn("findings", summary)
        self.assertNotIn("侵权", summary["conclusion"])

    def test_copyright_summary_keeps_detection_fields(self) -> None:
        summary = _build_task_summary(
            task="围绕关键词 '玫瑰的故事' 开展侵权取证",
            stopped_reason="stop_action",
            last_error=None,
            extracts=[],
            confirmed_findings=[],
            candidate_findings=["example.com"],
        )

        self.assertEqual(summary["detected"], 1)
        self.assertEqual(summary["candidates"], 1)
        self.assertEqual(summary["findings"], ["example.com"])
        self.assertIn("候选命中项", summary["conclusion"])

    async def test_llm_plan_mode_executes_actions_and_collects_artifacts(self) -> None:
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
                llm_provider="mock",
                llm_model="fake",
            )
            page = FakeAgentPage()
            adapter = GenericBrowserAgentAdapter(
                browser_runtime_factory=lambda browser_config: FakeRuntime(page),
                llm_provider_factory=lambda provider_config: FakeProvider(),
            )
            job = AutomationJob(
                job_id="job_1",
                run_id="run_1",
                job_type="generic.browser.agent",
                adapter="generic.browser_agent",
                target={"allowed_domains": ["example.com"]},
                input={"url": "https://example.com/search", "task": "search LiFePO4", "mode": "llm_plan"},
                policy={
                    "headed": False,
                    "allowed_actions": ["observe_page", "fill", "press", "extract", "screenshot"],
                    "action_timeout_seconds": 5,
                },
            )
            context = JobContext(
                job=job,
                config=config,
                api_client=FakeClient(),
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            result = await adapter.run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(result.summary["mode"], "llm_plan")
            self.assertGreaterEqual(len(context.api_client.checkpoints), 2)
            artifact_types = {artifact.artifact_type for artifact in context.artifact_collector.collected()}
            self.assertIn("agent_trace", artifact_types)
            self.assertIn("screenshot", artifact_types)


if __name__ == "__main__":
    unittest.main()
