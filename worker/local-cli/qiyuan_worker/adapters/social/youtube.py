"""YouTube Studio video uploader.

Uploads videos to YouTube Studio via Playwright persistent context.

YouTube Studio makes heavy use of **Shadow DOM** (Web Components).
Playwright's locator engine can pierce Shadow DOM automatically when
using CSS selectors, but some elements require explicit traversal.
"""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

from qiyuan_worker.adapters.social.base import PlatformUploader, UploadMetadata

logger = logging.getLogger(__name__)

_SELECTORS = {
    # The file input is inside a Shadow DOM but Playwright can still
    # locate it with a broad CSS selector.
    "file_input": 'input[type="file"]',
    # The "Create" / upload button on the Studio dashboard.
    "create_button": "#create-icon, ytcp-button#create-icon",
    "upload_option": 'tp-yt-paper-item:has-text("Upload videos"), #text-item-0',
    # Title and description textboxes (inside Shadow DOM).
    "title_input": '#title-textarea #textbox, div#textbox[slot="input"]',
    "description_input": '#description-textarea #textbox',
    # Visibility radio buttons in the "Visibility" step.
    "visibility_public": 'tp-yt-paper-radio-button[name="PUBLIC"]',
    "visibility_unlisted": 'tp-yt-paper-radio-button[name="UNLISTED"]',
    "visibility_private": 'tp-yt-paper-radio-button[name="PRIVATE"]',
    # Navigation buttons in the upload wizard.
    "next_button": '#next-button, ytcp-button#next-button',
    "done_button": '#done-button, ytcp-button#done-button',
    # Upload progress / processing indicator.
    "processing_text": '.progress-label, span.ytcp-video-upload-progress',
    # Login detection.
    "login_marker": 'a[href*="accounts.google.com/ServiceLogin"]',
}

# Map our visibility names to YouTube's radio button names.
_VISIBILITY_MAP = {
    "public": "PUBLIC",
    "unlisted": "UNLISTED",
    "private": "PRIVATE",
    "draft": "PRIVATE",  # "draft" maps to private on YouTube
}


class YouTubeUploader(PlatformUploader):
    platform = "youtube"
    upload_url = "https://studio.youtube.com"

    async def _do_navigate(self, page: Any) -> None:
        await page.goto(self.upload_url, wait_until="domcontentloaded", timeout=60_000)
        await page.wait_for_timeout(3000)

        if not await self.check_login(page):
            raise RuntimeError(
                "未登录 YouTube Studio，请先通过 qiyuan-worker mcp --profile social-youtube "
                "手动登录 Google 账号"
            )

    async def check_login(self, page: Any) -> bool:
        if "accounts.google.com" in page.url:
            return False
        login_link = page.locator(_SELECTORS["login_marker"]).first
        if await login_link.count() > 0:
            return False
        return True

    async def _do_upload_file(self, page: Any, video_path: Path) -> None:
        # Try to open the upload dialog via Create button.
        create_btn = page.locator(_SELECTORS["create_button"]).first
        if await create_btn.count() > 0:
            await create_btn.click()
            await page.wait_for_timeout(1000)

            upload_option = page.locator(_SELECTORS["upload_option"]).first
            if await upload_option.count() > 0:
                await upload_option.click()
                await page.wait_for_timeout(2000)

        # Set the file via the file input.
        file_input = page.locator(_SELECTORS["file_input"]).first
        await file_input.wait_for(state="attached", timeout=15_000)
        await file_input.set_input_files(str(video_path))
        logger.info("youtube: video file set via input: %s", video_path.name)

        # Wait for upload to start processing.
        await page.wait_for_timeout(5000)

    async def _do_fill_metadata(self, page: Any, metadata: UploadMetadata) -> None:
        # Fill title.
        title_input = page.locator(_SELECTORS["title_input"]).first
        if await title_input.count() > 0:
            await title_input.click()
            await page.keyboard.press("Control+A")
            await page.keyboard.press("Backspace")
            await title_input.fill(metadata.title or "Untitled")
            logger.info("youtube: filled title: %s", metadata.title[:60] if metadata.title else "Untitled")
        else:
            logger.warning("youtube: title input not found")

        # Fill description.
        if metadata.description:
            desc_input = page.locator(_SELECTORS["description_input"]).first
            if await desc_input.count() > 0:
                await desc_input.click()
                desc_text = metadata.description
                if metadata.tags:
                    tag_str = " ".join(f"#{tag}" for tag in metadata.tags)
                    desc_text = f"{desc_text}\n\n{tag_str}"
                await desc_input.fill(desc_text)
                logger.info("youtube: filled description (%d chars)", len(desc_text))

        await page.wait_for_timeout(1000)

    async def _do_set_visibility(self, page: Any, visibility: str) -> None:
        # YouTube Studio uses a multi-step wizard.  The visibility step is
        # the last step before "Done".  We need to click "Next" through
        # the intermediate steps first.
        next_btn = page.locator(_SELECTORS["next_button"]).first
        for step in range(3):  # Details → Video elements → Checks → Visibility
            if await next_btn.count() > 0 and await next_btn.is_enabled():
                await next_btn.click()
                await page.wait_for_timeout(1500)
                logger.info("youtube: clicked Next (step %d)", step + 1)

        # Now select the visibility radio.
        yt_name = _VISIBILITY_MAP.get(visibility, "PRIVATE")
        radio_selector = f'tp-yt-paper-radio-button[name="{yt_name}"]'
        radio = page.locator(radio_selector).first
        if await radio.count() > 0:
            await radio.click()
            logger.info("youtube: set visibility to %s", yt_name)
        else:
            logger.warning("youtube: visibility radio '%s' not found, defaulting to current", yt_name)

        await page.wait_for_timeout(1000)

    async def _do_submit(self, page: Any, publish: bool = False) -> None:
        # The "Done" button publishes / saves the video.
        done_btn = page.locator(_SELECTORS["done_button"]).first
        if await done_btn.count() > 0:
            await done_btn.wait_for(state="visible", timeout=30_000)
            await done_btn.click()
            logger.info("youtube: clicked Done button (publish=%s)", publish)
        else:
            logger.warning("youtube: Done button not found")

        await page.wait_for_timeout(3000)
