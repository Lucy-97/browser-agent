"""Social platform video upload adapter.

Orchestrates the five-step upload protocol using platform-specific
uploaders.  Login/manual intervention is handled while the browser is
still open so the user can authenticate and the same run can continue.
"""

from __future__ import annotations

import asyncio
import logging
from pathlib import Path
from typing import TYPE_CHECKING, Any

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.adapters.social.base import PlatformUploader, UploadMetadata, UploadStepResult
from qiyuan_worker.browser import BrowserRuntime, BrowserRuntimeConfig
from qiyuan_worker.browser.runtime import BrowserRuntimeError
from qiyuan_worker.protocols import AdapterResult, ManualAction

if TYPE_CHECKING:
    from qiyuan_worker.runtime.context import JobContext

logger = logging.getLogger(__name__)


def _get_uploader(platform: str) -> PlatformUploader:
    """Return the uploader instance for the given platform name."""
    if platform == "douyin":
        from qiyuan_worker.adapters.social.douyin import DouyinUploader
        return DouyinUploader()
    elif platform == "tiktok":
        from qiyuan_worker.adapters.social.tiktok import TikTokUploader
        return TikTokUploader()
    elif platform == "youtube":
        from qiyuan_worker.adapters.social.youtube import YouTubeUploader
        return YouTubeUploader()
    elif platform == "instagram":
        from qiyuan_worker.adapters.social.instagram import InstagramUploader
        return InstagramUploader()
    else:
        raise ValueError(f"Unsupported platform: {platform}")


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
        # ---- Parse input ----
        video_path = str(context.job.input.get("video_path") or "").strip()
        artifact_id = str(context.job.input.get("artifact_id") or "").strip()
        title = str(context.job.input.get("title") or "").strip()
        description = str(context.job.input.get("description") or "").strip()
        raw_tags = context.job.input.get("tags") or []
        if isinstance(raw_tags, str):
            raw_tags = [t.strip() for t in raw_tags.split(",") if t.strip()]
        tags = tuple(raw_tags)

        if not video_path and not artifact_id:
            return AdapterResult.failed("SOCIAL_VIDEO_REQUIRED", "input.video_path or input.artifact_id is required")
        if video_path and not Path(video_path).exists():
            return AdapterResult.failed("SOCIAL_VIDEO_NOT_FOUND", f"video_path does not exist: {video_path}")

        # If artifact_id is provided but no local path, download from API.
        resolved_video = Path(video_path) if video_path else None
        if not resolved_video and artifact_id:
            try:
                download_dir = context.work_dir / "downloads"
                download_dir.mkdir(parents=True, exist_ok=True)
                result = context.api_client.download_artifact(artifact_id, download_dir)
                resolved_video = Path(result["local_path"])
            except Exception as exc:
                return AdapterResult.failed("SOCIAL_ARTIFACT_DOWNLOAD_FAILED", str(exc), retryable=True)

        if not resolved_video or not resolved_video.exists():
            return AdapterResult.failed("SOCIAL_VIDEO_NOT_FOUND", "resolved video path does not exist")

        # ---- Resolve uploader ----
        try:
            uploader = _get_uploader(self.platform)
        except ValueError as exc:
            return AdapterResult.failed("SOCIAL_PLATFORM_UNSUPPORTED", str(exc))

        metadata = UploadMetadata(
            title=title,
            description=description,
            tags=tags,
            visibility="private",
        )

        manual_publish_required = context.job.policy.get("manual_publish_required", False)
        publish = not manual_publish_required

        # ---- Launch browser ----
        runtime = BrowserRuntime(
            BrowserRuntimeConfig(
                profile_dir=context.config.secrets_dir / "browser-profiles" / f"social-{self.platform}",
                downloads_dir=context.config.data_dir / "downloads" / context.job.job_id,
                headed=bool(context.job.policy.get("headed", True)),
            )
        )

        try:
            async with await runtime.open_page() as page:
                # Execute five-step upload protocol.
                steps = [
                    ("navigate", lambda: uploader.navigate(page)),
                    ("upload_file", lambda: uploader.upload_file(page, resolved_video)),
                    ("fill_metadata", lambda: uploader.fill_metadata(page, metadata)),
                    ("set_visibility", lambda: uploader.set_visibility(page, metadata.visibility)),
                    ("submit", lambda: uploader.submit(page, publish=publish)),
                ]

                completed_steps: list[str] = []
                for step_name, step_fn in steps:
                    if step_name == "submit" and manual_publish_required and self.platform == "instagram":
                        step_result = await _wait_for_manual_publish(
                            page=page,
                            context=context,
                            platform=self.platform,
                            title=title,
                            completed_steps=completed_steps,
                        )
                        if not step_result.success:
                            screenshot_path = await _take_screenshot(page, context.work_dir, step_name)
                            if screenshot_path:
                                context.artifact_collector.add_file(
                                    "screenshot",
                                    screenshot_path,
                                    metadata={"platform": self.platform, "step": step_name, "error": step_result.message},
                                )
                            return AdapterResult(
                                status="needs_manual_action",
                                summary={
                                    "platform": self.platform,
                                    "title": title,
                                    "published": False,
                                    "requires_manual_action": True,
                                    "completed_steps": completed_steps,
                                    "failed_step": step_name,
                                    "error": step_result.message,
                                    "publish_allowed": publish,
                                },
                                manual_action=ManualAction(
                                    action_type=f"{self.platform}_manual_publish_required",
                                    message=(
                                        f"{self.platform} 已进入发布确认阶段。"
                                        "请在本机浏览器中完成发布；如果窗口已关闭，请重新下发任务。"
                                    ),
                                    payload={
                                        "platform": self.platform,
                                        "video_path": str(resolved_video),
                                        "title": title,
                                        "failed_step": step_name,
                                        "completed_steps": completed_steps,
                                        "publish_allowed": publish,
                                    },
                                ),
                            )
                        completed_steps.append(step_name)
                        continue

                    step_result: UploadStepResult = await step_fn()
                    if not step_result.success and step_name == "navigate":
                        step_result = await _wait_for_login_and_retry_navigate(
                            page=page,
                            uploader=uploader,
                            context=context,
                            platform=self.platform,
                            previous_message=step_result.message,
                        )
                    elif not step_result.success and not await uploader.check_login(page):
                        login_result = await _wait_for_login_and_retry_navigate(
                            page=page,
                            uploader=uploader,
                            context=context,
                            platform=self.platform,
                            previous_message=step_result.message,
                        )
                        if login_result.success:
                            step_result = await step_fn()
                        else:
                            step_result = UploadStepResult(
                                success=False,
                                step=step_name,
                                message=login_result.message,
                            )
                    if not step_result.success:
                        # Take a screenshot for diagnostics.
                        screenshot_path = await _take_screenshot(page, context.work_dir, step_name)
                        if screenshot_path:
                            context.artifact_collector.add_file(
                                "screenshot",
                                screenshot_path,
                                metadata={"platform": self.platform, "step": step_name, "error": step_result.message},
                            )
                        return AdapterResult(
                            status="needs_manual_action",
                            summary={
                                "platform": self.platform,
                                "title": title,
                                "published": False,
                                "requires_manual_action": True,
                                "completed_steps": completed_steps,
                                "failed_step": step_name,
                                "error": step_result.message,
                                "publish_allowed": publish,
                            },
                            manual_action=ManualAction(
                                action_type=f"{self.platform}_upload_step_failed",
                                message=(
                                    f"{self.platform} 自动上传在 '{step_name}' 步骤失败: {step_result.message}。"
                                    "请在本机浏览器中完成处理后重新下发任务。"
                                ),
                                payload={
                                    "platform": self.platform,
                                    "video_path": str(resolved_video),
                                    "title": title,
                                    "failed_step": step_name,
                                    "completed_steps": completed_steps,
                                    "publish_allowed": publish,
                                },
                            ),
                        )
                    completed_steps.append(step_name)

                # All steps succeeded — take a final screenshot.
                final_screenshot = await _take_screenshot(page, context.work_dir, "final")
                if final_screenshot:
                    context.artifact_collector.add_file(
                        "screenshot",
                        final_screenshot,
                        metadata={"platform": self.platform, "step": "final", "status": "completed"},
                    )

                return AdapterResult.completed(
                    summary={
                        "platform": self.platform,
                        "title": title,
                        "published": publish,
                        "completed_steps": completed_steps,
                        "url": page.url,
                    },
                )

        except BrowserRuntimeError as exc:
            return AdapterResult.failed(exc.code, exc.message, retryable=False)
        except Exception as exc:
            return AdapterResult.failed("SOCIAL_UPLOAD_ERROR", str(exc), retryable=True)


async def _wait_for_login_and_retry_navigate(
    page: Any,
    uploader: PlatformUploader,
    context: "JobContext",
    platform: str,
    previous_message: str,
) -> UploadStepResult:
    """Keep the browser open while the user logs in, then retry navigation."""
    timeout_seconds = _policy_float(context.job.policy, "manual_login_timeout_seconds", 300.0)
    poll_seconds = _policy_float(context.job.policy, "manual_login_poll_seconds", 2.0)
    timeout_seconds = max(0.0, min(timeout_seconds, 1800.0))
    poll_seconds = max(0.0, min(poll_seconds, 30.0))
    if timeout_seconds <= 0:
        return UploadStepResult(success=False, step="navigate", message=previous_message)

    try:
        context.api_client.run_heartbeat(
            context.job.run_id,
            {
                "status": "needs_manual_action",
                "current_step": "manual_login",
                "cursor": context.job.cursor,
                "message": f"{platform} 需要人工登录: {previous_message}",
            },
        )
    except Exception:
        logger.debug("failed to send manual_login heartbeat", exc_info=True)

    deadline = asyncio.get_running_loop().time() + timeout_seconds
    next_navigation_retry_at = 0.0
    navigation_retry_seconds = max(10.0, min(timeout_seconds, 30.0))
    while asyncio.get_running_loop().time() < deadline:
        await asyncio.sleep(poll_seconds)
        try:
            if await uploader.is_upload_ready(page):
                return UploadStepResult(success=True, step="navigate")
            if await uploader.check_login(page):
                now = asyncio.get_running_loop().time()
                if now < next_navigation_retry_at:
                    continue
                next_navigation_retry_at = now + navigation_retry_seconds
                try:
                    retry = await uploader.navigate(page)
                except Exception as exc:
                    logger.debug("manual login navigation retry raised for %s: %s", platform, exc)
                    continue
                if retry.success:
                    return retry
        except Exception as exc:
            logger.debug("login wait check failed for %s: %s", platform, exc)

    return UploadStepResult(
        success=False,
        step="navigate",
        message=f"{previous_message}; 等待人工登录超时（{int(timeout_seconds)} 秒）",
    )


async def _wait_for_manual_publish(
    page: Any,
    context: "JobContext",
    platform: str,
    title: str,
    completed_steps: list[str],
) -> UploadStepResult:
    timeout_seconds = _policy_float(context.job.policy, "manual_publish_timeout_seconds", 300.0)
    poll_seconds = _policy_float(context.job.policy, "manual_publish_poll_seconds", 5.0)
    timeout_seconds = max(0.0, min(timeout_seconds, 1800.0))
    poll_seconds = max(0.0, min(poll_seconds, 30.0))

    try:
        context.api_client.run_heartbeat(
            context.job.run_id,
            {
                "status": "needs_manual_action",
                "current_step": "manual_publish",
                "cursor": context.job.cursor,
                "message": f"{platform} 已完成自动上传步骤，请在本机浏览器中确认发布: {title}",
                "completed_steps": completed_steps,
            },
        )
    except Exception:
        logger.debug("failed to send manual_publish heartbeat", exc_info=True)

    if timeout_seconds <= 0:
        return UploadStepResult(
            success=False,
            step="submit",
            message="Instagram 已进入发布确认阶段，请在本机浏览器中人工发布",
        )

    deadline = asyncio.get_running_loop().time() + timeout_seconds
    while asyncio.get_running_loop().time() < deadline:
        await asyncio.sleep(poll_seconds)

    return UploadStepResult(
        success=False,
        step="submit",
        message=f"Instagram 已等待人工发布 {int(timeout_seconds)} 秒；请确认是否已在浏览器中完成发布",
    )


def _policy_float(policy: dict[str, Any], key: str, default: float) -> float:
    try:
        return float(policy.get(key, default))
    except (TypeError, ValueError):
        return default


async def _take_screenshot(page: Any, work_dir: Path, label: str) -> Path | None:
    """Take a screenshot and save it to work_dir.  Returns the path or None."""
    try:
        screenshots_dir = work_dir / "screenshots"
        screenshots_dir.mkdir(parents=True, exist_ok=True)
        path = screenshots_dir / f"{label}.png"
        await page.screenshot(path=str(path), full_page=False)
        return path
    except Exception as exc:
        logger.warning("failed to take screenshot (%s): %s", label, exc)
        return None
