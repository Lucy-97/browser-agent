from __future__ import annotations

from qiyuan_worker.agent.policy import AgentPolicyError, allowed_actions_from_policy, action_timeout_seconds
from qiyuan_worker.protocols import AutomationJob


class PolicyViolation(RuntimeError):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


_BROWSER_JOB_PREFIXES = (
    "social.youtube.",
    "social.tiktok.",
    "social.instagram.",
    "generic.browser.",
    "generic.form.",
    "generic.file.",
)


def validate_job_policy(job: AutomationJob) -> None:
    if not job.job_type.startswith(_BROWSER_JOB_PREFIXES):
        return

    allowed_domains = job.target.get("allowed_domains") or job.policy.get("allowed_domains")
    if not allowed_domains:
        raise PolicyViolation(
            "POLICY_ALLOWED_DOMAINS_REQUIRED",
            f"allowed_domains is required for browser automation job {job.job_type}",
        )
    if not isinstance(allowed_domains, list) or not all(isinstance(item, str) for item in allowed_domains):
        raise PolicyViolation(
            "POLICY_ALLOWED_DOMAINS_INVALID",
            "allowed_domains must be a list of domain patterns",
        )
    try:
        allowed_actions_from_policy(job.policy)
        action_timeout_seconds(job.policy)
    except AgentPolicyError as exc:
        raise PolicyViolation(exc.code, exc.message) from exc
