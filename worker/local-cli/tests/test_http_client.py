from __future__ import annotations

import unittest
from io import BytesIO
from urllib.error import HTTPError

from qiyuan_worker.http_client import APIClient


class APIClientTest(unittest.TestCase):
    def test_plain_5xx_http_error_is_retryable(self) -> None:
        err = HTTPError(
            url="https://example.test",
            code=502,
            msg="Bad Gateway",
            hdrs={},
            fp=BytesIO(b"Bad Gateway"),
        )

        api_error = APIClient._api_error(err)

        self.assertEqual(api_error.code, "HTTP_502")
        self.assertTrue(api_error.retryable)


if __name__ == "__main__":
    unittest.main()
