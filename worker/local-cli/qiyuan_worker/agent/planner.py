from __future__ import annotations

import inspect
import json
from dataclasses import dataclass
from typing import Any, Callable, Protocol

from .action_schema import AgentAction, AgentActionSchemaError, validate_action_plan
from .observer import PageObservation
from .prompts import BROWSER_AGENT_PLANNER_PROMPT


PlannerProvider = Callable[[dict[str, Any]], Any]


class StructuredPlannerProvider(Protocol):
    def complete_json(self, request: dict[str, Any]) -> Any:
        raise NotImplementedError


@dataclass(frozen=True)
class PlannerConfig:
    provider: str = "disabled"
    model: str = ""


class PlannerError(RuntimeError):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


class BrowserAgentPlanner:
    def __init__(self, provider: PlannerProvider | StructuredPlannerProvider, config: PlannerConfig | None = None):
        self.provider = provider
        self.config = config or PlannerConfig()

    async def plan(
        self,
        observation: PageObservation,
        task: str,
        policy: dict[str, object],
    ) -> tuple[AgentAction, ...]:
        request = self.build_request(observation, task, policy)
        response = self._call_provider(request)
        if inspect.isawaitable(response):
            response = await response
        return self.validate_response(response, policy)

    def build_request(
        self,
        observation: PageObservation,
        task: str,
        policy: dict[str, object],
        previous_actions: list[dict[str, Any]] | None = None,
        previous_result: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        payload = {
            "task": task,
            "policy": policy,
            "observation": observation.to_dict(),
        }
        if previous_actions is not None:
            payload["previous_actions"] = previous_actions
        if previous_result is not None:
            payload["previous_result"] = previous_result
            
        return {
            "prompt": BROWSER_AGENT_PLANNER_PROMPT,
            "provider": self.config.provider,
            "model": self.config.model,
            "task": payload["task"],
            "policy": payload["policy"],
            "observation": payload["observation"],
            "previous_actions": payload.get("previous_actions"),
            "previous_result": payload.get("previous_result"),
        }

    def validate_response(self, response: Any, policy: dict[str, object]) -> tuple[AgentAction, ...]:
        payload = _decode_response(response)
        try:
            return validate_action_plan(payload, policy)
        except AgentActionSchemaError as exc:
            raise PlannerError(exc.code, exc.message) from exc

    def _call_provider(self, request: dict[str, Any]) -> Any:
        # Blocking call. Callers offload it to a thread (browser_agent runs it via
        # loop.run_in_executor); do not start another executor here or it would run in a
        # thread without a running event loop.
        complete_json = getattr(self.provider, "complete_json", None)
        if callable(complete_json):
            return complete_json(request)
        return self.provider(request)


def _decode_response(response: Any) -> Any:
    if isinstance(response, (dict, list)):
        return response
    if not isinstance(response, str):
        raise PlannerError("AGENT_PLAN_RESPONSE_INVALID", "planner response must be JSON text or structured data")
    try:
        return json.loads(response)
    except json.JSONDecodeError as exc:
        raise PlannerError("AGENT_PLAN_JSON_INVALID", f"planner returned invalid JSON: {exc.msg}") from exc
