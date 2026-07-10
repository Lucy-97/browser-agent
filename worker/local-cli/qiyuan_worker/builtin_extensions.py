from __future__ import annotations

from qiyuan_worker.adapters.browser_act import BrowserActAdapter
from qiyuan_worker.adapters.generic.browser_agent import GenericBrowserAgentAdapter
from qiyuan_worker.adapters.manual.upload import ManualUploadAdapter
from qiyuan_worker.adapters.mock.echo import MockEchoAdapter
from qiyuan_worker.adapters.social.upload import SocialUploadAdapter
from qiyuan_worker.adapters.weixin.desktop_sync import WeixinDesktopSyncAdapter
from qiyuan_worker.sdk import PolicyTemplate, WorkerExtension


def builtin_worker_extensions() -> tuple[WorkerExtension, ...]:
    return (
        WorkerExtension(
            name="core.mock",
            product_line="core",
            adapters=(MockEchoAdapter(),),
            capabilities=(
                "adapter.mock.echo",
                "artifact.metadata",
                "automation.runtime",
            ),
            policy_templates=(
                PolicyTemplate(
                    name="core.mock.echo",
                    product_line="core",
                    job_type="mock.echo",
                    adapter="mock.echo",
                    target={},
                    policy={},
                ),
            ),
        ),
        WorkerExtension(
            name="browser_agent.generic",
            product_line="browser_agent",
            adapters=(GenericBrowserAgentAdapter(),),
            capabilities=("adapter.generic.browser_agent",),
            policy_templates=(
                PolicyTemplate(
                    name="browser_agent.generic.llm_plan",
                    product_line="browser_agent",
                    job_type="generic.browser.agent",
                    adapter="generic.browser_agent",
                    target={"allowed_domains": []},
                    policy={
                        "allowed_actions": [
                            "observe_page",
                            "click",
                            "click_element",
                            "fill",
                            "press",
                            "extract",
                            "screenshot",
                            "wait_for",
                        ],
                        "action_timeout_seconds": 30,
                    },
                ),
            ),
            requires_playwright=True,
        ),
        WorkerExtension(
            name="browser_agent.browser_act",
            product_line="browser_agent",
            adapters=(BrowserActAdapter(),),
            capabilities=("adapter.browser.act",),
            policy_templates=(
                PolicyTemplate(
                    name="browser_agent.browser_act.cli",
                    product_line="browser_agent",
                    job_type="generic.browser.act",
                    adapter="browser.act",
                    target={},
                    policy={},
                ),
            ),
        ),

        WorkerExtension(
            name="manual.upload",
            product_line="core",
            adapters=(ManualUploadAdapter(),),
            capabilities=("adapter.manual",),
            policy_templates=(
                PolicyTemplate(
                    name="core.manual.upload",
                    product_line="core",
                    job_type="qiyuan.manual_upload",
                    adapter="manual",
                    target={"source": "web_upload"},
                    policy={},
                ),
            ),
        ),
        WorkerExtension(
            name="social.upload",
            product_line="social",
            adapters=(
                SocialUploadAdapter("douyin"),
                SocialUploadAdapter("youtube"),
                SocialUploadAdapter("tiktok"),
                SocialUploadAdapter("instagram"),
            ),
            capabilities=(
                "adapter.social.douyin.upload_video",
                "adapter.social.youtube.upload_video",
                "adapter.social.tiktok.upload_video",
                "adapter.social.instagram.upload_video",
            ),
            policy_templates=(
                PolicyTemplate(
                    name="social.douyin.upload_video.publish",
                    product_line="social",
                    job_type="social.douyin.upload_video",
                    adapter="social.douyin.upload_video",
                    target={"allowed_domains": ["creator.douyin.com", "*.douyin.com"]},
                    policy={
                        "manual_publish_required": False,
                    },
                ),
                PolicyTemplate(
                    name="social.youtube.upload_video.publish",
                    product_line="social",
                    job_type="social.youtube.upload_video",
                    adapter="social.youtube.upload_video",
                    target={"allowed_domains": ["studio.youtube.com", "*.youtube.com"]},
                    policy={
                        "manual_publish_required": False,
                    },
                ),
                PolicyTemplate(
                    name="social.tiktok.upload_video.publish",
                    product_line="social",
                    job_type="social.tiktok.upload_video",
                    adapter="social.tiktok.upload_video",
                    target={"allowed_domains": ["www.tiktok.com", "*.tiktok.com"]},
                    policy={
                        "manual_publish_required": False,
                    },
                ),
                PolicyTemplate(
                    name="social.instagram.upload_video.publish",
                    product_line="social",
                    job_type="social.instagram.upload_video",
                    adapter="social.instagram.upload_video",
                    target={"allowed_domains": ["www.instagram.com", "*.instagram.com"]},
                    policy={
                        "manual_publish_required": False,
                    },
                ),
            ),
            requires_playwright=True,
        ),
        WorkerExtension(
            name="weixin.desktop_sync",
            product_line="weixin",
            adapters=(WeixinDesktopSyncAdapter(),),
            capabilities=("adapter.weixin.desktop_sync",),
            policy_templates=(
                PolicyTemplate(
                    name="weixin.desktop_sync.local_files",
                    product_line="weixin",
                    job_type="weixin.desktop_sync",
                    adapter="weixin.desktop_sync",
                    target={"source": "desktop_weixin"},
                    policy={
                        "max_files": 200,
                        "max_file_bytes": 104857600,
                    },
                ),
            ),
        ),
    )
