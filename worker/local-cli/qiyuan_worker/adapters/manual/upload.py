from __future__ import annotations

from typing import TYPE_CHECKING

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.protocols import AdapterResult

if TYPE_CHECKING:
    from qiyuan_worker.runtime.context import JobContext


class ManualUploadAdapter(AutomationAdapter):
    """Adapter for synthetic jobs created by the Go API for manual PDF uploads.

    The actual file saving and result creation are performed inline by the
    Go API handler. This adapter merely acknowledges the job so the worker
    does not fail with ADAPTER_UNSUPPORTED.
    """

    name = "manual"
    supported_job_types = ("qiyuan.manual_upload",)

    async def run(self, context: "JobContext") -> AdapterResult:
        return AdapterResult.completed(
            summary={"adapter": self.name, "source": context.job.input.get("source", "unknown")},
            cursor={"done": True},
        )
