from __future__ import annotations

from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from qiyuan_worker.adapters.weixin.desktop_sync import WeixinDesktopSyncAdapter
from qiyuan_worker.artifacts import ArtifactCollector
from qiyuan_worker.config import write_default_config
from qiyuan_worker.protocols import AutomationJob
from qiyuan_worker.runtime.context import JobContext


class WeixinDesktopSyncAdapterTest(unittest.IsolatedAsyncioTestCase):
    async def test_requires_source_dirs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            context = JobContext(
                job=AutomationJob(
                    job_id="job_1",
                    run_id="run_1",
                    job_type="weixin.desktop_sync",
                    adapter="weixin.desktop_sync",
                ),
                config=config,
                api_client=object(),
                artifact_collector=ArtifactCollector(tmp_path / "artifacts"),
                work_dir=tmp_path / "work",
            )

            result = await WeixinDesktopSyncAdapter().run(context)

            self.assertEqual(result.status, "failed")
            self.assertEqual(result.error_code, "WEIXIN_SOURCE_DIRS_REQUIRED")

    async def test_expands_home_env_in_source_dirs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            source_dir = tmp_path / "wechat-files"
            source_dir.mkdir()
            kept = source_dir / "paper.pdf"
            kept.write_bytes(b"pdf")

            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            collector = ArtifactCollector(tmp_path / "artifacts")
            context = JobContext(
                job=AutomationJob(
                    job_id="job_1",
                    run_id="run_1",
                    job_type="weixin.desktop_sync",
                    adapter="weixin.desktop_sync",
                    input={"source_dirs": ["$HOME/wechat-files"]},
                    policy={"max_files": 10},
                ),
                config=config,
                api_client=object(),
                artifact_collector=collector,
                work_dir=tmp_path / "work",
            )

            with patch.dict("os.environ", {"HOME": str(tmp_path)}, clear=False):
                result = await WeixinDesktopSyncAdapter().run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(result.summary["synced_count"], 1)

    async def test_syncs_files_and_manifest(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            source_dir = tmp_path / "wechat-files"
            group_dir = source_dir / "research-group"
            group_dir.mkdir(parents=True)
            kept = group_dir / "paper.pdf"
            kept.write_bytes(b"pdf")
            skipped = source_dir / "other-chat.txt"
            skipped.write_text("skip", encoding="utf-8")

            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            collector = ArtifactCollector(tmp_path / "artifacts")
            context = JobContext(
                job=AutomationJob(
                    job_id="job_1",
                    run_id="run_1",
                    job_type="weixin.desktop_sync",
                    adapter="weixin.desktop_sync",
                    input={
                        "source_dirs": [str(source_dir)],
                        "group_keywords": ["research-group"],
                    },
                    policy={"max_files": 10},
                ),
                config=config,
                api_client=object(),
                artifact_collector=collector,
                work_dir=tmp_path / "work",
            )

            result = await WeixinDesktopSyncAdapter().run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(result.summary["synced_count"], 1)
            self.assertIsNotNone(result.cursor)
            artifact_types = [artifact.artifact_type for artifact in collector.collected()]
            self.assertEqual(artifact_types.count("weixin_file"), 1)
            self.assertIn("weixin_manifest", artifact_types)

    async def test_group_names_drive_path_matching(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            source_dir = tmp_path / "wechat-files"
            group_dir = source_dir / "项目资料群"
            group_dir.mkdir(parents=True)
            kept = group_dir / "proposal.docx"
            kept.write_bytes(b"docx")
            skipped_dir = source_dir / "other-group"
            skipped_dir.mkdir()
            skipped = skipped_dir / "proposal.docx"
            skipped.write_bytes(b"other")

            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            collector = ArtifactCollector(tmp_path / "artifacts")
            context = JobContext(
                job=AutomationJob(
                    job_id="job_1",
                    run_id="run_1",
                    job_type="weixin.desktop_sync",
                    adapter="weixin.desktop_sync",
                    input={
                        "source_dirs": [str(source_dir)],
                        "group_names": ["项目资料群"],
                        "selected_groups": [{"display_name": "项目资料群"}],
                    },
                    policy={"max_files": 10},
                ),
                config=config,
                api_client=object(),
                artifact_collector=collector,
                work_dir=tmp_path / "work",
            )

            result = await WeixinDesktopSyncAdapter().run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(result.summary["synced_count"], 1)
            self.assertEqual(result.summary["group_names"], ["项目资料群"])

    async def test_skips_duplicate_file_content_in_same_run(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            source_dir = tmp_path / "wechat-files"
            source_dir.mkdir()
            first = source_dir / "a.pdf"
            first.write_bytes(b"same")
            duplicate = source_dir / "b.pdf"
            duplicate.write_bytes(b"same")

            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            collector = ArtifactCollector(tmp_path / "artifacts")
            context = JobContext(
                job=AutomationJob(
                    job_id="job_1",
                    run_id="run_1",
                    job_type="weixin.desktop_sync",
                    adapter="weixin.desktop_sync",
                    input={"source_dirs": [str(source_dir)]},
                    policy={"max_files": 10},
                ),
                config=config,
                api_client=object(),
                artifact_collector=collector,
                work_dir=tmp_path / "work",
            )

            result = await WeixinDesktopSyncAdapter().run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(result.summary["synced_count"], 1)
            self.assertEqual(result.summary["skipped_duplicate_count"], 1)
            artifact_types = [artifact.artifact_type for artifact in collector.collected()]
            self.assertEqual(artifact_types.count("weixin_file"), 1)
            self.assertIn("weixin_manifest", artifact_types)

    async def test_skips_cache_like_files(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            source_dir = tmp_path / "wechat-data"
            source_dir.mkdir()
            kept = source_dir / "report.docx"
            kept.write_bytes(b"docx")
            cache_without_suffix = source_dir / "CURRENT"
            cache_without_suffix.write_bytes(b"cache")
            cache_bin = source_dir / "storage_runtime.bin"
            cache_bin.write_bytes(b"cache")

            config = write_default_config(config_path=tmp_path / "config.yaml", data_dir=tmp_path / "data")
            collector = ArtifactCollector(tmp_path / "artifacts")
            context = JobContext(
                job=AutomationJob(
                    job_id="job_1",
                    run_id="run_1",
                    job_type="weixin.desktop_sync",
                    adapter="weixin.desktop_sync",
                    input={"source_dirs": [str(source_dir)]},
                    policy={"max_files": 10},
                ),
                config=config,
                api_client=object(),
                artifact_collector=collector,
                work_dir=tmp_path / "work",
            )

            result = await WeixinDesktopSyncAdapter().run(context)

            self.assertEqual(result.status, "completed")
            self.assertEqual(result.summary["synced_count"], 1)
            artifact_types = [artifact.artifact_type for artifact in collector.collected()]
            self.assertEqual(artifact_types.count("weixin_file"), 1)


if __name__ == "__main__":
    unittest.main()
