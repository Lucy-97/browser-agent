from __future__ import annotations

import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from qiyuan_worker.crypto import DevFileSecretStore, MacOSKeychainSecretStore, build_secret_store


class SecretStoreTests(unittest.TestCase):
    def test_explicit_file_secret_store_overrides_macos_keychain(self) -> None:
        with TemporaryDirectory() as tmpdir:
            with patch("qiyuan_worker.crypto.platform.system", return_value="Darwin"):
                with patch.dict(
                    "os.environ",
                    {"QIYUAN_WORKER_ALLOW_INSECURE_FILE_SECRETS": "1"},
                    clear=False,
                ):
                    store = build_secret_store(Path(tmpdir))

        self.assertIsInstance(store, DevFileSecretStore)

    def test_macos_uses_keychain_by_default(self) -> None:
        with TemporaryDirectory() as tmpdir:
            with patch("qiyuan_worker.crypto.platform.system", return_value="Darwin"):
                with patch.dict("os.environ", {}, clear=True):
                    store = build_secret_store(Path(tmpdir))

        self.assertIsInstance(store, MacOSKeychainSecretStore)


if __name__ == "__main__":
    unittest.main()
