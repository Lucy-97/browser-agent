from __future__ import annotations

from abc import ABC, abstractmethod
from typing import TYPE_CHECKING

from qiyuan_worker.protocols import AdapterResult

if TYPE_CHECKING:
    from qiyuan_worker.runtime.context import JobContext


class AutomationAdapter(ABC):
    name: str
    supported_job_types: tuple[str, ...]
    required_capabilities: tuple[str, ...] = ()

    async def prepare(self, context: "JobContext") -> None:
        return None

    @abstractmethod
    async def run(self, context: "JobContext") -> AdapterResult:
        raise NotImplementedError

    async def cleanup(self, context: "JobContext") -> None:
        return None
