from pathlib import Path
import unittest

from qiyuan_worker.config import load_config, write_default_config


class ConfigTest(unittest.TestCase):
    def test_write_and_load_config(self) -> None:
        import tempfile

        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config_path = tmp_path / "config.yaml"
            data_dir = tmp_path / "data"
            config = write_default_config(
                server="http://localhost:28080",
                config_path=config_path,
                data_dir=data_dir,
            )

            loaded = load_config(config_path)

            self.assertEqual(loaded.server, "http://localhost:28080")
            self.assertEqual(loaded.data_dir, data_dir)
            self.assertEqual(loaded.poll_interval_seconds, config.poll_interval_seconds)
            self.assertEqual(loaded.heartbeat_interval_seconds, config.heartbeat_interval_seconds)
            self.assertEqual(loaded.enabled_products, ("core", "browser_agent", "literature", "social"))
            self.assertEqual(loaded.llm_provider, "disabled")

    def test_enabled_products_can_be_overridden_by_env(self) -> None:
        import os
        import tempfile

        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            config_path = tmp_path / "config.yaml"
            write_default_config(
                server="http://localhost:28080",
                config_path=config_path,
                data_dir=tmp_path / "data",
            )
            previous = os.environ.get("QIYUAN_WORKER_ENABLED_PRODUCTS")
            os.environ["QIYUAN_WORKER_ENABLED_PRODUCTS"] = "core,social"
            try:
                loaded = load_config(config_path)
            finally:
                if previous is None:
                    os.environ.pop("QIYUAN_WORKER_ENABLED_PRODUCTS", None)
                else:
                    os.environ["QIYUAN_WORKER_ENABLED_PRODUCTS"] = previous

            self.assertEqual(loaded.enabled_products, ("core", "social"))


if __name__ == "__main__":
    unittest.main()
