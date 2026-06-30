from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from .policy import AgentPolicyError, ensure_action_allowed, ensure_url_allowed


HIGH_RISK_ACTIONS = {
    "delete",
    "external_upload",
    "final_submit",
    "grant_auth",
    "payment",
    "publish",
    "upload_file",
}


@dataclass(frozen=True)
class AgentAction:
    action: str
    params: dict[str, Any]

    def to_dict(self) -> dict[str, Any]:
        return {"action": self.action, **self.params}


class AgentActionSchemaError(RuntimeError):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


def validate_action_plan(payload: Any, policy: dict[str, object]) -> tuple[AgentAction, ...]:
    raw_actions = _extract_actions(payload)
    actions = tuple(_validate_action(item, policy) for item in raw_actions)
    if not actions:
        raise AgentActionSchemaError("AGENT_PLAN_EMPTY", "action plan must contain at least one action")
    return actions


def actions_to_payload(actions: tuple[AgentAction, ...]) -> list[dict[str, Any]]:
    return [action.to_dict() for action in actions]


def _extract_actions(payload: Any) -> list[Any]:
    if isinstance(payload, list):
        # Handle LLM returning a list wrapping an object with actions key:
        # [{"actions": [...]}]  →  unwrap to [...]
        if (
            len(payload) == 1
            and isinstance(payload[0], dict)
            and isinstance(payload[0].get("actions"), list)
        ):
            return list(payload[0]["actions"])
        return payload
    if isinstance(payload, dict) and isinstance(payload.get("actions"), list):
        return list(payload["actions"])
    raise AgentActionSchemaError("AGENT_PLAN_SCHEMA_INVALID", "action plan must be a list or object with actions list")


def _validate_action(item: Any, policy: dict[str, object]) -> AgentAction:
    if not isinstance(item, dict):
        raise AgentActionSchemaError("AGENT_ACTION_SCHEMA_INVALID", "each action must be an object")
    name = item.get("action")
    if not isinstance(name, str) or not name:
        raise AgentActionSchemaError("AGENT_ACTION_NAME_REQUIRED", "each action requires a string action")
    if name in HIGH_RISK_ACTIONS:
        raise AgentActionSchemaError("AGENT_MANUAL_ACTION_REQUIRED", f"high risk action {name} requires manual action")
    try:
        ensure_action_allowed(name, policy)
    except AgentPolicyError as exc:
        raise AgentActionSchemaError(exc.code, exc.message) from exc

    params = {key: value for key, value in item.items() if key != "action"}
    _validate_params(name, params, policy)
    return AgentAction(action=name, params=params)


def _validate_params(name: str, params: dict[str, Any], policy: dict[str, object]) -> None:
    if name == "observe_page":
        _reject_unknown(params, set())
        return
    if name == "click":
        _require_string(params, "selector")
        _reject_unknown(params, {"selector"})
        return
    if name == "click_element":
        _reject_unknown(params, {"index"})
        if not isinstance(params.get("index"), int):
            raise AgentActionSchemaError("AGENT_ACTION_PARAM_REQUIRED", "index must be an integer")
        return
    if name == "fill":
        _require_string(params, "selector")
        _require_string(params, "value")
        _reject_unknown(params, {"selector", "value"})
        return
    if name == "press":
        _require_string(params, "key")
        _reject_unknown(params, {"key"})
        return
    if name == "extract":
        _reject_unknown(params, {"selector", "instruction", "fields"})
        if "selector" in params:
            _require_string(params, "selector")
        if "instruction" in params:
            _require_string(params, "instruction")
        if "fields" in params and not isinstance(params["fields"], dict):
            raise AgentActionSchemaError("AGENT_ACTION_PARAM_INVALID", "fields must be an object")
        return
    if name == "download":
        _reject_unknown(params, {"selector", "url"})
        if "selector" not in params and "url" not in params:
            raise AgentActionSchemaError("AGENT_ACTION_PARAM_REQUIRED", "download requires selector or url")
        if "selector" in params:
            _require_string(params, "selector")
        if "url" in params:
            _require_string(params, "url")
            allowed_domains = policy.get("allowed_domains")
            if isinstance(allowed_domains, list) and all(isinstance(item, str) for item in allowed_domains):
                try:
                    ensure_url_allowed(params["url"], allowed_domains)
                except AgentPolicyError as exc:
                    raise AgentActionSchemaError(exc.code, exc.message) from exc
        return
    if name == "screenshot":
        _reject_unknown(params, {"name", "overlay"})
        if "name" in params:
            _require_string(params, "name")
        if "overlay" in params and not isinstance(params["overlay"], bool):
            raise AgentActionSchemaError("AGENT_ACTION_PARAM_INVALID", "overlay must be a boolean")
        return
    if name == "wait_for":
        _reject_unknown(params, {"condition", "timeout_ms"})
        if "condition" in params:
            _require_string(params, "condition")
        if "timeout_ms" in params and not isinstance(params["timeout_ms"], int):
            raise AgentActionSchemaError("AGENT_ACTION_PARAM_INVALID", "timeout_ms must be an integer")
        return
    if name == "stop":
        _require_string(params, "reason")
        _reject_unknown(params, {"reason"})
        return
    raise AgentActionSchemaError("AGENT_ACTION_UNSUPPORTED", f"action {name} is not supported")


def _require_string(params: dict[str, Any], key: str) -> None:
    if not isinstance(params.get(key), str) or not params[key].strip():
        raise AgentActionSchemaError("AGENT_ACTION_PARAM_REQUIRED", f"{key} must be a non-empty string")


def _reject_unknown(params: dict[str, Any], allowed: set[str]) -> None:
    unknown = sorted(set(params) - allowed)
    if unknown:
        raise AgentActionSchemaError("AGENT_ACTION_PARAM_UNKNOWN", f"unknown action params: {', '.join(unknown)}")
