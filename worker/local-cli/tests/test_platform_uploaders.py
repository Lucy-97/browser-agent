"""Unit tests for platform-specific uploaders and SocialUploadAdapter.

All tests mock Playwright Page and BrowserRuntime to avoid a real browser.
"""

from __future__ import annotations

import asyncio
import tempfile
import unittest
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

from qiyuan_worker.adapters.social.base import PlatformUploader, UploadMetadata, UploadStepResult
from qiyuan_worker.adapters.social.douyin import DouyinUploader
from qiyuan_worker.adapters.social.instagram import InstagramUploader
from qiyuan_worker.adapters.social.tiktok import TikTokUploader
from qiyuan_worker.adapters.social.youtube import YouTubeUploader
from qiyuan_worker.adapters.social.upload import SocialUploadAdapter, _get_uploader


def run(coro):
    return asyncio.run(coro)


# ---------------------------------------------------------------------------
# _get_uploader
# ---------------------------------------------------------------------------

class GetUploaderTest(unittest.TestCase):
    def test_douyin(self) -> None:
        self.assertIsInstance(_get_uploader("douyin"), DouyinUploader)

    def test_tiktok(self) -> None:
        self.assertIsInstance(_get_uploader("tiktok"), TikTokUploader)

    def test_youtube(self) -> None:
        self.assertIsInstance(_get_uploader("youtube"), YouTubeUploader)

    def test_instagram(self) -> None:
        self.assertIsInstance(_get_uploader("instagram"), InstagramUploader)

    def test_unsupported(self) -> None:
        with self.assertRaises(ValueError):
            _get_uploader("weibo")


# ---------------------------------------------------------------------------
# PlatformUploader base – error wrapping
# ---------------------------------------------------------------------------

class ConcreteUploader(PlatformUploader):
    """Minimal concrete subclass for testing the base class logic."""
    platform = "test"
    upload_url = "https://example.com/upload"

    async def _do_navigate(self, page): pass
    async def _do_upload_file(self, page, video_path): pass
    async def _do_fill_metadata(self, page, metadata): pass
    async def _do_set_visibility(self, page, visibility): pass
    async def _do_submit(self, page, publish=False): pass


class FailingUploader(PlatformUploader):
    """Uploader whose steps all raise."""
    platform = "fail"
    upload_url = "https://example.com/fail"

    async def _do_navigate(self, page): raise RuntimeError("nav error")
    async def _do_upload_file(self, page, video_path): raise RuntimeError("upload error")
    async def _do_fill_metadata(self, page, metadata): raise RuntimeError("meta error")
    async def _do_set_visibility(self, page, visibility): raise RuntimeError("vis error")
    async def _do_submit(self, page, publish=False): raise RuntimeError("submit error")


class BaseUploaderTest(unittest.TestCase):
    def test_success_returns_true(self) -> None:
        u = ConcreteUploader()
        page = MagicMock()
        result = run(u.navigate(page))
        self.assertTrue(result.success)
        self.assertEqual(result.step, "navigate")

    def test_failure_returns_false(self) -> None:
        u = FailingUploader()
        page = MagicMock()
        result = run(u.navigate(page))
        self.assertFalse(result.success)
        self.assertIn("nav error", result.message)

    def test_all_steps_wrapped(self) -> None:
        u = FailingUploader()
        page = MagicMock()
        for step_name in ("navigate", "upload_file", "fill_metadata", "set_visibility", "submit"):
            fn = getattr(u, step_name)
            if step_name == "upload_file":
                result = run(fn(page, Path("/tmp/test.mp4")))
            elif step_name == "fill_metadata":
                result = run(fn(page, UploadMetadata()))
            elif step_name == "set_visibility":
                result = run(fn(page, "private"))
            elif step_name == "submit":
                result = run(fn(page, publish=False))
            else:
                result = run(fn(page))
            self.assertFalse(result.success, f"step {step_name} should fail")
            self.assertEqual(result.step, step_name)


# ---------------------------------------------------------------------------
# DouyinUploader
# ---------------------------------------------------------------------------

def _make_douyin_page(logged_in: bool = True):
    page = AsyncMock()
    page.url = "https://creator.douyin.com/creator-micro/content/upload"
    # get_by_text mocking
    no_login = MagicMock()
    no_login.first = MagicMock()
    no_login.first.count = AsyncMock(return_value=0 if logged_in else 1)
    page.get_by_text = MagicMock(return_value=no_login)

    # Locator mock
    file_input = AsyncMock()
    file_input.count = AsyncMock(return_value=1)
    file_input.wait_for = AsyncMock()
    file_input.set_input_files = AsyncMock()

    title_editor = AsyncMock()
    title_editor.count = AsyncMock(return_value=1)
    title_editor.click = AsyncMock()
    title_editor.fill = AsyncMock()

    draft_btn = AsyncMock()
    draft_btn.count = AsyncMock(return_value=1)
    draft_btn.click = AsyncMock()

    def locator_side_effect(selector):
        mock = MagicMock()
        if "file" in selector:
            mock.first = file_input
        elif "draft" in selector.lower() or "存草稿" in selector:
            mock.first = draft_btn
        elif "contenteditable" in selector or "editor" in selector.lower():
            mock.first = title_editor
        else:
            generic = AsyncMock()
            generic.count = AsyncMock(return_value=0)
            mock.first = generic
        return mock

    page.locator = MagicMock(side_effect=locator_side_effect)
    page.keyboard = AsyncMock()
    page.wait_for_timeout = AsyncMock()
    page.goto = AsyncMock()
    return page


class DouyinUploaderTest(unittest.TestCase):
    def test_navigate_success(self) -> None:
        u = DouyinUploader()
        page = _make_douyin_page(logged_in=True)
        result = run(u.navigate(page))
        self.assertTrue(result.success)

    def test_navigate_not_logged_in(self) -> None:
        u = DouyinUploader()
        page = _make_douyin_page(logged_in=False)
        result = run(u.navigate(page))
        self.assertFalse(result.success)
        self.assertIn("未登录", result.message)

    def test_upload_file(self) -> None:
        u = DouyinUploader()
        page = _make_douyin_page()
        result = run(u.upload_file(page, Path("/tmp/test.mp4")))
        self.assertTrue(result.success)

    def test_fill_metadata(self) -> None:
        u = DouyinUploader()
        page = _make_douyin_page()
        meta = UploadMetadata(title="测试视频", tags=("技术", "AI"))
        result = run(u.fill_metadata(page, meta))
        self.assertTrue(result.success)

    def test_submit_draft(self) -> None:
        u = DouyinUploader()
        page = _make_douyin_page()
        result = run(u.submit(page, publish=False))
        self.assertTrue(result.success)


# ---------------------------------------------------------------------------
# TikTokUploader
# ---------------------------------------------------------------------------

def _make_tiktok_page(logged_in: bool = True):
    page = AsyncMock()
    page.url = "https://www.tiktok.com/creator-center/upload"

    login_btn = AsyncMock()
    login_btn.count = AsyncMock(return_value=0 if logged_in else 1)
    login_btn.is_visible = AsyncMock(return_value=not logged_in)

    file_input = AsyncMock()
    file_input.count = AsyncMock(return_value=1)
    file_input.wait_for = AsyncMock()
    file_input.set_input_files = AsyncMock()

    caption_editor = AsyncMock()
    caption_editor.count = AsyncMock(return_value=1)
    caption_editor.click = AsyncMock()
    caption_editor.fill = AsyncMock()

    draft_btn = AsyncMock()
    draft_btn.count = AsyncMock(return_value=1)
    draft_btn.click = AsyncMock()

    def locator_side_effect(selector):
        mock = MagicMock()
        if "Log in" in selector or "登录 TikTok" in selector or "二维码" in selector or "手机号" in selector:
            mock.first = login_btn
        elif "file" in selector:
            mock.first = file_input
        elif "contenteditable" in selector:
            mock.first = caption_editor
        elif "draft" in selector.lower() or "Draft" in selector:
            mock.first = draft_btn
        else:
            generic = AsyncMock()
            generic.count = AsyncMock(return_value=0)
            mock.first = generic
        return mock

    page.locator = MagicMock(side_effect=locator_side_effect)
    page.keyboard = AsyncMock()
    page.wait_for_timeout = AsyncMock()
    page.goto = AsyncMock()
    return page


class TikTokUploaderTest(unittest.TestCase):
    def test_navigate_logged_in(self) -> None:
        u = TikTokUploader()
        page = _make_tiktok_page(logged_in=True)
        result = run(u.navigate(page))
        self.assertTrue(result.success)

    def test_navigate_not_logged_in(self) -> None:
        u = TikTokUploader()
        page = _make_tiktok_page(logged_in=False)
        result = run(u.navigate(page))
        self.assertFalse(result.success)

    def test_chinese_login_page_is_not_logged_in(self) -> None:
        u = TikTokUploader()
        page = _make_tiktok_page(logged_in=False)
        result = run(u.check_login(page))
        self.assertFalse(result)

    def test_upload_ready_requires_file_input(self) -> None:
        u = TikTokUploader()
        page = _make_tiktok_page(logged_in=True)
        result = run(u.is_upload_ready(page))
        self.assertTrue(result)

    def test_upload_file(self) -> None:
        u = TikTokUploader()
        page = _make_tiktok_page()
        result = run(u.upload_file(page, Path("/tmp/test.mp4")))
        self.assertTrue(result.success)


# ---------------------------------------------------------------------------
# YouTubeUploader
# ---------------------------------------------------------------------------

def _make_youtube_page(logged_in: bool = True):
    page = AsyncMock()
    page.url = "https://studio.youtube.com/channel/123"

    login_link = AsyncMock()
    login_link.count = AsyncMock(return_value=0 if logged_in else 1)

    create_btn = AsyncMock()
    create_btn.count = AsyncMock(return_value=1)
    create_btn.click = AsyncMock()

    upload_option = AsyncMock()
    upload_option.count = AsyncMock(return_value=1)
    upload_option.click = AsyncMock()

    file_input = AsyncMock()
    file_input.count = AsyncMock(return_value=1)
    file_input.wait_for = AsyncMock()
    file_input.set_input_files = AsyncMock()

    title_input = AsyncMock()
    title_input.count = AsyncMock(return_value=1)
    title_input.click = AsyncMock()
    title_input.fill = AsyncMock()

    desc_input = AsyncMock()
    desc_input.count = AsyncMock(return_value=1)
    desc_input.click = AsyncMock()
    desc_input.fill = AsyncMock()

    next_btn = AsyncMock()
    next_btn.count = AsyncMock(return_value=1)
    next_btn.is_enabled = AsyncMock(return_value=True)
    next_btn.click = AsyncMock()

    radio = AsyncMock()
    radio.count = AsyncMock(return_value=1)
    radio.click = AsyncMock()

    done_btn = AsyncMock()
    done_btn.count = AsyncMock(return_value=1)
    done_btn.wait_for = AsyncMock()
    done_btn.click = AsyncMock()

    def locator_side_effect(selector):
        mock = MagicMock()
        if "ServiceLogin" in selector:
            mock.first = login_link
        elif "create-icon" in selector:
            mock.first = create_btn
        elif "text-item" in selector or "Upload videos" in selector:
            mock.first = upload_option
        elif 'type="file"' in selector:
            mock.first = file_input
        elif "title-textarea" in selector or '#textbox' in selector:
            mock.first = title_input
        elif "description-textarea" in selector:
            mock.first = desc_input
        elif "next-button" in selector:
            mock.first = next_btn
        elif "paper-radio-button" in selector:
            mock.first = radio
        elif "done-button" in selector:
            mock.first = done_btn
        else:
            generic = AsyncMock()
            generic.count = AsyncMock(return_value=0)
            mock.first = generic
        return mock

    page.locator = MagicMock(side_effect=locator_side_effect)
    page.keyboard = AsyncMock()
    page.wait_for_timeout = AsyncMock()
    page.goto = AsyncMock()
    return page


class YouTubeUploaderTest(unittest.TestCase):
    def test_navigate_logged_in(self) -> None:
        u = YouTubeUploader()
        page = _make_youtube_page(logged_in=True)
        result = run(u.navigate(page))
        self.assertTrue(result.success)

    def test_navigate_not_logged_in(self) -> None:
        u = YouTubeUploader()
        page = _make_youtube_page(logged_in=False)
        result = run(u.navigate(page))
        self.assertFalse(result.success)

    def test_upload_file(self) -> None:
        u = YouTubeUploader()
        page = _make_youtube_page()
        result = run(u.upload_file(page, Path("/tmp/test.mp4")))
        self.assertTrue(result.success)

    def test_fill_metadata(self) -> None:
        u = YouTubeUploader()
        page = _make_youtube_page()
        meta = UploadMetadata(title="Test Video", description="A description", tags=("tech", "AI"))
        result = run(u.fill_metadata(page, meta))
        self.assertTrue(result.success)

    def test_set_visibility(self) -> None:
        u = YouTubeUploader()
        page = _make_youtube_page()
        result = run(u.set_visibility(page, "unlisted"))
        self.assertTrue(result.success)

    def test_submit(self) -> None:
        u = YouTubeUploader()
        page = _make_youtube_page()
        result = run(u.submit(page, publish=False))
        self.assertTrue(result.success)


# ---------------------------------------------------------------------------
# InstagramUploader
# ---------------------------------------------------------------------------

def _make_instagram_page(
    logged_in: bool = True,
    upload_ready: bool = True,
    url: str = "https://www.instagram.com/",
    reels_info_dialog: bool = False,
    share_success: bool = True,
    share_click_error: Exception | None = None,
):
    page = AsyncMock()
    page.url = url

    login_marker = AsyncMock()
    login_marker.count = AsyncMock(return_value=0 if logged_in else 1)
    login_marker.is_visible = AsyncMock(return_value=not logged_in)

    new_post = AsyncMock()
    new_post.count = AsyncMock(return_value=1)
    new_post.click = AsyncMock()

    select_button = AsyncMock()
    select_button.count = AsyncMock(return_value=1 if upload_ready else 0)
    select_button.is_visible = AsyncMock(return_value=upload_ready)
    select_button.click = AsyncMock()

    file_input = AsyncMock()
    file_input.count = AsyncMock(return_value=1 if upload_ready else 0)
    file_input.wait_for = AsyncMock()
    file_input.set_input_files = AsyncMock()

    next_button = AsyncMock()
    next_button.count = AsyncMock(return_value=1)
    next_button.click = AsyncMock()

    caption_editor = AsyncMock()
    caption_editor.count = AsyncMock(return_value=1)
    caption_editor.click = AsyncMock()
    caption_editor.fill = AsyncMock()

    share_button = AsyncMock()
    share_button.count = AsyncMock(return_value=1)
    share_button.is_visible = AsyncMock(return_value=True)
    share_button.wait_for = AsyncMock()
    share_button.click = AsyncMock()
    if share_click_error is not None:
        share_button.click = AsyncMock(side_effect=[share_click_error, None])

    share_success_marker = AsyncMock()
    share_success_marker.count = AsyncMock(return_value=1 if share_success else 0)
    share_success_marker.is_visible = AsyncMock(return_value=share_success)

    reels_confirm = AsyncMock()
    reels_confirm.count = AsyncMock(return_value=1 if reels_info_dialog else 0)
    reels_confirm.is_visible = AsyncMock(return_value=reels_info_dialog)
    reels_confirm.click = AsyncMock()

    def locator_side_effect(selector):
        mock = MagicMock()
        if "username" in selector or "password" in selector or "Log in" in selector:
            mock.first = login_marker
        elif "确定" in selector or "Got it" in selector or "OK" in selector:
            mock.first = reels_confirm
        elif "New post" in selector or "create/select" in selector or "Create" in selector:
            mock.first = new_post
        elif "Select from computer" in selector:
            mock.first = select_button
        elif 'type="file"' in selector:
            mock.first = file_input
        elif "Next" in selector or "Continue" in selector or "继续" in selector:
            mock.first = next_button
        elif "caption" in selector or "textbox" in selector:
            mock.first = caption_editor
        elif "Share" in selector:
            mock.first = share_button
        elif "shared" in selector or "已分享" in selector:
            mock.first = share_success_marker
        else:
            generic = AsyncMock()
            generic.count = AsyncMock(return_value=0)
            mock.first = generic
        return mock

    page.locator = MagicMock(side_effect=locator_side_effect)
    page.keyboard = AsyncMock()
    page.wait_for_timeout = AsyncMock()
    page.goto = AsyncMock()
    return page


class InstagramUploaderTest(unittest.TestCase):
    def test_navigate_logged_in(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page(logged_in=True)
        result = run(u.navigate(page))
        self.assertTrue(result.success)
        page.goto.assert_awaited_with("https://www.instagram.com/", wait_until="domcontentloaded", timeout=60_000)

    def test_navigate_not_logged_in(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page(logged_in=False)
        result = run(u.navigate(page))
        self.assertFalse(result.success)

    def test_auth_platform_page_is_not_logged_in(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page(
            logged_in=True,
            url="https://www.instagram.com/auth_platform/?apc=test",
        )
        result = run(u.check_login(page))
        self.assertFalse(result)

    def test_upload_ready_requires_create_upload_controls(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page(logged_in=True, upload_ready=False)
        result = run(u.is_upload_ready(page))
        self.assertFalse(result)

    def test_upload_file(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page()
        result = run(u.upload_file(page, Path("/tmp/test.mp4")))
        self.assertTrue(result.success)

    def test_upload_file_dismisses_reels_info_dialog(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page(reels_info_dialog=True)
        result = run(u.upload_file(page, Path("/tmp/test.mp4")))
        self.assertTrue(result.success)
        confirm_button = page.locator('button:has-text("确定")').first
        confirm_button.click.assert_awaited()

    def test_fill_metadata(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page()
        meta = UploadMetadata(title="Test Reel", description="A description", tags=("ai", "agents"))
        result = run(u.fill_metadata(page, meta))
        self.assertTrue(result.success)

    def test_submit_requires_manual_draft_when_publish_disabled(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page()
        result = run(u.submit(page, publish=False))
        self.assertFalse(result.success)
        self.assertIn("不支持可靠草稿保存", result.message)

    def test_submit_publish(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page()
        result = run(u.submit(page, publish=True))
        self.assertTrue(result.success)
        page.locator('text="Your reel has been shared."').first.is_visible.assert_awaited()

    def test_submit_publish_retries_with_force_when_overlay_intercepts_click(self) -> None:
        u = InstagramUploader()
        page = _make_instagram_page(share_click_error=RuntimeError("subtree intercepts pointer events"))
        result = run(u.submit(page, publish=True))
        self.assertTrue(result.success)
        share_button = page.locator('div[role="dialog"] div[role="button"]:has-text("Share")').first
        share_button.click.assert_any_await(force=True)


# ---------------------------------------------------------------------------
# SocialUploadAdapter integration
# ---------------------------------------------------------------------------

class SocialUploadAdapterTest(unittest.IsolatedAsyncioTestCase):
    async def test_requires_video_input(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            from qiyuan_worker.config import write_default_config
            from qiyuan_worker.protocols import AutomationJob
            from qiyuan_worker.runtime.context import JobContext
            from qiyuan_worker.artifacts import ArtifactCollector

            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            job = AutomationJob(
                job_id="job_1", run_id="run_1",
                job_type="social.douyin.upload_video",
                adapter="social.douyin.upload_video",
                target={}, input={}, policy={},
            )
            context = JobContext(
                job=job, config=config, api_client=object(),
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )
            result = await SocialUploadAdapter("douyin").run(context)
            self.assertEqual(result.status, "failed")
            self.assertEqual(result.error_code, "SOCIAL_VIDEO_REQUIRED")

    async def test_unsupported_platform_returns_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            from qiyuan_worker.config import write_default_config
            from qiyuan_worker.protocols import AutomationJob
            from qiyuan_worker.runtime.context import JobContext
            from qiyuan_worker.artifacts import ArtifactCollector

            video = tmp_path / "test.mp4"
            video.write_bytes(b"fake")
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            job = AutomationJob(
                job_id="job_2", run_id="run_2",
                job_type="social.weibo.upload_video",
                adapter="social.weibo.upload_video",
                target={}, input={"video_path": str(video)}, policy={},
            )
            context = JobContext(
                job=job, config=config, api_client=object(),
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )
            result = await SocialUploadAdapter("weibo").run(context)
            self.assertEqual(result.status, "failed")
            self.assertEqual(result.error_code, "SOCIAL_PLATFORM_UNSUPPORTED")


class BuiltinExtensionsTest(unittest.TestCase):
    def test_douyin_adapter_registered(self) -> None:
        from qiyuan_worker.builtin_extensions import builtin_worker_extensions
        extensions = builtin_worker_extensions()
        social_ext = [e for e in extensions if e.name == "social.upload"][0]
        adapter_names = [a.name for a in social_ext.adapters]
        self.assertIn("social.douyin.upload_video", adapter_names)
        self.assertIn("social.youtube.upload_video", adapter_names)
        self.assertIn("social.tiktok.upload_video", adapter_names)
        self.assertIn("social.instagram.upload_video", adapter_names)

    def test_douyin_capability_registered(self) -> None:
        from qiyuan_worker.builtin_extensions import builtin_worker_extensions
        extensions = builtin_worker_extensions()
        social_ext = [e for e in extensions if e.name == "social.upload"][0]
        self.assertIn("adapter.social.douyin.upload_video", social_ext.capabilities)
        self.assertIn("adapter.social.instagram.upload_video", social_ext.capabilities)


if __name__ == "__main__":
    unittest.main()
