from __future__ import annotations

from dataclasses import asdict, dataclass
from typing import Any


@dataclass(frozen=True)
class PageObservation:
    url: str
    title: str
    html: str
    text: str
    controls: tuple[dict[str, Any], ...]

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


async def observe_page(page: Any) -> PageObservation:
    title = await page.title()
    html = ""
    if hasattr(page, "content"):
        try:
            html = await page.content()
        except Exception:
            html = ""
    text = await page.locator("body").inner_text(timeout=10000)
    controls = await page.evaluate(
        """() => {
          const SELECTOR = 'input, textarea, button, a, select';
          const collected = [];
          // Walk the DOM including open shadow roots so controls inside web components
          // (e.g. Reddit's <shreddit-search-bar>) become observable and targetable.
          const walk = (root) => {
            if (!root) return;
            for (const el of root.querySelectorAll(SELECTOR)) {
              collected.push(el);
            }
            for (const el of root.querySelectorAll('*')) {
              if (el.shadowRoot) walk(el.shadowRoot);
            }
          };
          walk(document);
          return collected
            .filter((el) => {
              const rect = el.getBoundingClientRect();
              const style = window.getComputedStyle(el);
              return rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none';
            })
            .slice(0, 80)
            .map((el, index) => {
              el.setAttribute('data-qiyuan-agent-index', String(index));
              const rect = el.getBoundingClientRect();
              const selector = el.id ? `#${CSS.escape(el.id)}` : `[data-qiyuan-agent-index="${index}"]`;
              return ({
              index,
              tag: el.tagName.toLowerCase(),
              type: el.getAttribute('type') || '',
              role: el.getAttribute('role') || '',
              name: el.getAttribute('aria-label') || el.getAttribute('name') || el.id || '',
              text: (el.innerText || el.value || el.getAttribute('placeholder') || '').trim().slice(0, 160),
              selector,
              visible: true,
              bounds: {
                x: Math.round(rect.x),
                y: Math.round(rect.y),
                width: Math.round(rect.width),
                height: Math.round(rect.height)
              }
            });
          });
        }"""
    )
    return PageObservation(
        url=str(getattr(page, "url", "")),
        title=str(title),
        html=str(html)[:20000],
        text=str(text)[:8000],
        controls=tuple(dict(item) for item in controls),
    )
