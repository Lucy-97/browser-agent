"""Platform uploader abstract base class.

Defines the five-step upload protocol that all platform-specific
uploaders must implement:

    navigate → upload_file → fill_metadata → set_visibility → submit

Each step is isolated so the caller (SocialUploadAdapter) can capture
failures at any stage and fall back to ``needs_manual_action``.
"""

from __future__ import annotations

import logging
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class UploadMetadata:
    """Video metadata passed to the uploader."""

    title: str = ""
    description: str = ""
    tags: tuple[str, ...] = ()
    cover_path: str | None = None
    visibility: str = "private"  # "public" | "private" | "unlisted" | "draft"
    extra: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class UploadStepResult:
    """Result of a single upload step."""

    success: bool
    step: str
    message: str = ""
    screenshot_path: Path | None = None


class PlatformUploader(ABC):
    """Abstract base for platform-specific video uploaders.

    Subclasses must set ``platform`` and ``upload_url`` class
    attributes and implement all five abstract methods.
    """

    platform: str  # e.g. "douyin", "tiktok", "youtube"
    upload_url: str  # creator studio upload page URL

    async def navigate(self, page: Any) -> UploadStepResult:
        """Navigate to the upload page and verify we are logged in."""
        try:
            await self._do_navigate(page)
            return UploadStepResult(success=True, step="navigate")
        except Exception as exc:
            logger.warning("navigate failed for %s: %s", self.platform, exc)
            return UploadStepResult(success=False, step="navigate", message=str(exc))

    async def upload_file(self, page: Any, video_path: Path) -> UploadStepResult:
        """Upload the video file via file input."""
        try:
            await self._do_upload_file(page, video_path)
            return UploadStepResult(success=True, step="upload_file")
        except Exception as exc:
            logger.warning("upload_file failed for %s: %s", self.platform, exc)
            return UploadStepResult(success=False, step="upload_file", message=str(exc))

    async def fill_metadata(self, page: Any, metadata: UploadMetadata) -> UploadStepResult:
        """Fill in title, description, tags, and cover image."""
        try:
            await self._do_fill_metadata(page, metadata)
            return UploadStepResult(success=True, step="fill_metadata")
        except Exception as exc:
            logger.warning("fill_metadata failed for %s: %s", self.platform, exc)
            return UploadStepResult(success=False, step="fill_metadata", message=str(exc))

    async def set_visibility(self, page: Any, visibility: str) -> UploadStepResult:
        """Set the video visibility (public, private, unlisted, draft)."""
        try:
            await self._do_set_visibility(page, visibility)
            return UploadStepResult(success=True, step="set_visibility")
        except Exception as exc:
            logger.warning("set_visibility failed for %s: %s", self.platform, exc)
            return UploadStepResult(success=False, step="set_visibility", message=str(exc))

    async def submit(self, page: Any, publish: bool = False) -> UploadStepResult:
        """Submit the upload (save as draft or publish).

        Parameters
        ----------
        publish:
            If ``True``, click the publish button.  If ``False``
            (default), save as draft / keep unpublished.
        """
        try:
            await self._do_submit(page, publish=publish)
            return UploadStepResult(success=True, step="submit")
        except Exception as exc:
            logger.warning("submit failed for %s: %s", self.platform, exc)
            return UploadStepResult(success=False, step="submit", message=str(exc))

    # -- Abstract methods to implement per platform --

    @abstractmethod
    async def _do_navigate(self, page: Any) -> None:
        raise NotImplementedError

    @abstractmethod
    async def _do_upload_file(self, page: Any, video_path: Path) -> None:
        raise NotImplementedError

    @abstractmethod
    async def _do_fill_metadata(self, page: Any, metadata: UploadMetadata) -> None:
        raise NotImplementedError

    @abstractmethod
    async def _do_set_visibility(self, page: Any, visibility: str) -> None:
        raise NotImplementedError

    @abstractmethod
    async def _do_submit(self, page: Any, publish: bool = False) -> None:
        raise NotImplementedError

    async def check_login(self, page: Any) -> bool:
        """Check if the user is logged in.  Default returns True."""
        return True

    async def is_upload_ready(self, page: Any) -> bool:
        """Return True when the platform upload form is ready for automation."""
        return await self.check_login(page)
