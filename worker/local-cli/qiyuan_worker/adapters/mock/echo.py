from __future__ import annotations

from typing import TYPE_CHECKING

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.protocols import AdapterResult

if TYPE_CHECKING:
    from qiyuan_worker.runtime.context import JobContext


class MockEchoAdapter(AutomationAdapter):
    name = "mock.echo"
    supported_job_types = ("generic.browser.script", "mock.echo")
    required_capabilities = ("adapter.mock.echo",)

    async def run(self, context: "JobContext") -> AdapterResult:
        if context.job.input.get("force_error"):
            return AdapterResult.failed("MOCK_FORCED_ERROR", "mock adapter forced failure", retryable=False)

        context.artifact_collector.add_metadata(
            "mock.summary",
            {
                "job_id": context.job.job_id,
                "job_type": context.job.job_type,
                "input": context.job.input,
            },
        )
        return AdapterResult.completed(
            summary={
                "adapter": self.name,
                "echo": context.job.input,
                "artifacts_uploaded": 1,
            },
            cursor={"mock": "done"},
        )
