from __future__ import annotations

from pathlib import Path
import tempfile
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

from qiyuan_worker.adapters.social.upload import SocialUploadAdapter
from qiyuan_worker.adapters.social.base import UploadStepResult
from qiyuan_worker.artifacts import ArtifactCollector
from qiyuan_worker.config import write_default_config
from qiyuan_worker.protocols import AutomationJob
from qiyuan_worker.runtime.context import JobContext


class SocialUploadAdapterTest(unittest.IsolatedAsyncioTestCase):
    async def test_upload_adapter_requires_video_input(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            job = AutomationJob(
                job_id="job_1",
                run_id="run_1",
                job_type="social.youtube.upload_video",
                adapter="social.youtube.upload_video",
                target={"allowed_domains": ["studio.youtube.com"]},
                input={},
                policy={},
            )
            context = JobContext(
                job=job,
                config=config,
                api_client=object(),
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            result = await SocialUploadAdapter("youtube").run(context)

            self.assertEqual(result.status, "failed")
            self.assertEqual(result.error_code, "SOCIAL_VIDEO_REQUIRED")

    async def test_upload_adapter_waits_for_login_and_continues(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            video_path = tmp_path / "demo.mp4"
            video_path.write_bytes(b"fake-video")
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            job = AutomationJob(
                job_id="job_1",
                run_id="run_1",
                job_type="social.tiktok.upload_video",
                adapter="social.tiktok.upload_video",
                target={"allowed_domains": ["www.tiktok.com"]},
                input={"video_path": str(video_path), "title": "draft"},
                policy={
                    "manual_publish_required": False,
                    "manual_login_timeout_seconds": 1,
                    "manual_login_poll_seconds": 0,
                },
            )
            api_client = MagicMock()
            context = JobContext(
                job=job,
                config=config,
                api_client=api_client,
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            uploader = MagicMock()
            uploader.navigate = AsyncMock(
                side_effect=[
                    UploadStepResult(success=False, step="navigate", message="未登录 TikTok"),
                    UploadStepResult(success=True, step="navigate"),
                ]
            )
            uploader.check_login = AsyncMock(return_value=True)
            uploader.is_upload_ready = AsyncMock(side_effect=[False, True])
            uploader.upload_file = AsyncMock(return_value=UploadStepResult(success=True, step="upload_file"))
            uploader.fill_metadata = AsyncMock(return_value=UploadStepResult(success=True, step="fill_metadata"))
            uploader.set_visibility = AsyncMock(return_value=UploadStepResult(success=True, step="set_visibility"))
            uploader.submit = AsyncMock(return_value=UploadStepResult(success=True, step="submit"))
            page = AsyncMock()
            page.screenshot = AsyncMock()
            session = AsyncMock()
            session.__aenter__.return_value = page
            session.__aexit__.return_value = None
            runtime = MagicMock()
            runtime.open_page = AsyncMock(return_value=session)

            with (
                patch("qiyuan_worker.adapters.social.upload._get_uploader", return_value=uploader),
                patch("qiyuan_worker.adapters.social.upload.BrowserRuntime", return_value=runtime),
            ):
                result = await SocialUploadAdapter("tiktok").run(context)

            self.assertEqual(result.status, "completed")
            self.assertIsNone(result.manual_action)
            self.assertEqual(result.summary["completed_steps"], ["navigate", "upload_file", "fill_metadata", "set_visibility", "submit"])
            self.assertEqual(uploader.navigate.await_count, 2)
            self.assertGreaterEqual(uploader.is_upload_ready.await_count, 1)
            api_client.run_heartbeat.assert_called()

    async def test_upload_adapter_returns_manual_action_after_login_timeout(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            video_path = tmp_path / "demo.mp4"
            video_path.write_bytes(b"fake-video")
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            job = AutomationJob(
                job_id="job_1",
                run_id="run_1",
                job_type="social.tiktok.upload_video",
                adapter="social.tiktok.upload_video",
                target={"allowed_domains": ["www.tiktok.com"]},
                input={"video_path": str(video_path), "title": "draft"},
                policy={
                    "manual_publish_required": True,
                    "manual_login_timeout_seconds": 0.01,
                    "manual_login_poll_seconds": 0,
                },
            )
            api_client = MagicMock()
            context = JobContext(
                job=job,
                config=config,
                api_client=api_client,
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            uploader = MagicMock()
            uploader.navigate = AsyncMock(return_value=UploadStepResult(success=False, step="navigate", message="未登录 TikTok"))
            uploader.check_login = AsyncMock(return_value=False)
            uploader.is_upload_ready = AsyncMock(return_value=False)
            page = AsyncMock()
            page.screenshot = AsyncMock()
            session = AsyncMock()
            session.__aenter__.return_value = page
            session.__aexit__.return_value = None
            runtime = MagicMock()
            runtime.open_page = AsyncMock(return_value=session)

            with (
                patch("qiyuan_worker.adapters.social.upload._get_uploader", return_value=uploader),
                patch("qiyuan_worker.adapters.social.upload.BrowserRuntime", return_value=runtime),
            ):
                result = await SocialUploadAdapter("tiktok").run(context)

            self.assertEqual(result.status, "needs_manual_action")
            self.assertIsNotNone(result.manual_action)
            self.assertEqual(result.summary["failed_step"], "navigate")
            self.assertIn("等待人工登录超时", result.summary["error"])
            api_client.run_heartbeat.assert_called()

    async def test_upload_adapter_waits_for_login_after_mid_flow_challenge(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            video_path = tmp_path / "demo.mp4"
            video_path.write_bytes(b"fake-video")
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            job = AutomationJob(
                job_id="job_1",
                run_id="run_1",
                job_type="social.instagram.upload_video",
                adapter="social.instagram.upload_video",
                target={"allowed_domains": ["www.instagram.com"]},
                input={"video_path": str(video_path), "title": "draft"},
                policy={
                    "manual_publish_required": False,
                    "manual_login_timeout_seconds": 1,
                    "manual_login_poll_seconds": 0,
                },
            )
            api_client = MagicMock()
            context = JobContext(
                job=job,
                config=config,
                api_client=api_client,
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            uploader = MagicMock()
            uploader.navigate = AsyncMock(return_value=UploadStepResult(success=True, step="navigate"))
            uploader.check_login = AsyncMock(return_value=False)
            uploader.is_upload_ready = AsyncMock(return_value=True)
            uploader.upload_file = AsyncMock(
                side_effect=[
                    UploadStepResult(success=False, step="upload_file", message="Instagram security challenge"),
                    UploadStepResult(success=True, step="upload_file"),
                ]
            )
            uploader.fill_metadata = AsyncMock(return_value=UploadStepResult(success=True, step="fill_metadata"))
            uploader.set_visibility = AsyncMock(return_value=UploadStepResult(success=True, step="set_visibility"))
            uploader.submit = AsyncMock(return_value=UploadStepResult(success=True, step="submit"))
            page = AsyncMock()
            page.screenshot = AsyncMock()
            session = AsyncMock()
            session.__aenter__.return_value = page
            session.__aexit__.return_value = None
            runtime = MagicMock()
            runtime.open_page = AsyncMock(return_value=session)

            with (
                patch("qiyuan_worker.adapters.social.upload._get_uploader", return_value=uploader),
                patch("qiyuan_worker.adapters.social.upload.BrowserRuntime", return_value=runtime),
            ):
                result = await SocialUploadAdapter("instagram").run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(uploader.upload_file.await_count, 2)
            api_client.run_heartbeat.assert_called()

    async def test_instagram_waits_before_manual_publish_action(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            video_path = tmp_path / "demo.mp4"
            video_path.write_bytes(b"fake-video")
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            job = AutomationJob(
                job_id="job_1",
                run_id="run_1",
                job_type="social.instagram.upload_video",
                adapter="social.instagram.upload_video",
                target={"allowed_domains": ["www.instagram.com"]},
                input={"video_path": str(video_path), "title": "draft"},
                policy={
                    "manual_publish_required": True,
                    "manual_publish_timeout_seconds": 0.01,
                    "manual_publish_poll_seconds": 0,
                },
            )
            api_client = MagicMock()
            context = JobContext(
                job=job,
                config=config,
                api_client=api_client,
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            uploader = MagicMock()
            uploader.navigate = AsyncMock(return_value=UploadStepResult(success=True, step="navigate"))
            uploader.upload_file = AsyncMock(return_value=UploadStepResult(success=True, step="upload_file"))
            uploader.fill_metadata = AsyncMock(return_value=UploadStepResult(success=True, step="fill_metadata"))
            uploader.set_visibility = AsyncMock(return_value=UploadStepResult(success=True, step="set_visibility"))
            uploader.submit = AsyncMock(return_value=UploadStepResult(success=True, step="submit"))
            page = AsyncMock()
            page.screenshot = AsyncMock()
            session = AsyncMock()
            session.__aenter__.return_value = page
            session.__aexit__.return_value = None
            runtime = MagicMock()
            runtime.open_page = AsyncMock(return_value=session)

            with (
                patch("qiyuan_worker.adapters.social.upload._get_uploader", return_value=uploader),
                patch("qiyuan_worker.adapters.social.upload.BrowserRuntime", return_value=runtime),
            ):
                result = await SocialUploadAdapter("instagram").run(context)

            self.assertEqual(result.status, "needs_manual_action")
            self.assertEqual(result.summary["failed_step"], "submit")
            uploader.submit.assert_not_called()
            api_client.run_heartbeat.assert_called()


if __name__ == "__main__":
    unittest.main()
