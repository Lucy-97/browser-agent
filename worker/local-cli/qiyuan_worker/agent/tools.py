from __future__ import annotations

import hashlib
from pathlib import Path
from typing import Any
from urllib.parse import urlparse
from urllib.request import Request, urlopen


# Per-action locator wait used by click/fill. Kept well below the policy-level
# action_timeout_seconds hard cap so a bad selector fails fast and the agent loop
# has budget left to recover with an alternative selector.
ACTION_LOCATOR_TIMEOUT_MS = 15000


async def fill_search(page: Any, query: str, input_selector: str | None = None) -> str:
    selector = input_selector or await first_search_input_selector(page)
    if not selector:
        raise RuntimeError("search input not found")
    await page.fill(selector, query)
    return selector


async def submit_search(page: Any, submit_selector: str | None = None) -> str:
    if submit_selector:
        await page.click(submit_selector)
        return submit_selector
    await page.keyboard.press("Enter")
    return "keyboard:Enter"


async def extract_results(page: Any, result_selector: str | None = None) -> list[str]:
    if result_selector:
        values = await page.locator(result_selector).all_inner_texts()
    else:
        values = await page.locator("body").inner_text(timeout=10000)
        values = values.splitlines()
    return [line.strip() for line in values if line.strip()][:50]


async def click_selector(page: Any, selector: str, timeout_ms: int = ACTION_LOCATOR_TIMEOUT_MS) -> dict[str, Any]:
    await page.click(selector, timeout=timeout_ms)
    return {"selector": selector}


async def click_element(page: Any, index: int, timeout_ms: int = ACTION_LOCATOR_TIMEOUT_MS) -> dict[str, Any]:
    selector = f'[data-qiyuan-agent-index="{index}"]'
    await page.click(selector, timeout=timeout_ms)
    return {"index": index, "selector": selector}


async def fill_selector(page: Any, selector: str, value: str, timeout_ms: int = ACTION_LOCATOR_TIMEOUT_MS) -> dict[str, Any]:
    await page.fill(selector, value, timeout=timeout_ms)
    return {"selector": selector, "value_length": len(value)}


async def press_key(page: Any, key: str) -> dict[str, Any]:
    await page.keyboard.press(key)
    return {"key": key}


async def extract_text(page: Any, selector: str | None = None) -> dict[str, Any]:
    lines = await extract_results(page, result_selector=selector)
    return {"selector": selector, "count": len(lines), "items": lines}


async def wait_for_condition(page: Any, condition: str | None = None, timeout_ms: int | None = None) -> dict[str, Any]:
    timeout = timeout_ms or 5000
    if condition == "networkidle" and hasattr(page, "wait_for_load_state"):
        await page.wait_for_load_state("networkidle", timeout=timeout)
        return {"condition": condition, "timeout_ms": timeout}
    if condition == "domcontentloaded" and hasattr(page, "wait_for_load_state"):
        await page.wait_for_load_state("domcontentloaded", timeout=timeout)
        return {"condition": condition, "timeout_ms": timeout}
    if hasattr(page, "wait_for_timeout"):
        await page.wait_for_timeout(min(timeout, 10000))
    return {"condition": condition or "timeout", "timeout_ms": timeout}


async def screenshot_page(page: Any, destination: Path, overlay: bool = False) -> dict[str, Any]:
    if overlay:
        await _install_overlay(page)
    try:
        await page.screenshot(path=str(destination), full_page=True)
    finally:
        if overlay:
            await _remove_overlay(page)
    return {"path": str(destination), "overlay": overlay}


async def download_file(
    page: Any,
    downloads_dir: Path,
    selector: str | None = None,
    url: str | None = None,
    max_bytes: int = 25 * 1024 * 1024,
) -> dict[str, Any]:
    resolved_url = url or await _href_for_selector(page, selector)
    if not resolved_url:
        raise RuntimeError("download requires selector with href or url")
    downloads_dir.mkdir(parents=True, exist_ok=True)
    filename = _safe_download_name(resolved_url)
    destination = downloads_dir / filename
    body = await _fetch_download(page, resolved_url, max_bytes=max_bytes)
    if len(body) > max_bytes:
        raise RuntimeError("download exceeds max_bytes")
    if not _looks_allowed_download(body, resolved_url):
        raise RuntimeError("download type is not allowed")
    destination.write_bytes(body)
    digest = hashlib.sha256(body).hexdigest()
    return {
        "url": resolved_url,
        "path": str(destination),
        "sha256": digest,
        "size_bytes": len(body),
    }


async def first_search_input_selector(page: Any) -> str:
    selector = await page.evaluate(
        """() => {
          const candidates = Array.from(document.querySelectorAll('input, textarea'));
          const preferred = candidates.find((el) => {
            const type = (el.getAttribute('type') || 'text').toLowerCase();
            const name = `${el.id || ''} ${el.getAttribute('name') || ''} ${el.getAttribute('placeholder') || ''}`.toLowerCase();
            return ['search', 'text', ''].includes(type) && (name.includes('search') || name.includes('query') || name.includes('q'));
          }) || candidates.find((el) => ['search', 'text', ''].includes((el.getAttribute('type') || 'text').toLowerCase()));
          if (!preferred) return '';
          if (preferred.id) return `#${CSS.escape(preferred.id)}`;
          preferred.setAttribute('data-qiyuan-agent-input', '1');
          return '[data-qiyuan-agent-input="1"]';
        }"""
    )
    return str(selector or "")


async def _href_for_selector(page: Any, selector: str | None) -> str | None:
    if not selector:
        return None
    locator = page.locator(selector)
    get_attribute = getattr(locator, "get_attribute", None)
    if callable(get_attribute):
        return await get_attribute("href")
    return await page.evaluate(
        """(selector) => {
          const el = document.querySelector(selector);
          return el ? (el.href || el.getAttribute('href') || '') : '';
        }""",
        selector,
    )


async def _fetch_download(page: Any, url: str, max_bytes: int) -> bytes:
    if hasattr(page, "goto"):
        try:
            response = await page.goto(url, wait_until="domcontentloaded", timeout=45000)
            if response and hasattr(response, "body"):
                body = await response.body()
                if body:
                    return bytes(body[: max_bytes + 1])
        except Exception:
            pass
    request = Request(url, headers={"User-Agent": "qiyuan-worker/0.1"})
    with urlopen(request, timeout=30) as response:
        return response.read(max_bytes + 1)


def _safe_download_name(url: str) -> str:
    path = urlparse(url).path
    name = Path(path).name or "download.bin"
    safe = "".join(ch for ch in name if ch.isalnum() or ch in {"-", "_", "."}).strip(".")
    return safe or "download.bin"


def _looks_allowed_download(body: bytes, url: str) -> bool:
    lowered = url.lower()
    if lowered.endswith(".pdf") and body.startswith(b"%PDF"):
        return True
    return False


async def _install_overlay(page: Any) -> None:
    if not hasattr(page, "evaluate"):
        return
    await page.evaluate(
        """() => {
          const old = document.getElementById('qiyuan-agent-overlay-root');
          if (old) old.remove();
          const root = document.createElement('div');
          root.id = 'qiyuan-agent-overlay-root';
          root.style.position = 'fixed';
          root.style.left = '0';
          root.style.top = '0';
          root.style.width = '100%';
          root.style.height = '100%';
          root.style.pointerEvents = 'none';
          root.style.zIndex = '2147483647';
          document.body.appendChild(root);
          document.querySelectorAll('[data-qiyuan-agent-index]').forEach((el) => {
            const rect = el.getBoundingClientRect();
            const label = document.createElement('div');
            label.textContent = el.getAttribute('data-qiyuan-agent-index') || '';
            label.style.position = 'fixed';
            label.style.left = `${Math.max(0, rect.left)}px`;
            label.style.top = `${Math.max(0, rect.top)}px`;
            label.style.background = '#ffcc00';
            label.style.color = '#111';
            label.style.border = '1px solid #111';
            label.style.font = '12px sans-serif';
            label.style.padding = '1px 4px';
            root.appendChild(label);
          });
        }"""
    )


async def _remove_overlay(page: Any) -> None:
    if not hasattr(page, "evaluate"):
        return
    await page.evaluate("""() => document.getElementById('qiyuan-agent-overlay-root')?.remove()""")
