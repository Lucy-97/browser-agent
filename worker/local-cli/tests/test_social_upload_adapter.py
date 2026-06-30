from __future__ import annotations

from pathlib import Path
import tempfile
import unittest

from qiyuan_worker.adapters.social.upload import SocialUploadAdapter
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

    async def test_upload_adapter_stops_at_manual_action(self) -> None:
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
                policy={"manual_publish_required": True},
            )
            context = JobContext(
                job=job,
                config=config,
                api_client=object(),
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            result = await SocialUploadAdapter("tiktok").run(context)

            self.assertEqual(result.status, "needs_manual_action")
            self.assertIsNotNone(result.manual_action)
            self.assertFalse(result.manual_action.payload["publish_allowed"])


if __name__ == "__main__":
    unittest.main()
