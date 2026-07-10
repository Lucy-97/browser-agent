"""Instagram Reels video uploader.

Uploads videos through Instagram's web create flow using a persistent
Playwright profile.  The DOM changes often, so selectors are intentionally
grouped and the adapter falls back to manual action when a step cannot be
completed safely.
"""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

from qiyuan_worker.adapters.social.base import PlatformUploader, UploadMetadata

logger = logging.getLogger(__name__)

_SELECTORS = {
    "file_input": 'input[type="file"]',
    "new_post": (
        'svg[aria-label="New post"]',
        'svg[aria-label="Create"]',
        'svg[aria-label="创建"]',
        'svg[aria-label="新帖子"]',
        'div[role="button"]:has(svg[aria-label="New post"])',
        'div[role="button"]:has(svg[aria-label="Create"])',
        'div[role="button"]:has(svg[aria-label="创建"])',
        'div[role="button"]:has(svg[aria-label="新帖子"])',
        'a[href="/create/select/"]',
        'div[role="button"]:has-text("Create")',
        'div[role="button"]:has-text("创建")',
    ),
    "select_from_computer": (
        'button:has-text("Select from computer")',
        'button:has-text("从电脑中选择")',
    ),
    "caption_editor": (
        'div[aria-label="Write a caption..."][contenteditable="true"]',
        'div[aria-label="撰写说明文字..."][contenteditable="true"]',
        'div[role="textbox"][contenteditable="true"]',
    ),
    "next_button": (
        'div[role="button"]:has-text("Next")',
        'div[role="button"]:has-text("下一步")',
        'div[role="button"]:has-text("Continue")',
        'div[role="button"]:has-text("继续")',
    ),
    "share_button": (
        'div[role="dialog"] div[role="button"]:has-text("Share")',
        'div[role="dialog"] div[role="button"]:has-text("分享")',
        'div[role="button"]:has-text("Share")',
        'div[role="button"]:has-text("分享")',
    ),
    "share_success": (
        'text="Your reel has been shared."',
        'text="Your post has been shared."',
        'text="Reel shared"',
        'text="Post shared"',
        'text="已分享"',
    ),
    "reels_info_confirm": (
        'button:has-text("确定")',
        'button:has-text("OK")',
        'button:has-text("Got it")',
        'div[role="button"]:has-text("确定")',
        'div[role="button"]:has-text("OK")',
        'div[role="button"]:has-text("Got it")',
    ),
    "login_marker": (
        'input[name="username"]',
        'input[name="password"]',
        'button:has-text("Log in")',
        'button:has-text("登录")',
        'text="Log in"',
        'text="Sign up"',
        'text="登录"',
        'text="注册"',
    ),
}


class InstagramUploader(PlatformUploader):
    platform = "instagram"
    upload_url = "https://www.instagram.com/"

    async def _do_navigate(self, page: Any) -> None:
        await page.goto(self.upload_url, wait_until="domcontentloaded", timeout=60_000)
        await page.wait_for_timeout(3000)

        if not await self.check_login(page):
            raise RuntimeError(
                "未登录 Instagram，请先通过 qiyuan-worker mcp --profile social-instagram 手动登录"
            )

        for selector in _SELECTORS["new_post"]:
            button = page.locator(selector).first
            if await button.count() > 0:
                await button.click()
                await page.wait_for_timeout(1500)
                break
        if not await self.is_upload_ready(page):
            raise RuntimeError(
                "Instagram 尚未进入创建/上传界面，请在本机浏览器中完成登录或安全验证"
            )

    async def check_login(self, page: Any) -> bool:
        current_url = page.url.lower()
        login_paths = (
            "/accounts/login",
            "/accounts/emailsignup",
            "/auth_platform/",
            "/challenge/",
        )
        if any(path in current_url for path in login_paths):
            return False
        for selector in _SELECTORS["login_marker"]:
            marker = page.locator(selector).first
            if await marker.count() > 0:
                try:
                    if await marker.is_visible():
                        return False
                except Exception:
                    return False
        return True

    async def is_upload_ready(self, page: Any) -> bool:
        if not await self.check_login(page):
            return False
        for selector in (_SELECTORS["file_input"], *_SELECTORS["select_from_computer"]):
            locator = page.locator(selector).first
            if await locator.count() > 0:
                try:
                    if selector == _SELECTORS["file_input"] or await locator.is_visible():
                        return True
                except Exception:
                    return selector == _SELECTORS["file_input"]
        return False

    async def _do_upload_file(self, page: Any, video_path: Path) -> None:
        select_button = None
        for selector in _SELECTORS["select_from_computer"]:
            locator = page.locator(selector).first
            if await locator.count() > 0:
                select_button = locator
                break
        if select_button is not None:
            await select_button.click()
            await page.wait_for_timeout(1000)

        file_input = page.locator(_SELECTORS["file_input"]).first
        await file_input.wait_for(state="attached", timeout=15_000)
        await file_input.set_input_files(str(video_path))
        logger.info("instagram: video file set via input: %s", video_path.name)
        await page.wait_for_timeout(5000)
        await self._dismiss_reels_info_dialog(page)

    async def _do_fill_metadata(self, page: Any, metadata: UploadMetadata) -> None:
        await self._click_next(page)
        await self._click_next(page)

        caption_editor = None
        for selector in _SELECTORS["caption_editor"]:
            locator = page.locator(selector).first
            if await locator.count() > 0:
                caption_editor = locator
                break

        if caption_editor is None:
            logger.warning("instagram: caption editor not found, skipping metadata fill")
            return

        caption = metadata.title
        if metadata.description:
            caption = f"{caption}\n\n{metadata.description}" if caption else metadata.description
        if metadata.tags:
            tag_str = " ".join(f"#{tag}" for tag in metadata.tags)
            caption = f"{caption}\n\n{tag_str}" if caption else tag_str

        await caption_editor.click()
        await page.keyboard.press("Control+A")
        await page.keyboard.press("Backspace")
        try:
            await caption_editor.fill(caption)
        except Exception:
            await page.keyboard.type(caption, delay=30)
        logger.info("instagram: filled caption (%d chars)", len(caption))
        await page.wait_for_timeout(1000)

    async def _do_set_visibility(self, page: Any, visibility: str) -> None:
        logger.info("instagram: visibility=%s (Instagram web publish flow has no draft visibility step)", visibility)

    async def _do_submit(self, page: Any, publish: bool = False) -> None:
        if not publish:
            raise RuntimeError("Instagram Web 不支持可靠草稿保存，请人工确认后发布或关闭页面")

        share_button = None
        for selector in _SELECTORS["share_button"]:
            candidate = page.locator(selector).first
            if await candidate.count() > 0:
                share_button = candidate
                break
        if share_button is None:
            raise RuntimeError("Instagram Share button not found")

        await share_button.wait_for(state="visible", timeout=30_000)
        try:
            await share_button.click()
        except Exception as exc:
            logger.info("instagram: normal Share click failed, retrying with force: %s", exc)
            await share_button.click(force=True)
        logger.info("instagram: clicked Share button")
        await self._wait_for_share_completion(page)

    async def _click_next(self, page: Any) -> None:
        await self._dismiss_reels_info_dialog(page)
        for selector in _SELECTORS["next_button"]:
            button = page.locator(selector).first
            if await button.count() > 0:
                await button.wait_for(state="visible", timeout=10_000)
                await button.click()
                await page.wait_for_timeout(1500)
                await self._dismiss_reels_info_dialog(page)
                return
        logger.warning("instagram: Next button not found")

    async def _dismiss_reels_info_dialog(self, page: Any) -> None:
        for selector in _SELECTORS["reels_info_confirm"]:
            button = page.locator(selector).first
            if await button.count() == 0:
                continue
            try:
                if not await button.is_visible():
                    continue
            except Exception:
                pass
            await button.click()
            logger.info("instagram: dismissed Reels info dialog")
            await page.wait_for_timeout(1000)
            return

    async def _wait_for_share_completion(self, page: Any) -> None:
        for _ in range(45):
            await page.wait_for_timeout(1000)
            for selector in _SELECTORS["share_success"]:
                marker = page.locator(selector).first
                if await marker.count() == 0:
                    continue
                try:
                    if await marker.is_visible():
                        logger.info("instagram: share completion marker detected")
                        return
                except Exception:
                    logger.info("instagram: share completion marker detected")
                    return

            share_visible = False
            for selector in _SELECTORS["share_button"]:
                button = page.locator(selector).first
                if await button.count() == 0:
                    continue
                try:
                    if await button.is_visible():
                        share_visible = True
                        break
                except Exception:
                    share_visible = True
                    break
            if not share_visible:
                logger.info("instagram: Share button disappeared after publish")
                return

        logger.warning("instagram: share completion marker not detected before timeout")
