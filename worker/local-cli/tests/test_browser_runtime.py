from pathlib import Path
import tempfile
import unittest

from qiyuan_worker.browser import BrowserRuntime, BrowserRuntimeConfig


class BrowserRuntimeTest(unittest.TestCase):
    def test_doctor_prepares_profile_and_download_dirs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            runtime = BrowserRuntime(
                BrowserRuntimeConfig(
                    profile_dir=tmp_path / "profile",
                    downloads_dir=tmp_path / "downloads",
                )
            )

            result = runtime.doctor()

            self.assertTrue(result.profile_dir_ready)
            self.assertTrue(result.downloads_dir_ready)
            self.assertTrue((tmp_path / "profile").is_dir())
            self.assertTrue((tmp_path / "downloads").is_dir())


if __name__ == "__main__":
    unittest.main()
