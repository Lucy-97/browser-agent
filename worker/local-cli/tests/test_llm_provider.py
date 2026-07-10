from __future__ import annotations

import os
import unittest
from io import BytesIO
from unittest.mock import patch
from urllib.error import HTTPError

from qiyuan_worker.agent.llm_provider import LLMProviderConfig, LLMProviderError, build_llm_provider


class LLMProviderTest(unittest.TestCase):
    def test_disabled_provider_fails_with_config_error(self) -> None:
        provider = build_llm_provider(LLMProviderConfig(provider="disabled", model=""))

        with self.assertRaises(LLMProviderError) as ctx:
            provider.complete_json({"prompt": "", "task": "", "policy": {}, "observation": {}})

        self.assertEqual(ctx.exception.code, "AGENT_PROVIDER_CONFIG_INVALID")
        self.assertFalse(ctx.exception.retryable)

    def test_mock_static_provider_reads_json_from_env(self) -> None:
        with patch.dict(os.environ, {"LLM_MOCK_RESPONSE": '{"actions":[{"action":"observe_page"}]}'}, clear=False):
            provider = build_llm_provider(LLMProviderConfig(provider="mock_static", model=""))

            response = provider.complete_json({"prompt": "", "task": "", "policy": {}, "observation": {}})

        self.assertEqual(response["actions"][0]["action"], "observe_page")

    def test_extract_content_from_gemini_payload(self) -> None:
        from qiyuan_worker.agent.llm_provider import _extract_llm_content

        payload = '{"candidates":[{"content":{"parts":[{"text":"{\\"actions\\":[{\\"action\\":\\"stop\\"}]}"}]}}]}'
        content = _extract_llm_content(payload, "gemini")

        self.assertIn('"action":"stop"', content)

    def test_extract_content_from_openai_sse_payload(self) -> None:
        from qiyuan_worker.agent.llm_provider import _extract_llm_content

        payload = 'data: {"choices":[{"delta":{"content":"{\\"actions\\":[{\\"action\\":\\"observe_page\\"}]}"}}]}\n\ndata: [DONE]'
        content = _extract_llm_content(payload, "openai")

        self.assertIn('"action":"observe_page"', content)

    def test_empty_content_error_is_retryable(self) -> None:
        err = LLMProviderError("AGENT_PROVIDER_RESPONSE_INVALID", "provider response missing message content", retryable=True)

        self.assertEqual(err.code, "AGENT_PROVIDER_RESPONSE_INVALID")
        self.assertTrue(err.retryable)

    def test_format_http_error_includes_response_body(self) -> None:
        from qiyuan_worker.agent.llm_provider import _format_http_error

        err = HTTPError(
            url="https://example.test",
            code=400,
            msg="Bad Request",
            hdrs={},
            fp=BytesIO(b'{"error":{"message":"bad response_format"}}'),
        )

        message = _format_http_error(err)

        self.assertIn("HTTP Error 400", message)
        self.assertIn("bad response_format", message)


if __name__ == "__main__":
    unittest.main()
