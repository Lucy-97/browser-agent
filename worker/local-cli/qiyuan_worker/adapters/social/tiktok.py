"""TikTok creator center video uploader.

Uploads videos to ``www.tiktok.com/creator-center/upload`` via
Playwright persistent context.
"""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

from qiyuan_worker.adapters.social.base import PlatformUploader, UploadMetadata

logger = logging.getLogger(__name__)

_SELECTORS = {
    "file_input": 'input[type="file"]',
    # TikTok uses a contenteditable div for the caption.
    "caption_editor": (
        'div[data-text="true"][contenteditable="true"]',
        'div[aria-label*="caption" i][contenteditable="true"]',
        '.notranslate[contenteditable="true"]',
        'div[contenteditable="true"]',
    ),
    # Post / Schedule / Save draft buttons.
    "post_button": 'button:has-text("Post"), button[data-e2e="post_video_button"]',
    "draft_button": 'button:has-text("Save draft"), button:has-text("Drafts")',
    # Login detection.
    "login_marker": (
        'button:has-text("Log in")',
        'a:has-text("Log in")',
        'text="登录 TikTok"',
        'text="使用二维码登录"',
        'text="使用手机号/电子邮箱/用户名登录"',
        'text="使用手机号码/电子邮箱/用户名登录"',
        'text="继续即表示你同意 TikTok 的"',
    ),
}


class TikTokUploader(PlatformUploader):
    platform = "tiktok"
    upload_url = "https://www.tiktok.com/creator-center/upload"

    async def _do_navigate(self, page: Any) -> None:
        await page.goto(self.upload_url, wait_until="domcontentloaded", timeout=60_000)
        await page.wait_for_timeout(3000)

        if not await self.is_upload_ready(page):
            raise RuntimeError(
                "未登录 TikTok，请先通过 qiyuan-worker mcp --profile social-tiktok 手动登录"
            )

    async def check_login(self, page: Any) -> bool:
        for selector in _SELECTORS["login_marker"]:
            marker = page.locator(selector).first
            if await marker.count() > 0:
                try:
                    if await marker.is_visible():
                        return False
                except Exception:
                    return False
        # Also check URL — TikTok may redirect to login page.
        if "/login" in page.url.lower():
            return False
        return True

    async def is_upload_ready(self, page: Any) -> bool:
        if not await self.check_login(page):
            return False
        file_input = page.locator(_SELECTORS["file_input"]).first
        return await file_input.count() > 0

    async def _do_upload_file(self, page: Any, video_path: Path) -> None:
        file_input = page.locator(_SELECTORS["file_input"]).first
        await file_input.wait_for(state="attached", timeout=15_000)
        await file_input.set_input_files(str(video_path))
        logger.info("tiktok: video file set via input: %s", video_path.name)
        # Wait for upload to begin processing.
        await page.wait_for_timeout(5000)

    async def _do_fill_metadata(self, page: Any, metadata: UploadMetadata) -> None:
        caption_editor = None
        for selector in _SELECTORS["caption_editor"]:
            locator = page.locator(selector).first
            if await locator.count() > 0:
                caption_editor = locator
                break

        if caption_editor is None:
            logger.warning("tiktok: caption editor not found, skipping metadata fill")
            return

        await caption_editor.click()
        await page.keyboard.press("Control+A")
        await page.keyboard.press("Backspace")

        caption = metadata.title
        if metadata.tags:
            tag_str = " ".join(f"#{tag}" for tag in metadata.tags)
            caption = f"{caption} {tag_str}"

        # TikTok's caption editor may not support fill() well;
        # fall back to typing character by character for reliability.
        try:
            await caption_editor.fill(caption)
        except Exception:
            await page.keyboard.type(caption, delay=30)

        logger.info("tiktok: filled caption: %s", caption[:80])
        await page.wait_for_timeout(1000)

    async def _do_set_visibility(self, page: Any, visibility: str) -> None:
        # TikTok's visibility is typically controlled by a dropdown.
        # Default upload state is "Everyone" (public).  We leave it as-is
        # and rely on the submit step to save as draft vs. post.
        logger.info("tiktok: visibility=%s (controlled via submit step)", visibility)

    async def _do_submit(self, page: Any, publish: bool = False) -> None:
        if publish:
            post_btn = page.locator(_SELECTORS["post_button"]).first
            await post_btn.wait_for(state="visible", timeout=30_000)
            await post_btn.click()
            logger.info("tiktok: clicked Post button")
        else:
            draft_btn = page.locator(_SELECTORS["draft_button"]).first
            if await draft_btn.count() > 0:
                await draft_btn.click()
                logger.info("tiktok: saved as draft")
            else:
                logger.warning("tiktok: no draft button found")

        await page.wait_for_timeout(3000)
