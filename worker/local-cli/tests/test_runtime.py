from __future__ import annotations

from pathlib import Path
import tempfile
import unittest

from qiyuan_worker.adapters import AdapterRegistry, build_default_registry
from qiyuan_worker.config import write_default_config
from qiyuan_worker.job_loop import run_once
from qiyuan_worker.models import DeviceInfo
from qiyuan_worker.protocols import AutomationJob
from qiyuan_worker.runtime import JobRunner


class FakeClient:
    def __init__(self) -> None:
        self.heartbeats: list[tuple[str, dict]] = []
        self.checkpoints: list[tuple[str, dict]] = []
        self.artifacts: list[tuple[str, dict]] = []
        self.completions: list[tuple[str, dict]] = []
        self.manual_actions: list[tuple[str, dict]] = []

    def run_heartbeat(self, run_id: str, payload: dict) -> dict:
        self.heartbeats.append((run_id, payload))
        return {"ok": True}

    def run_checkpoint(self, run_id: str, payload: dict) -> dict:
        self.checkpoints.append((run_id, payload))
        return {"checkpoint_id": f"chk_{len(self.checkpoints)}"}

    def create_run_artifact(self, run_id: str, payload: dict) -> dict:
        self.artifacts.append((run_id, payload))
        return {"artifact_id": f"art_{len(self.artifacts)}", **payload}

    def create_manual_action(self, run_id: str, payload: dict) -> dict:
        self.manual_actions.append((run_id, payload))
        return {"manual_action_id": f"act_{len(self.manual_actions)}"}

    def complete_run(self, run_id: str, payload: dict) -> dict:
        self.completions.append((run_id, payload))
        return {"ok": True}


def make_job(**overrides: object) -> AutomationJob:
    values = {
        "job_id": "job_1",
        "run_id": "run_1",
        "job_type": "generic.browser.script",
        "adapter": "mock.echo",
        "target": {"allowed_domains": ["example.com"]},
        "input": {"message": "hello"},
        "policy": {},
        "cursor": None,
    }
    values.update(overrides)
    return AutomationJob(**values)


class RuntimeTest(unittest.IsolatedAsyncioTestCase):
    async def run_job(
        self,
        job: AutomationJob,
        capabilities: set[str] | None = None,
        registry: AdapterRegistry | None = None,
    ):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            client = FakeClient()
            resolved_capabilities = {"adapter.mock.echo"} if capabilities is None else capabilities
            runner = JobRunner(registry or build_default_registry(), resolved_capabilities)

            result = await runner.run(client, config, job)

            return result, client

    async def test_mock_adapter_success_reports_completion_and_artifact(self) -> None:
        result, client = await self.run_job(make_job())

        self.assertEqual(result.status, "completed")
        self.assertEqual(client.completions[0][1]["status"], "completed")
        self.assertEqual(len(client.checkpoints), 1)
        self.assertEqual(len(client.artifacts), 1)

    async def test_mock_adapter_failure_reports_failed(self) -> None:
        result, client = await self.run_job(make_job(input={"force_error": True}))

        self.assertEqual(result.status, "failed")
        self.assertEqual(client.completions[0][1]["error"]["code"], "MOCK_FORCED_ERROR")

    async def test_unknown_adapter_reports_unsupported(self) -> None:
        result, client = await self.run_job(make_job(job_type="mock.echo", adapter="missing.adapter"))

        self.assertEqual(result.status, "failed")
        self.assertEqual(client.completions[0][1]["error"]["code"], "ADAPTER_UNSUPPORTED")

    async def test_missing_capability_reports_mismatch(self) -> None:
        result, client = await self.run_job(make_job(), capabilities=set())

        self.assertEqual(result.status, "failed")
        self.assertEqual(client.completions[0][1]["error"]["code"], "CAPABILITY_MISMATCH")

    async def test_browser_job_requires_allowed_domains(self) -> None:
        result, client = await self.run_job(make_job(target={}))

        self.assertEqual(result.status, "failed")
        self.assertEqual(client.completions[0][1]["error"]["code"], "POLICY_ALLOWED_DOMAINS_REQUIRED")


class FakeLoopClient(FakeClient):
    def __init__(self, job_payload: dict | None) -> None:
        super().__init__()
        self.job_payload = job_payload
        self.device_heartbeats: list[tuple[str, dict]] = []
        self.claim_count = 0

    def device_heartbeat(self, device_id: str, payload: dict) -> dict:
        self.device_heartbeats.append((device_id, payload))
        return {"ok": True}

    def next_automation_job(self, source: str | None = None) -> dict | None:
        self.claim_count += 1
        if self.claim_count > 1:
            return None
        return self.job_payload


class WorkerLoopE2ETest(unittest.TestCase):
    def test_run_once_claims_executes_and_completes_mock_job(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            config = config.__class__(
                server=config.server,
                data_dir=config.data_dir,
                log_level=config.log_level,
                poll_interval_seconds=config.poll_interval_seconds,
                heartbeat_interval_seconds=config.heartbeat_interval_seconds,
                enabled_products=("core",),
                llm_provider=config.llm_provider,
                llm_model=config.llm_model,
            )
            client = FakeLoopClient(
                {
                    "job_id": "job_1",
                    "run_id": "run_1",
                    "job_type": "mock.echo",
                    "adapter": "mock.echo",
                    "input": {"message": "hello"},
                }
            )
            device = DeviceInfo(
                device_id="dev_1",
                name="local",
                platform="darwin-arm64",
                worker_version="0.1.0",
            )

            had_job = run_once(client, config, device)

            self.assertTrue(had_job)
            self.assertEqual(client.completions[0][1]["status"], "completed")
            self.assertEqual(client.artifacts[0][1]["artifact_type"], "mock.summary")
            self.assertIn("adapter.mock.echo", client.device_heartbeats[0][1]["capabilities"])


if __name__ == "__main__":
    unittest.main()
