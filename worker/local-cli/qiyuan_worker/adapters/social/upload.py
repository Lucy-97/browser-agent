from __future__ import annotations

from pathlib import Path
from typing import TYPE_CHECKING

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.protocols import AdapterResult, ManualAction

if TYPE_CHECKING:
    from qiyuan_worker.runtime.context import JobContext


class SocialUploadAdapter(AutomationAdapter):
    def __init__(self, platform: str):
        self.platform = platform
        self.name = f"social.{platform}.upload_video"
        self.supported_job_types = (f"social.{platform}.upload_video",)
        self.required_capabilities = (
            "browser.playwright.chromium",
            "browser.profile.persistent",
            f"adapter.social.{platform}.upload_video",
        )

    async def run(self, context: "JobContext") -> AdapterResult:
        video_path = str(context.job.input.get("video_path") or "").strip()
        artifact_id = str(context.job.input.get("artifact_id") or "").strip()
        title = str(context.job.input.get("title") or "").strip()
        if not video_path and not artifact_id:
            return AdapterResult.failed("SOCIAL_VIDEO_REQUIRED", "input.video_path or input.artifact_id is required")
        if video_path and not Path(video_path).exists():
            return AdapterResult.failed("SOCIAL_VIDEO_NOT_FOUND", f"video_path does not exist: {video_path}")

        return AdapterResult(
            status="needs_manual_action",
            summary={
                "platform": self.platform,
                "title": title,
                "reason": "upload_requires_manual_confirmation",
                "publish_blocked": True,
            },
            manual_action=ManualAction(
                action_type=f"{self.platform}_draft_upload_required",
                message=(
                    f"{self.platform} upload requires user confirmation. "
                    "Complete upload in the local browser and keep the video as draft/private."
                ),
                payload={
                    "platform": self.platform,
                    "video_path": video_path or None,
                    "artifact_id": artifact_id or None,
                    "title": title or None,
                    "publish_allowed": False,
                },
            ),
        )
