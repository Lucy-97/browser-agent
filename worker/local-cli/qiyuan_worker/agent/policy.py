from __future__ import annotations

from urllib.parse import urlparse


DEFAULT_ALLOWED_ACTIONS = (
    "observe_page",
    "fill",
    "click",
    "click_element",
    "press",
    "extract",
    "screenshot",
    "wait_for",
)

# Control actions are not browser interactions and are never policy-gated. The planner
# is required to emit `stop` to end the loop, so blocking it would make every completed
# task fail with AGENT_ACTION_BLOCKED.
CONTROL_ACTIONS = frozenset({"stop"})


class AgentPolicyError(RuntimeError):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


def ensure_url_allowed(url: str, allowed_domains: list[str]) -> None:
    parsed = urlparse(url)
    if parsed.scheme == "data" and "data:" in allowed_domains:
        return
    if parsed.scheme == "file" and "file:" in allowed_domains:
        return
    host = parsed.hostname or ""
    for pattern in allowed_domains:
        if pattern == "*":
            return
        if pattern.startswith("*.") and host.endswith(pattern[1:]):
            return
        if host == pattern:
            return
    raise AgentPolicyError("AGENT_DOMAIN_BLOCKED", f"url {url} is not allowed by policy")


def allowed_actions_from_policy(policy: dict[str, object]) -> set[str]:
    raw = policy.get("allowed_actions")
    if raw is None:
        return set(DEFAULT_ALLOWED_ACTIONS)
    if not isinstance(raw, list) or not all(isinstance(item, str) for item in raw):
        raise AgentPolicyError("AGENT_ALLOWED_ACTIONS_INVALID", "allowed_actions must be a list of action names")
    return {item for item in raw if item}


def ensure_action_allowed(action: str, policy: dict[str, object]) -> None:
    if action in CONTROL_ACTIONS:
        return
    allowed_actions = allowed_actions_from_policy(policy)
    if action not in allowed_actions:
        raise AgentPolicyError("AGENT_ACTION_BLOCKED", f"action {action} is not allowed by policy")


def action_timeout_seconds(policy: dict[str, object]) -> float:
    raw = policy.get("action_timeout_seconds")
    if raw is None:
        return 30.0
    try:
        timeout = float(raw)
    except (TypeError, ValueError) as exc:
        raise AgentPolicyError("AGENT_ACTION_TIMEOUT_INVALID", "action_timeout_seconds must be numeric") from exc
    if timeout <= 0 or timeout > 120:
        raise AgentPolicyError("AGENT_ACTION_TIMEOUT_INVALID", "action_timeout_seconds must be between 0 and 120")
    return timeout
