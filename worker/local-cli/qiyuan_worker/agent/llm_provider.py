from __future__ import annotations

import json
import os
import sys
from dataclasses import dataclass
from typing import Any
from urllib.request import Request, urlopen


class LLMProviderError(RuntimeError):
    def __init__(self, code: str, message: str, retryable: bool = False):
        super().__init__(message)
        self.code = code
        self.message = message
        self.retryable = retryable


@dataclass(frozen=True)
class LLMProviderConfig:
    provider: str
    model: str
    timeout_seconds: float = 60.0


class LLMProvider:
    def complete_json(self, request: dict[str, Any]) -> Any:
        raise NotImplementedError


class DisabledLLMProvider(LLMProvider):
    def complete_json(self, request: dict[str, Any]) -> Any:
        raise LLMProviderError(
            "AGENT_PROVIDER_CONFIG_INVALID",
            "llm_provider is disabled; set llm_provider and credentials before using llm_plan mode",
            retryable=False,
        )


class MockStaticLLMProvider(LLMProvider):
    def complete_json(self, request: dict[str, Any]) -> Any:
        raw = os.environ.get("LLM_MOCK_RESPONSE")
        if not raw:
            raise LLMProviderError(
                "AGENT_PROVIDER_CONFIG_INVALID",
                "LLM_MOCK_RESPONSE is required for mock_static provider",
                retryable=False,
            )
        return json.loads(raw)


class OpenAICompatibleLLMProvider(LLMProvider):
    _PROVIDER_DEFAULT_BASE_URL: dict[str, str] = {
        "gemini": "https://generativelanguage.googleapis.com/v1beta/openai",
    }

    def __init__(self, config: LLMProviderConfig):
        self.config = config

    def _effective_base_url(self) -> str:
        """返回实际使用的 base_url，gemini 自动映射。"""
        provider = self.config.provider.strip().lower()
        base_url = os.environ.get("LLM_BASE_URL", "https://api.openai.com/v1").rstrip("/")
        default = self._PROVIDER_DEFAULT_BASE_URL.get(provider, "")
        if provider == "gemini" and (not base_url or "openai.com" in base_url):
            return default
        return base_url

    def complete_json(self, request: dict[str, Any]) -> str:
        api_key = os.environ.get("LLM_API_KEY")
        base_url = self._effective_base_url()
        provider = self.config.provider.strip().lower()
        if not api_key:
            raise LLMProviderError("AGENT_PROVIDER_CONFIG_INVALID", "LLM_API_KEY is required", retryable=False)
        if not self.config.model:
            raise LLMProviderError("AGENT_PROVIDER_CONFIG_INVALID", "llm_model is required", retryable=False)

        body = json.dumps(
            {
                "model": self.config.model,
                "messages": [
                    {"role": "system", "content": request["prompt"]},
                    {
                        "role": "user",
                        "content": json.dumps(
                            {
                                k: request[k]
                                for k in ["task", "policy", "observation", "previous_actions", "previous_result"]
                                if k in request and request[k] is not None
                            },
                            ensure_ascii=False,
                        ),
                    },
                ],
                "temperature": 0,
                "response_format": {"type": "json_object"},
                "stream": provider != "gemini",
            }
        ).encode("utf-8")
        http_request = Request(
            f"{base_url}/chat/completions",
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {api_key}",
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Connection": "close",
            },
        )
        import time
        import urllib.error
        import urllib.request
        import socket
        import ssl
        import http.client

        ssl_context = ssl.create_default_context()
        if hasattr(ssl, "OP_IGNORE_UNEXPECTED_EOF"):
            ssl_context.options |= getattr(ssl, "OP_IGNORE_UNEXPECTED_EOF")

        max_retries = 5
        for attempt in range(max_retries):
            try:
                with urlopen(http_request, timeout=self.config.timeout_seconds, context=ssl_context) as response:
                    payload = response.read()
                final_text = _extract_llm_content(payload.decode("utf-8"), provider)
                if not final_text:
                    raise LLMProviderError(
                        "AGENT_PROVIDER_RESPONSE_INVALID",
                        "provider response missing message content",
                        retryable=True,
                    )
                return final_text
            except (urllib.error.URLError, socket.error, TimeoutError, http.client.IncompleteRead, ssl.SSLError) as exc:
                print(f"LLM Provider Transient Error (attempt {attempt+1}/{max_retries}): {exc}", file=sys.stderr)
                if attempt == max_retries - 1:
                    raise LLMProviderError("AGENT_PROVIDER_ERROR", str(exc), retryable=True) from exc
                time.sleep(2 ** attempt)
            except LLMProviderError as exc:
                if exc.code == "AGENT_PROVIDER_RESPONSE_INVALID" and exc.retryable and attempt < max_retries - 1:
                    print(
                        f"LLM Provider Empty Content Retry (attempt {attempt+1}/{max_retries}): {exc.message}",
                        file=sys.stderr,
                    )
                    time.sleep(2 ** attempt)
                    continue
                raise
            except Exception as exc:
                print(f"LLM Provider Fatal Error: {exc}", file=sys.stderr)
                raise LLMProviderError("AGENT_PROVIDER_ERROR", str(exc), retryable=True) from exc


def _extract_llm_content(raw_response: str, provider: str) -> str:
    text = raw_response.strip()
    if not text:
        return ""
    if "data: " in text and "[DONE]" in text:
        content = _extract_content_from_sse(text)
        if content:
            return content
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        if provider == "gemini":
            return _extract_content_from_sse(text)
        return ""
    content = _extract_content_from_payload(payload)
    if content:
        return content
    if provider == "gemini":
        return _extract_content_from_gemini(payload)
    return ""


def _extract_content_from_sse(text: str) -> str:
    parts: list[str] = []
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("data: "):
            continue
        data_str = line[6:]
        if data_str == "[DONE]":
            break
        try:
            chunk = json.loads(data_str)
        except json.JSONDecodeError:
            continue
        content = _extract_content_from_payload(chunk)
        if content:
            parts.append(content)
    return "".join(parts)


def _extract_content_from_payload(payload: Any) -> str:
    if not isinstance(payload, dict):
        return ""
    choices = payload.get("choices")
    if isinstance(choices, list):
        for choice in choices:
            if not isinstance(choice, dict):
                continue
            delta = choice.get("delta")
            if isinstance(delta, dict):
                content = delta.get("content")
                if isinstance(content, str) and content:
                    return content
            message = choice.get("message")
            if isinstance(message, dict):
                content = message.get("content")
                if isinstance(content, str) and content:
                    return content
    return ""


def _extract_content_from_gemini(payload: Any) -> str:
    if not isinstance(payload, dict):
        return ""
    candidates = payload.get("candidates")
    if not isinstance(candidates, list):
        return ""
    for candidate in candidates:
        if not isinstance(candidate, dict):
            continue
        content = candidate.get("content")
        if isinstance(content, dict):
            parts = content.get("parts")
            if isinstance(parts, list):
                texts = []
                for part in parts:
                    if isinstance(part, dict):
                        text = part.get("text")
                        if isinstance(text, str) and text:
                            texts.append(text)
                if texts:
                    return "".join(texts)
    return ""


def build_llm_provider(config: LLMProviderConfig) -> LLMProvider:
    provider = config.provider.strip().lower()
    if provider in {"", "disabled", "none"}:
        return DisabledLLMProvider()
    if provider == "mock_static":
        return MockStaticLLMProvider()
    if provider in {"openai", "openai_compatible", "gemini"}:
        return OpenAICompatibleLLMProvider(config)
    raise LLMProviderError("AGENT_PROVIDER_CONFIG_INVALID", f"unsupported llm_provider {config.provider}", retryable=False)
