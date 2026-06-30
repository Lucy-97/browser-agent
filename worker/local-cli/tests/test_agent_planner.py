from __future__ import annotations

import unittest

from qiyuan_worker.agent.action_schema import validate_action_plan
from qiyuan_worker.agent.observer import PageObservation
from qiyuan_worker.agent.planner import BrowserAgentPlanner, PlannerError


OBSERVATION = PageObservation(
    url="https://example.com/search",
    title="Search",
    text="Search page",
    controls=(
        {"index": 0, "tag": "input", "name": "q", "selector": "#q"},
        {"index": 1, "tag": "button", "text": "Search", "selector": "#submit"},
    ),
)


class AgentPlannerTest(unittest.IsolatedAsyncioTestCase):
    async def test_planner_accepts_valid_json_action_plan(self) -> None:
        planner = BrowserAgentPlanner(
            lambda request: {
                "actions": [
                    {"action": "fill", "selector": "#q", "value": "LiFePO4"},
                    {"action": "click", "selector": "#submit"},
                    {"action": "extract", "selector": ".result"},
                ]
            }
        )

        actions = await planner.plan(
            OBSERVATION,
            task="search LiFePO4",
            policy={"allowed_actions": ["fill", "click", "extract"]},
        )

        self.assertEqual([item.action for item in actions], ["fill", "click", "extract"])
        self.assertEqual(actions[0].params["value"], "LiFePO4")

    async def test_planner_rejects_invalid_json(self) -> None:
        planner = BrowserAgentPlanner(lambda request: "{not json")

        with self.assertRaises(PlannerError) as ctx:
            await planner.plan(OBSERVATION, task="search", policy={})

        self.assertEqual(ctx.exception.code, "AGENT_PLAN_JSON_INVALID")

    async def test_planner_rejects_forbidden_action(self) -> None:
        planner = BrowserAgentPlanner(lambda request: {"actions": [{"action": "download", "url": "https://example.com/a.pdf"}]})

        with self.assertRaises(PlannerError) as ctx:
            await planner.plan(OBSERVATION, task="download", policy={"allowed_actions": ["extract"]})

        self.assertEqual(ctx.exception.code, "AGENT_ACTION_BLOCKED")

    async def test_planner_marks_high_risk_action_for_manual_action(self) -> None:
        planner = BrowserAgentPlanner(lambda request: {"actions": [{"action": "publish"}]})

        with self.assertRaises(PlannerError) as ctx:
            await planner.plan(OBSERVATION, task="publish", policy={"allowed_actions": ["publish"]})

        self.assertEqual(ctx.exception.code, "AGENT_MANUAL_ACTION_REQUIRED")

    def test_action_schema_rejects_download_outside_allowed_domain(self) -> None:
        with self.assertRaises(Exception) as ctx:
            validate_action_plan(
                {"actions": [{"action": "download", "url": "https://evil.example/a.pdf"}]},
                policy={"allowed_actions": ["download"], "allowed_domains": ["example.com"]},
            )

        self.assertEqual(getattr(ctx.exception, "code", ""), "AGENT_DOMAIN_BLOCKED")


if __name__ == "__main__":
    unittest.main()
