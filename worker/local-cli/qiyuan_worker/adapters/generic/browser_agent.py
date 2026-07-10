from __future__ import annotations

import asyncio
import inspect
import os
from pathlib import Path
from typing import Any, Callable

from qiyuan_worker.adapters.base import AutomationAdapter
from qiyuan_worker.agent import (
    AgentPolicyError,
    AgentRunCancelled,
    BrowserActionExecutor,
    BrowserAgentExecutor,
    BrowserAgentPlanner,
    LLMProviderConfig,
    LLMProviderError,
    PlannerConfig,
    PlannerError,
    build_llm_provider,
    ensure_url_allowed,
)
from qiyuan_worker.agent.llm_provider import LLMProvider
from qiyuan_worker.agent.observer import observe_page
from qiyuan_worker.agent.redaction import redact_value
from qiyuan_worker.agent.trace import AgentTrace
from qiyuan_worker.browser import BrowserRuntime, BrowserRuntimeConfig
from qiyuan_worker.browser.runtime import BrowserRuntimeError
from qiyuan_worker.manifest import append_checkpoint
from qiyuan_worker.protocols import AdapterResult, ManualAction


class GenericBrowserAgentAdapter(AutomationAdapter):
    name = "generic.browser_agent"
    supported_job_types = ("generic.browser.agent",)
    required_capabilities = (
        "browser.playwright.chromium",
        "browser.profile.persistent",
        "adapter.generic.browser_agent",
    )

    def __init__(
        self,
        browser_runtime_factory: Callable[[BrowserRuntimeConfig], BrowserRuntime] | None = None,
        llm_provider_factory: Callable[[LLMProviderConfig], LLMProvider] | None = None,
    ):
        self.browser_runtime_factory = browser_runtime_factory or BrowserRuntime
        self.llm_provider_factory = llm_provider_factory or build_llm_provider
        self.executor = BrowserAgentExecutor()

    async def run(self, context) -> AdapterResult:
        start_url = str(context.job.input.get("url") or "").strip()
        query = str(context.job.input.get("query") or "").strip()
        task = str(context.job.input.get("task") or query).strip()
        mode = str(context.job.input.get("mode") or "deterministic_search").strip()
        if not start_url:
            return AdapterResult.failed("AGENT_URL_REQUIRED", "input.url is required", retryable=False)
        if mode != "llm_plan" and not query:
            return AdapterResult.failed("AGENT_QUERY_REQUIRED", "input.query is required", retryable=False)
        if mode == "llm_plan" and not task:
            return AdapterResult.failed("AGENT_TASK_REQUIRED", "input.task or input.query is required", retryable=False)

        allowed_domains = context.job.target.get("allowed_domains") or context.job.policy.get("allowed_domains") or []
        try:
            ensure_url_allowed(start_url, allowed_domains)
        except AgentPolicyError as exc:
            return AdapterResult.failed(exc.code, exc.message, retryable=False)
        except AgentRunCancelled as exc:
            return AdapterResult.failed(exc.code, exc.message, retryable=False)

        runtime = self.browser_runtime_factory(
            BrowserRuntimeConfig(
                profile_dir=context.config.secrets_dir / "browser-profiles" / "generic-agent",
                downloads_dir=context.config.data_dir / "downloads" / context.job.job_id,
                headed=bool(context.job.policy.get("headed", True)),
            )
        )
        try:
            async with await runtime.open_page() as page:
                await page.goto(start_url, wait_until="domcontentloaded", timeout=45000)
                trace = AgentTrace(context.work_dir / "agent")
                if mode == "llm_plan":
                    result = await self._run_llm_plan(context, page, trace, task, allowed_domains)
                else:
                    result = await self.executor.run_search(
                        page=page,
                        trace=trace,
                        query=query,
                        input_selector=_optional_string(context.job.input.get("input_selector")),
                        submit_selector=_optional_string(context.job.input.get("submit_selector")),
                        result_selector=_optional_string(context.job.input.get("result_selector")),
                        policy=context.job.policy,
                    )
                pause_seconds = float(context.job.policy.get("pause_after_seconds") or 0)
                if pause_seconds > 0:
                    await asyncio.sleep(min(pause_seconds, 60))
        except BrowserRuntimeError as exc:
            return AdapterResult.failed(exc.code, exc.message, retryable=False)
        except AgentPolicyError as exc:
            return AdapterResult.failed(exc.code, exc.message, retryable=False)
        except LLMProviderError as exc:
            return AdapterResult.failed(exc.code, exc.message, retryable=exc.retryable)
        except PlannerError as exc:
            if exc.code == "AGENT_MANUAL_ACTION_REQUIRED":
                return AdapterResult(
                    status="needs_manual_action",
                    summary={"reason": exc.message, "mode": mode},
                    manual_action=ManualAction(
                        action_type="agent_high_risk_action",
                        message=exc.message,
                        payload={"url": start_url, "task": task},
                    ),
                )
            return AdapterResult.failed(exc.code, exc.message, retryable=False)
        except Exception as exc:
            return AdapterResult.failed("AGENT_RUNTIME_ERROR", str(exc), retryable=True)

        trace_path = Path(result.trace_path)
        # 为了在 Web 面板可追溯每次执行的浏览器行为，agent_trace 在所有模式下都应上传。
        context.artifact_collector.add_file(
            "agent_trace",
            trace_path,
            metadata={"url": start_url, "query": query, "mode": mode},
        )
        if result.screenshot_path:
            context.artifact_collector.add_file("screenshot", Path(result.screenshot_path), metadata={"url": start_url, "query": query})
        return AdapterResult.completed(
            summary=result.summary,
            cursor={"source": "generic.browser_agent", "url": result.summary["url"]},
        )

    async def _run_llm_plan(self, context, page, trace: AgentTrace, task: str, allowed_domains: list[str]):
        agent_policy = {**context.job.policy, "allowed_domains": allowed_domains}
        trace_collected = False
        
        all_screenshots = []
        all_downloads = []
        all_extracts = []
        all_actions_history = []
        action_count = 0
        previous_actions = None
        previous_result = None
        consecutive_failures = 0
        stopped_reason = "max_iterations"
        last_error = None

        try:
            planner = BrowserAgentPlanner(
                self.llm_provider_factory(
                    LLMProviderConfig(
                        provider=context.config.llm_provider, 
                        model=context.config.llm_model,
                        timeout_seconds=float(os.environ.get("LLM_TIMEOUT_SECONDS", "120")),
                    )
                ),
                config=PlannerConfig(provider=context.config.llm_provider, model=context.config.llm_model),
            )
            
            executor = BrowserActionExecutor(
                downloads_dir=context.config.data_dir / "downloads" / context.job.job_id,
                checkpoint=lambda payload: self._checkpoint(context, payload),
                should_cancel=lambda: self._is_cancelled(context),
            )
            
            for iteration in range(15):
                observation = await observe_page(page)
                trace.add(f"observe.planner_input.{iteration}", observation.to_dict())
                
                plan_request = planner.build_request(
                    observation, 
                    task, 
                    agent_policy,
                    previous_actions=previous_actions,
                    previous_result=previous_result,
                )
                trace.add(f"planner.request.{iteration}", redact_value(plan_request))
                self._checkpoint(context, {"step": "planner.request", "iteration": iteration, "task": task})
                
                loop = asyncio.get_running_loop()
                raw_response = await loop.run_in_executor(None, planner._call_provider, plan_request)
                if inspect.isawaitable(raw_response):
                    raw_response = await raw_response
                trace.add(f"planner.response.{iteration}", {"response": redact_value(raw_response)})
                
                actions = planner.validate_response(raw_response, agent_policy)
                trace.add(f"planner.actions.{iteration}", {"actions": [action.to_dict() for action in actions]})
                self._checkpoint(context, {"step": "planner.actions", "iteration": iteration, "action_count": len(actions)})
                
                result = await executor.execute(page, trace, actions, agent_policy)
                action_count += len(actions)
                for a in actions:
                    desc = a.action
                    if "selector" in a.params:
                        desc += f" on '{a.params['selector']}'"
                    elif a.action == "fill" and "value" in a.params:
                        desc += f" with '{a.params['value']}'"
                    elif a.action == "stop" and "reason" in a.params:
                        desc += f" (reason: {a.params['reason']})"
                    elif a.action == "wait_for" and "condition" in a.params:
                        desc += f" (condition: {a.params['condition']})"
                    elif a.action == "press" and "key" in a.params:
                        desc += f" key '{a.params['key']}'"
                    all_actions_history.append(desc)

                all_screenshots.extend(result.screenshots)
                all_downloads.extend(result.downloads)
                if result.summary.get("extracts"):
                    all_extracts.extend(result.summary["extracts"])

                previous_actions = [action.to_dict() for action in actions]
                previous_result = result.summary

                if result.summary.get("error"):
                    consecutive_failures += 1
                    last_error = result.summary["error"]
                    if consecutive_failures >= 3:
                        stopped_reason = "repeated_failures"
                        trace.add(
                            "planner.stop",
                            {"iteration": iteration, "reason": stopped_reason, "error": last_error},
                        )
                        break
                else:
                    consecutive_failures = 0

                if any(action.action == "stop" for action in actions):
                    stopped_reason = "stop_action"
                    trace.add("planner.stop", {"iteration": iteration, "reason": "stop action received"})
                    break

            trace_path = trace.write()
            trace_collected = True
            
            last_observation = await observe_page(page)
            confirmed_findings: list[str] = []
            candidate_findings: list[str] = []
            for extract in all_extracts:
                if not isinstance(extract, dict):
                    continue
                fields = extract.get("fields")
                if not isinstance(fields, dict):
                    continue
                label = _optional_string(
                    fields.get("title")
                    or fields.get("name")
                    or fields.get("domain")
                    or fields.get("url")
                    or fields.get("text")
                )
                if not label:
                    continue
                piracy_found = str(fields.get("piracy_found", "")).strip().lower()
                if piracy_found in {"true", "1", "yes", "found"}:
                    confirmed_findings.append(label)
                else:
                    candidate_findings.append(label)

            summary = {
                "action_count": action_count,
                "actions_history": all_actions_history,
                "extract_count": len(all_extracts),
                "download_count": len(all_downloads),
                "screenshot_count": len(all_screenshots),
                "url": last_observation.url,
                "title": last_observation.title,
                "extracts": all_extracts[-5:],
                "mode": "llm_plan",
                "task": task,
                "stopped_reason": stopped_reason,
                "last_error": last_error,
                "trace_path": str(trace_path),
            }
            summary.update(
                _build_task_summary(
                    task=task,
                    stopped_reason=stopped_reason,
                    last_error=last_error,
                    extracts=all_extracts,
                    confirmed_findings=confirmed_findings,
                    candidate_findings=candidate_findings,
                )
            )
            for screenshot in all_screenshots:
                context.artifact_collector.add_file("screenshot", screenshot, metadata={"url": summary["url"], "task": task})
            for download in all_downloads:
                context.artifact_collector.add_file(
                    "download",
                    Path(download["path"]),
                    metadata={
                        "url": download["url"],
                        "sha256": download["sha256"],
                        "size_bytes": download["size_bytes"],
                        "task": task,
                    },
                )
                
            # Save final state screenshot
            final_screenshot_path = await trace.screenshot(page, "final_state")
            if final_screenshot_path:
                context.artifact_collector.add_file("screenshot", final_screenshot_path, metadata={"url": summary["url"], "task": task, "type": "final_state"})
            
            # Save extracts as a JSON artifact
            if all_extracts:
                import json
                extracts_path = trace.work_dir / "extracts.json"
                extracts_path.write_text(json.dumps(all_extracts, ensure_ascii=False, indent=2), encoding="utf-8")
                context.artifact_collector.add_file("extract", extracts_path, metadata={"url": summary["url"], "task": task})
                
            return _PlanExecutionResult(summary=summary, trace_path=str(trace_path), screenshot_path=None)
        except Exception:
            if not trace_collected:
                trace_path = trace.write()
                context.artifact_collector.add_file("agent_trace", trace_path, metadata={"task": task, "status": "failed"})
            raise

    def _checkpoint(self, context, payload: dict) -> None:
        checkpoint_payload = {
            "job_id": context.job.job_id,
            "run_id": context.job.run_id,
            "cursor": {"source": "generic.browser_agent", **payload},
            "summary": redact_value(payload),
            "status": "running",
        }
        run_checkpoint = getattr(context.api_client, "run_checkpoint", None)
        if callable(run_checkpoint):
            response = run_checkpoint(context.job.run_id, checkpoint_payload)
            append_checkpoint(context.config, context.job.job_id, {"request": checkpoint_payload, "response": response})

    def _is_cancelled(self, context) -> bool:
        run_status = getattr(context.api_client, "run_status", None)
        if not callable(run_status):
            return False
        try:
            run = run_status(context.job.run_id)
            return str(run.get("status") or "") == "cancelled"
        except Exception:
            return False


class _PlanExecutionResult:
    def __init__(self, summary: dict, trace_path: str, screenshot_path: str | None):
        self.summary = summary
        self.trace_path = trace_path
        self.screenshot_path = screenshot_path


def _build_task_summary(
    *,
    task: str,
    stopped_reason: str,
    last_error: str | None,
    extracts: list[dict],
    confirmed_findings: list[str],
    candidate_findings: list[str],
) -> dict[str, Any]:
    if _is_copyright_task(task):
        detected_findings = confirmed_findings or candidate_findings
        detected_count = len(detected_findings)
        if confirmed_findings:
            conclusion = f"本次共发现 {detected_count} 个命中项，其中 {len(confirmed_findings)} 个已确认疑似侵权。"
        elif candidate_findings:
            conclusion = f"本次共发现 {detected_count} 个候选命中项，尚未确认明确侵权。"
        elif stopped_reason == "repeated_failures" and last_error:
            conclusion = f"本次执行中断：{last_error}"
        elif stopped_reason == "stop_action":
            conclusion = "本次任务已按规划正常停止，未形成可确认的侵权结论。"
        elif stopped_reason == "policy_blocked":
            conclusion = "本次任务受站点限制或风控阻断，未能形成可确认的侵权结论。"
        else:
            conclusion = "本次未识别到明确侵权页面。"
        return {
            "detected": detected_count,
            "confirmed": len(confirmed_findings),
            "candidates": len(candidate_findings),
            "findings": detected_findings[:10],
            "conclusion": conclusion,
        }

    if stopped_reason == "repeated_failures" and last_error:
        conclusion = f"本次执行中断：{last_error}"
    elif stopped_reason == "policy_blocked":
        conclusion = "本次任务受站点限制或风控阻断，未能完成。"
    elif extracts:
        conclusion = "本次任务已完成，并生成结构化抽取结果。"
    elif stopped_reason == "stop_action":
        conclusion = "本次任务已按操作要求完成。"
    else:
        conclusion = "本次任务已结束。"
    return {"conclusion": conclusion}


def _is_copyright_task(task: str) -> bool:
    lowered = task.lower()
    return any(keyword in lowered for keyword in ("侵权", "版权", "取证", "copyright", "piracy"))


def _optional_string(value: object) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None
