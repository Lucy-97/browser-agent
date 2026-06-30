from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass
from importlib import metadata
from typing import Any, Protocol

from qiyuan_worker.adapters.base import AutomationAdapter


SDK_API_VERSION = "1"
ENTRY_POINT_GROUP = "qiyuan_worker.extensions"


class ExtensionRegistry(Protocol):
    def register(self, adapter: AutomationAdapter) -> None:
        raise NotImplementedError


@dataclass(frozen=True)
class PolicyTemplate:
    name: str
    product_line: str
    job_type: str
    adapter: str
    policy: dict[str, Any]
    target: dict[str, Any]

    def to_dict(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "product_line": self.product_line,
            "job_type": self.job_type,
            "adapter": self.adapter,
            "policy": self.policy,
            "target": self.target,
        }


@dataclass(frozen=True)
class WorkerExtension:
    name: str
    product_line: str
    adapters: tuple[AutomationAdapter, ...] = ()
    capabilities: tuple[str, ...] = ()
    policy_templates: tuple[PolicyTemplate, ...] = ()
    requires_playwright: bool = False
    sdk_api_version: str = SDK_API_VERSION

    def register(self, registry: ExtensionRegistry) -> None:
        for adapter in self.adapters:
            registry.register(adapter)

    def detect_capabilities(self, runtime_capabilities: set[str]) -> set[str]:
        if self.requires_playwright and "browser.playwright.chromium" not in runtime_capabilities:
            return set()
        return set(self.capabilities)


def product_enabled(product_line: str, enabled_products: Iterable[str] | None) -> bool:
    if enabled_products is None:
        return True
    values = {item.strip() for item in enabled_products if item.strip()}
    return "*" in values or product_line in values


def filter_extensions(
    extensions: Iterable[WorkerExtension],
    enabled_products: Iterable[str] | None,
) -> tuple[WorkerExtension, ...]:
    return tuple(extension for extension in extensions if product_enabled(extension.product_line, enabled_products))


def detect_extension_capabilities(
    extensions: Iterable[WorkerExtension],
    runtime_capabilities: set[str],
) -> set[str]:
    capabilities = set(runtime_capabilities)
    for extension in extensions:
        capabilities.update(extension.detect_capabilities(runtime_capabilities))
    return capabilities


def load_entry_point_extensions() -> tuple[WorkerExtension, ...]:
    discovered: list[WorkerExtension] = []
    for entry_point in metadata.entry_points(group=ENTRY_POINT_GROUP):
        loaded = entry_point.load()
        discovered.extend(_coerce_extensions(loaded() if callable(loaded) else loaded))
    return tuple(discovered)


def _coerce_extensions(value: Any) -> tuple[WorkerExtension, ...]:
    if isinstance(value, WorkerExtension):
        return (value,)
    if isinstance(value, Iterable) and not isinstance(value, (str, bytes, dict)):
        extensions = tuple(value)
        if all(isinstance(item, WorkerExtension) for item in extensions):
            return extensions
    raise TypeError(f"entry point must return WorkerExtension or iterable of WorkerExtension, got {type(value)!r}")
