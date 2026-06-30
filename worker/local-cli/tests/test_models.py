from pathlib import Path
import unittest

from qiyuan_worker.models import DeviceInfo, load_device, save_device


class ModelsTest(unittest.TestCase):
    def test_save_and_load_device(self) -> None:
        import tempfile

        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "device.json"
            device = DeviceInfo(
                device_id="wdev_123",
                name="test",
                platform="darwin-arm64",
                worker_version="0.1.0",
            )

            save_device(path, device)

            self.assertEqual(load_device(path), device)


if __name__ == "__main__":
    unittest.main()
