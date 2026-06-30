from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from .redaction import redact_value


class AgentTrace:
    def __init__(self, work_dir: Path):
        self.work_dir = work_dir
        self.work_dir.mkdir(parents=True, exist_ok=True)
        self.steps: list[dict[str, Any]] = []

    def add(self, step: str, payload: dict[str, Any]) -> None:
        self.steps.append({"step": step, **redact_value(payload)})

    def write(self) -> Path:
        path = self.work_dir / "agent-trace.json"
        path.write_text(json.dumps({"steps": self.steps}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return path

    async def screenshot(self, page: Any, name: str) -> Path | None:
        path = self.work_dir / f"{name}.png"
        try:
            await page.screenshot(path=str(path), full_page=True)
            return path
        except Exception:
            return None
