from __future__ import annotations

from pathlib import Path
import tempfile
import unittest

from qiyuan_worker.adapters import build_default_registry, load_worker_extensions
from qiyuan_worker.config import write_default_config
from qiyuan_worker.job_loop import detect_capabilities
from qiyuan_worker.protocols import AutomationJob


class WorkerExtensionTest(unittest.TestCase):
    def test_core_product_registers_mock_and_manual_adapters(self) -> None:
        registry = build_default_registry(enabled_products=("core",))
        job = AutomationJob(
            job_id="job_1",
            run_id="run_1",
            job_type="mock.echo",
            adapter="mock.echo",
        )

        adapter = registry.resolve(job, {"adapter.mock.echo"})

        self.assertEqual(adapter.name, "mock.echo")
        self.assertEqual([item.name for item in registry.adapters()], ["mock.echo", "manual"])

    def test_builtin_extensions_expose_policy_templates(self) -> None:
        extensions = load_worker_extensions(enabled_products=("browser_agent",), include_entry_points=False)
        templates = [template.to_dict() for extension in extensions for template in extension.policy_templates]

        self.assertEqual(templates[0]["name"], "browser_agent.generic.llm_plan")
        self.assertEqual(templates[0]["adapter"], "generic.browser_agent")
        self.assertIn("click_element", templates[0]["policy"]["allowed_actions"])

    def test_social_product_registers_upload_templates(self) -> None:
        extensions = load_worker_extensions(enabled_products=("social",), include_entry_points=False)
        templates = [template.to_dict() for extension in extensions for template in extension.policy_templates]

        self.assertEqual({item["adapter"] for item in templates}, {"social.youtube.upload_video", "social.tiktok.upload_video"})
        self.assertTrue(all(item["policy"]["manual_publish_required"] for item in templates))

    def test_capabilities_follow_enabled_products(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config = write_default_config(
                config_path=tmp_path / "config.yaml",
                data_dir=tmp_path / "data",
            )
            core_config = config.__class__(
                server=config.server,
                data_dir=config.data_dir,
                log_level=config.log_level,
                poll_interval_seconds=config.poll_interval_seconds,
                heartbeat_interval_seconds=config.heartbeat_interval_seconds,
                enabled_products=("core",),
                llm_provider=config.llm_provider,
                llm_model=config.llm_model,
            )

            capabilities = detect_capabilities(core_config)

            self.assertIn("adapter.mock.echo", capabilities)


if __name__ == "__main__":
    unittest.main()
