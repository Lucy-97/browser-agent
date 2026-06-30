from __future__ import annotations

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.builtin_extensions import builtin_worker_extensions
from qiyuan_worker.protocols import AutomationJob
from qiyuan_worker.sdk import WorkerExtension, filter_extensions, load_entry_point_extensions


class AdapterResolutionError(RuntimeError):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


class AdapterRegistry:
    def __init__(self) -> None:
        self._by_name: dict[str, AutomationAdapter] = {}

    def register(self, adapter: AutomationAdapter) -> None:
        if adapter.name in self._by_name:
            raise ValueError(f"adapter {adapter.name} is already registered")
        self._by_name[adapter.name] = adapter

    def adapters(self) -> tuple[AutomationAdapter, ...]:
        return tuple(self._by_name.values())

    def resolve(self, job: AutomationJob, capabilities: set[str]) -> AutomationAdapter:
        adapter = self._by_name.get(job.adapter)
        if not adapter:
            raise AdapterResolutionError(
                "ADAPTER_UNSUPPORTED",
                f"adapter {job.adapter} is not registered",
            )
        if job.job_type not in adapter.supported_job_types:
            raise AdapterResolutionError(
                "ADAPTER_JOB_TYPE_UNSUPPORTED",
                f"adapter {job.adapter} does not support job type {job.job_type}",
            )

        missing = sorted(set(adapter.required_capabilities) - capabilities)
        if missing:
            raise AdapterResolutionError(
                "CAPABILITY_MISMATCH",
                f"adapter {job.adapter} requires missing capabilities: {', '.join(missing)}",
            )
        return adapter


def load_worker_extensions(
    enabled_products: tuple[str, ...] | None = None,
    include_entry_points: bool = True,
) -> tuple[WorkerExtension, ...]:
    extensions = list(builtin_worker_extensions())
    if include_entry_points:
        extensions.extend(load_entry_point_extensions())
    return filter_extensions(extensions, enabled_products)


def build_default_registry(
    enabled_products: tuple[str, ...] | None = None,
    extensions: tuple[WorkerExtension, ...] | None = None,
) -> AdapterRegistry:
    registry = AdapterRegistry()
    for extension in extensions or load_worker_extensions(enabled_products=enabled_products):
        extension.register(registry)
    return registry
