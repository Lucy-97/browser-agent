from __future__ import annotations

import hashlib
import json
import mimetypes
import os
import shutil
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import TYPE_CHECKING, Any

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.protocols import AdapterResult

if TYPE_CHECKING:
    from qiyuan_worker.runtime.context import JobContext


DEFAULT_MAX_FILES = 200
DEFAULT_MAX_FILE_BYTES = 100 * 1024 * 1024
MATERIAL_SUFFIXES = {
    ".csv",
    ".doc",
    ".docx",
    ".gif",
    ".heic",
    ".jpeg",
    ".jpg",
    ".md",
    ".mov",
    ".mp3",
    ".mp4",
    ".numbers",
    ".pages",
    ".pdf",
    ".png",
    ".ppt",
    ".pptx",
    ".rtf",
    ".txt",
    ".wav",
    ".xls",
    ".xlsx",
    ".zip",
}


@dataclass(frozen=True)
class SyncedWeixinFile:
    source_path: str
    stored_path: str
    file_name: str
    mime_type: str
    size_bytes: int
    sha256: str
    modified_at: float


class WeixinDesktopSyncAdapter(AutomationAdapter):
    name = "weixin.desktop_sync"
    supported_job_types = ("weixin.desktop_sync",)
    required_capabilities = ("adapter.weixin.desktop_sync",)

    async def run(self, context: "JobContext") -> AdapterResult:
        source_dirs = _string_list(context.job.input.get("source_dirs"))
        group_names = _group_names(context.job.input)
        group_keywords = group_names or _string_list(context.job.input.get("group_keywords"))
        max_files = _positive_int(context.job.policy.get("max_files"), DEFAULT_MAX_FILES)
        max_file_bytes = _positive_int(context.job.policy.get("max_file_bytes"), DEFAULT_MAX_FILE_BYTES)
        since_mtime = _cursor_float(context.job.cursor, "last_mtime")

        if not source_dirs:
            return AdapterResult.failed(
                "WEIXIN_SOURCE_DIRS_REQUIRED",
                "input.source_dirs is required for desktop Weixin sync",
                retryable=False,
            )

        existing_dirs = [_expand_source_dir(item) for item in source_dirs]
        existing_dirs = [item for item in existing_dirs if item.exists() and item.is_dir()]
        if not existing_dirs:
            return AdapterResult.failed(
                "WEIXIN_SOURCE_DIRS_NOT_FOUND",
                "none of input.source_dirs exists on this worker",
                retryable=False,
            )

        candidates = _collect_candidates(
            existing_dirs,
            group_keywords=group_keywords,
            since_mtime=since_mtime,
            max_file_bytes=max_file_bytes,
        )
        candidates = candidates[:max_files]

        archive_dir = context.work_dir / "weixin-desktop-sync"
        archive_dir.mkdir(parents=True, exist_ok=True)

        synced: list[SyncedWeixinFile] = []
        seen_sha256: set[str] = set()
        skipped_duplicate_count = 0
        for source in candidates:
            sha256 = _sha256_file(source)
            if sha256 in seen_sha256:
                skipped_duplicate_count += 1
                continue
            seen_sha256.add(sha256)

            stored_path = _copy_unique(source, archive_dir)
            stat = stored_path.stat()
            synced.append(
                SyncedWeixinFile(
                    source_path=str(source),
                    stored_path=str(stored_path),
                    file_name=source.name,
                    mime_type=mimetypes.guess_type(source.name)[0] or "application/octet-stream",
                    size_bytes=stat.st_size,
                    sha256=sha256,
                    modified_at=source.stat().st_mtime,
                )
            )
            context.artifact_collector.add_file(
                "weixin_file",
                stored_path,
                metadata={
                    "source_path": str(source),
                    "file_name": source.name,
                    "mime_type": mimetypes.guess_type(source.name)[0] or "application/octet-stream",
                    "sha256": sha256,
                    "source": "weixin_desktop",
                },
            )

        manifest_path = context.work_dir / "weixin-desktop-sync-manifest.json"
        manifest_payload = {
            "source_dirs": [str(item) for item in existing_dirs],
            "group_names": group_names,
            "group_keywords": group_keywords,
            "since_mtime": since_mtime,
            "synced_count": len(synced),
            "skipped_duplicate_count": skipped_duplicate_count,
            "files": [asdict(item) for item in synced],
        }
        manifest_path.write_text(json.dumps(manifest_payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        context.artifact_collector.add_file("weixin_manifest", manifest_path, metadata={"source": "weixin_desktop"})

        last_mtime = max((item.modified_at for item in synced), default=since_mtime)
        return AdapterResult.completed(
            summary={
                "source": "weixin_desktop",
                "synced_count": len(synced),
                "skipped_duplicate_count": skipped_duplicate_count,
                "source_dirs": [str(item) for item in existing_dirs],
                "group_names": group_names,
                "group_keywords": group_keywords,
            },
            cursor={"last_mtime": last_mtime},
        )


def _collect_candidates(
    source_dirs: list[Path],
    *,
    group_keywords: list[str],
    since_mtime: float | None,
    max_file_bytes: int,
) -> list[Path]:
    files: list[Path] = []
    for source_dir in source_dirs:
        for path in source_dir.rglob("*"):
            if not path.is_file():
                continue
            try:
                stat = path.stat()
            except OSError:
                continue
            if since_mtime is not None and stat.st_mtime <= since_mtime:
                continue
            if stat.st_size <= 0 or stat.st_size > max_file_bytes:
                continue
            if not _looks_like_material_file(path):
                continue
            haystack = str(path).lower()
            if group_keywords and not any(keyword.lower() in haystack for keyword in group_keywords):
                continue
            files.append(path)
    files.sort(key=lambda item: item.stat().st_mtime)
    return files


def _expand_source_dir(value: str) -> Path:
    return Path(os.path.expandvars(value)).expanduser()


def _looks_like_material_file(path: Path) -> bool:
    return path.suffix.lower() in MATERIAL_SUFFIXES


def _copy_unique(source: Path, target_dir: Path) -> Path:
    target = target_dir / source.name
    if not target.exists():
        shutil.copy2(source, target)
        return target
    stem = source.stem
    suffix = source.suffix
    for index in range(1, 10_000):
        candidate = target_dir / f"{stem}-{index}{suffix}"
        if not candidate.exists():
            shutil.copy2(source, candidate)
            return candidate
    raise RuntimeError(f"too many duplicate file names for {source.name}")


def _sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _string_list(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        return [item.strip() for item in value.splitlines() if item.strip()]
    if isinstance(value, list):
        return [str(item).strip() for item in value if str(item).strip()]
    return []


def _group_names(input_payload: dict[str, Any]) -> list[str]:
    values = _string_list(input_payload.get("group_names"))
    selected_groups = input_payload.get("selected_groups")
    if isinstance(selected_groups, list):
        for item in selected_groups:
            if not isinstance(item, dict):
                continue
            display_name = str(item.get("display_name", "")).strip()
            if display_name:
                values.append(display_name)
    return _unique_strings(values)


def _unique_strings(values: list[str]) -> list[str]:
    result: list[str] = []
    seen: set[str] = set()
    for value in values:
        if value in seen:
            continue
        seen.add(value)
        result.append(value)
    return result


def _positive_int(value: Any, default: int) -> int:
    try:
        parsed = int(value)
    except (TypeError, ValueError):
        return default
    return parsed if parsed > 0 else default


def _cursor_float(cursor: dict[str, Any] | None, key: str) -> float | None:
    if not cursor:
        return None
    try:
        return float(cursor.get(key))
    except (TypeError, ValueError):
        return None
