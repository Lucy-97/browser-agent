"""Douyin (抖音) creator studio video uploader.

Uploads videos to ``creator.douyin.com/creator-micro/content/upload``
via Playwright persistent context.

Selector strategy: selectors are grouped in ``_SELECTORS`` for easy
maintenance when Douyin changes their DOM structure.
"""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

from qiyuan_worker.adapters.social.base import PlatformUploader, UploadMetadata

logger = logging.getLogger(__name__)

# Selectors are centralised here for easy maintenance.
_SELECTORS = {
    # File input on the upload page.
    "file_input": 'input[type="file"]',
    # The title / caption input area (contenteditable div).
    "title_input": 'div[data-placeholder][contenteditable="true"]',
    # Fallback title selectors (Douyin frequently changes these).
    "title_input_alt": (
        '.editor-kit-container div[contenteditable="true"]',
        '.ql-editor[contenteditable="true"]',
        'div.notranslate[contenteditable="true"]',
    ),
    # Upload progress indicator — disappears when upload completes.
    "upload_progress": '.progress-bar, [class*="upload-progress"], [class*="uploading"]',
    # Publish / Save draft button.
    "publish_button": 'button:has-text("发布")',
    "draft_button": 'button:has-text("存草稿")',
    # Login detection markers.
    "login_marker": 'text="扫码登录"',
    "login_marker_alt": 'text="手机号登录"',
}


class DouyinUploader(PlatformUploader):
    platform = "douyin"
    upload_url = "https://creator.douyin.com/creator-micro/content/upload"

    async def _do_navigate(self, page: Any) -> None:
        await page.goto(self.upload_url, wait_until="domcontentloaded", timeout=60_000)
        await page.wait_for_timeout(3000)  # let SPA finish rendering

        # Verify we are not on the login page.
        if not await self.check_login(page):
            raise RuntimeError(
                "未登录抖音创作者后台，请先通过 qiyuan-worker mcp --profile social-douyin 手动扫码登录"
            )

    async def check_login(self, page: Any) -> bool:
        login_text = page.get_by_text("扫码登录", exact=True).first
        phone_text = page.get_by_text("手机号登录", exact=True).first
        has_login = (await login_text.count() > 0) or (await phone_text.count() > 0)
        is_upload_page = "content/upload" in page.url
        return is_upload_page and not has_login

    async def _do_upload_file(self, page: Any, video_path: Path) -> None:
        file_input = page.locator(_SELECTORS["file_input"]).first
        await file_input.wait_for(state="attached", timeout=15_000)
        await file_input.set_input_files(str(video_path))
        logger.info("douyin: video file set via input: %s", video_path.name)

        # Wait for the upload to start processing (page should update).
        await page.wait_for_timeout(3000)

    async def _do_fill_metadata(self, page: Any, metadata: UploadMetadata) -> None:
        # Try to locate the title/caption editor.
        title_editor = None
        for selector in (_SELECTORS["title_input"], *_SELECTORS["title_input_alt"]):
            locator = page.locator(selector).first
            if await locator.count() > 0:
                title_editor = locator
                break

        if title_editor is None:
            logger.warning("douyin: title editor not found, skipping metadata fill")
            return

        # Clear existing content and type the title.
        await title_editor.click()
        await page.keyboard.press("Control+A")
        await page.keyboard.press("Backspace")

        # Build caption: title + tags.
        caption = metadata.title
        if metadata.tags:
            tag_str = " ".join(f"#{tag}" for tag in metadata.tags)
            caption = f"{caption} {tag_str}"
        await title_editor.fill(caption)
        logger.info("douyin: filled caption: %s", caption[:80])

        # Small delay for the UI to process.
        await page.wait_for_timeout(1000)

    async def _do_set_visibility(self, page: Any, visibility: str) -> None:
        # Douyin does not have a simple public/private toggle on the upload page.
        # The default upload state is effectively a draft until "发布" is clicked.
        # For scheduled publishing, we would need to interact with the calendar.
        # For now, this is a no-op as we rely on submit(publish=False) for draft mode.
        logger.info("douyin: visibility=%s (controlled via submit step)", visibility)

    async def _do_submit(self, page: Any, publish: bool = False) -> None:
        if publish:
            publish_btn = page.locator(_SELECTORS["publish_button"]).first
            await publish_btn.wait_for(state="visible", timeout=30_000)
            await publish_btn.click()
            logger.info("douyin: clicked publish button")
        else:
            # Try to save as draft.
            draft_btn = page.locator(_SELECTORS["draft_button"]).first
            if await draft_btn.count() > 0:
                await draft_btn.click()
                logger.info("douyin: saved as draft")
            else:
                # Some versions may not have an explicit draft button.
                # In that case, closing without publishing leaves it unsaved.
                logger.warning("douyin: no draft button found, leaving page (video may not be saved)")

        # Wait for the action to complete.
        await page.wait_for_timeout(3000)
