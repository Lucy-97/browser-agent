# Browser Agent 通用本地 Automation Worker API 与数据模型设计

> **状态说明（2026-07-19）**
>
> 仓库现为单一 Browser Agent 项目，`main` 是唯一长期主线。本文保留早期 API 与数据模型演进记录，其中旧产品和文献业务模型仅作历史兼容参考；新增接口以当前代码和 [BRD](../brd/0627-browser-agent-automation-brd.md) 为准。



## Changelog

- 2026-07-19：同步生产化 Phase 1 数据与 API：增加 `tenant_id` ownership、严格配对批准、租户设备管理接口、可信 Gateway actor 和跨租户拒绝规则。
- 2026-07-19：同步单一 Browser Agent 项目现状，移除已废弃的双分支解耦说明。
- 2026-06-17：新增通用 Automation Worker API 与数据模型设计。将旧版文献场景 `crawl_*` 模型升级为 `automation_*` 通用模型，覆盖设备绑定、任务下发、能力声明、策略约束、checkpoint、artifact、人机协同、审计和 QIYUAN/YouTube/TikTok adapter 兼容关系。

> 上游架构：`docs/tech/0617-local-automation-worker-architecture.md`
>
> 旧版兼容：`docs/tech/0614-local-worker-api-schema.md` 中的 `crawl_job`、`crawl_run`、`crawl_artifact` 仍可作为 QIYUAN 文献 P1 的业务视图或兼容层，但新平台代码优先按本文的 `automation_*` 模型实现。

## 一、设计目标

本文定义通用 Local Automation Worker 与平台 go-api 的接口和数据模型。目标是用同一套协议支持：

1. QIYUAN 版权检测、产物采集和元数据采集。
2. YouTube Studio 视频上传、草稿保存、发布前人工确认。
3. TikTok 视频上传、caption/hashtag 填写、草稿或发布。
4. 企业后台表单填写、文件上传、报表下载等通用浏览器自动化。
5. 后续 Browser Agent 自然语言任务。

设计要求：

1. Worker 只主动通过 HTTPS 连接平台，不要求平台反向访问用户机器。
2. 所有平台侧写入由 `backend-api` 完成。
3. 第三方网站账号密码默认不进入平台。
4. 发布、删除、授权、付款等高风险动作必须支持人工确认。
5. 任务、run、checkpoint、artifact、审计必须幂等可重试。
6. 平台可按设备能力和 adapter 能力分配任务。

## 二、命名与兼容策略

### 1. 新主线命名

新代码和表名优先使用：

```text
automation_job
automation_run
automation_checkpoint
automation_artifact
automation_session
automation_audit_event
automation_policy
```

### 2. 旧文献模型兼容

旧文档中的：

```text
crawl_job
crawl_run
crawl_checkpoint
crawl_result
crawl_artifact
```

处理方式：

1. P1 如果已经按 `crawl_*` 实现，可保留为 QIYUAN 文献兼容层。
2. 新增平台能力不再扩大 `crawl_*` 命名。
3. 文献任务在通用模型中使用 `job_type=generic.browser.agent`、`adapter=generic.browser.agent`。
4. 文献结果可落单独业务表 `qiyuan_literature_result`，并通过 `automation_artifact` 关联 PDF/HTML/截图。

## 三、核心枚举

### 1. Job Type

```text
generic.browser.agent
qiyuan.literature.download_pdf
qiyuan.literature.collect_metadata
social.youtube.upload_video
social.tiktok.upload_video
generic.browser.script
generic.browser.agent
generic.form.fill
generic.file.download
generic.file.upload
```

### 2. Adapter

```text
generic.browser.agent
qiyuan.publisher_page
social.youtube_studio
social.tiktok_web
generic.playwright_script
generic.browser_agent
```

### 3. Job Status

```text
queued
assigned
running
needs_manual_action
waiting_for_confirmation
uploading
completed
failed_retryable
failed_terminal
cancelled
expired
```

### 4. Run Status

```text
running
needs_manual_action
waiting_for_confirmation
uploading
completed
failed
cancelled
expired
```

### 5. Manual Action Type

```text
login_required
captcha_required
mfa_required
publish_confirmation
file_selection_required
policy_blocked
user_review_required
```

### 6. Artifact Type

```text
pdf
html
markdown
screenshot
mhtml
trace
har
console_log
network_log
metadata_json
input_file
uploaded_file
downloaded_file
result_snapshot
run_report
```

### 7. Worker Capability

```text
browser.playwright.chromium
browser.cdp.connect
browser.profile.persistent
browser.agent.assist
artifact.upload.multipart
artifact.download.input
adapter.generic.browser.agent
adapter.social.youtube_studio
adapter.social.tiktok_web
human.manual_action
human.confirmation
```

## 四、平台数据模型

以下为逻辑模型。实际 migration 需要按 MySQL 方言定义字段类型、索引和幂等约束，并同步 `database/init.sql`。

### 1. `worker_device`

记录用户本地 Worker 设备。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 设备 ID |
| `tenant_id` | string | 强制资源归属租户 |
| `user_id` | string | 绑定用户 |
| `name` | string | 设备显示名 |
| `platform` | string | `darwin-arm64`、`windows-amd64`、`linux-amd64` |
| `worker_version` | string | Worker 版本 |
| `hostname_hash` | string | hostname 哈希 |
| `status` | string | `active|revoked|offline` |
| `capabilities_json` | json | 最近一次能力上报 |
| `last_seen_at` | datetime | 最近心跳 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `revoked_at` | datetime | 撤销时间 |

索引：

1. `idx_worker_device_tenant_status(tenant_id, status)`
2. `idx_worker_device_user_status(user_id, status)`
3. `idx_worker_device_last_seen(last_seen_at)`

### 2. `worker_pairing`

记录一次性设备配对。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 配对 ID |
| `tenant_id` | string | 批准配对时写入的租户；pending 时为空 |
| `pairing_code_hash` | string | 配对码哈希 |
| `status` | string | `pending|approved|expired|rejected` |
| `device_id` | string | 批准后设备 ID |
| `requested_platform` | string | Worker 上报平台 |
| `requested_worker_version` | string | Worker 版本 |
| `requested_hostname_hash` | string | hostname 哈希 |
| `requested_capabilities_json` | json | 初始能力声明 |
| `approved_by_user_id` | string | 批准用户 |
| `expires_at` | datetime | 过期时间 |
| `created_at` | datetime | 创建时间 |
| `approved_at` | datetime | 批准时间 |

约束：

1. 明文 pairing code 只在创建响应中返回一次。
2. `pending` 过期后不可批准。
3. 同一 pairing 只能批准一次。

### 3. `automation_job`

平台下发给 Worker 的通用自动化任务。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | job ID |
| `tenant_id` | string | 强制资源归属租户 |
| `user_id` | string | 任务归属用户 |
| `job_type` | string | 任务类型 |
| `adapter` | string | adapter 名称 |
| `title` | string | 展示标题 |
| `status` | string | job 状态 |
| `priority` | int | 优先级 |
| `assigned_device_id` | string | 当前分配设备 |
| `required_capabilities_json` | json | 需要的 Worker capability |
| `target_json` | json | 目标站点、入口 URL、域名白名单 |
| `input_json` | json | 任务输入 |
| `policy_json` | json | 策略约束 |
| `last_cursor_json` | json | 最近游标 |
| `last_error_code` | string | 最近错误码 |
| `last_error_message` | string | 最近错误摘要 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `assigned_at` | datetime | 分配时间 |
| `started_at` | datetime | 开始时间 |
| `completed_at` | datetime | 完成时间 |

索引：

1. `idx_automation_job_tenant_status(tenant_id, status)`
2. `idx_automation_job_user_status(user_id, status)`
3. `idx_automation_job_type_status(job_type, status)`
4. `idx_automation_job_assigned_device(assigned_device_id, status)`
5. `idx_automation_job_priority(status, priority, created_at)`

### 4. `automation_run`

一次 job 执行实例。重试、重新分配、人工复跑会产生新 run。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | run ID |
| `job_id` | string | job ID |
| `tenant_id` | string | 从 job 冗余的强制资源归属租户 |
| `user_id` | string | 冗余归属用户 |
| `device_id` | string | 执行设备 |
| `adapter` | string | 执行 adapter |
| `status` | string | run 状态 |
| `worker_version` | string | 执行 Worker 版本 |
| `capabilities_json` | json | 执行时能力 |
| `started_at` | datetime | 开始时间 |
| `last_heartbeat_at` | datetime | 最近心跳 |
| `ended_at` | datetime | 结束时间 |
| `summary_json` | json | 结果统计 |
| `error_code` | string | 错误码 |
| `error_message` | string | 错误摘要 |

约束：

1. 同一 job 同一时间只有一个 active run。
2. active run 心跳超时后标记 `expired`，job 可重新排队。

### 5. `automation_checkpoint`

记录 Worker 阶段性进度。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | checkpoint ID |
| `job_id` | string | job ID |
| `run_id` | string | run ID |
| `tenant_id` | string | 从 run 冗余的强制资源归属租户 |
| `sequence` | int | run 内递增序号 |
| `stage` | string | 阶段名 |
| `cursor_json` | json | 断点游标 |
| `progress_json` | json | 进度数据 |
| `result_json` | json | 阶段结果 |
| `created_at` | datetime | 创建时间 |

唯一键：

1. `uniq_automation_checkpoint_run_sequence(run_id, sequence)`
2. 可选 `uniq_automation_checkpoint_cursor(run_id, stage, cursor_hash)`

### 6. `automation_artifact`

记录输入、输出和诊断产物。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | artifact ID |
| `job_id` | string | job ID |
| `run_id` | string | run ID |
| `tenant_id` | string | 从 run 冗余的强制资源归属租户 |
| `result_id` | string | 业务结果 ID，可为空 |
| `artifact_type` | string | artifact 类型 |
| `storage_key` | string | 对象存储 key 或本地存储引用 |
| `filename` | string | 原始文件名 |
| `content_type` | string | MIME |
| `size_bytes` | int64 | 大小 |
| `sha256` | string | 校验 |
| `metadata_json` | json | 额外信息 |
| `redaction_status` | string | `not_required|redacted|blocked` |
| `created_at` | datetime | 创建时间 |

唯一键：

1. `uniq_automation_artifact_idempotency(job_id, run_id, artifact_type, sha256)`

### 7. `automation_manual_action`

记录需要用户本机处理的人工动作。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | manual action ID |
| `job_id` | string | job ID |
| `run_id` | string | run ID |
| `tenant_id` | string | 从 run 冗余的强制资源归属租户 |
| `type` | string | manual action type |
| `status` | string | `pending|resolved|cancelled|expired` |
| `prompt` | string | 给用户展示的说明 |
| `details_json` | json | 页面 URL、截图 artifact、期望动作 |
| `expires_at` | datetime | 过期时间 |
| `created_at` | datetime | 创建时间 |
| `resolved_at` | datetime | 完成时间 |

### 8. `automation_audit_event`

记录任务关键事件和高风险动作。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | audit ID |
| `tenant_id` | string | 强制资源归属租户 |
| `job_id` | string | job ID |
| `run_id` | string | run ID |
| `device_id` | string | 设备 ID |
| `event_type` | string | 事件类型 |
| `risk_level` | string | `low|medium|high` |
| `summary` | string | 摘要 |
| `payload_json` | json | 脱敏后的详情 |
| `created_at` | datetime | 创建时间 |

高风险事件包括：

1. 发布前确认。
2. 表单提交。
3. 文件上传。
4. 登录/MFA/验证码。
5. 策略阻断。
6. cookie/storage 上传尝试被阻断。

### 9. `qiyuan_literature_result`

QIYUAN 文献业务结果表，不属于通用 Worker core，但用于替代旧 `crawl_result`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | result ID |
| `job_id` | string | automation job ID |
| `run_id` | string | automation run ID |
| `source` | string | `google-scholar` 等 |
| `title` | string | 标题 |
| `title_normalized` | string | 标题归一化 |
| `authors_json` | json | 作者 |
| `year` | int | 年份 |
| `doi` | string | DOI |
| `source_url` | string | 搜索结果 URL |
| `landing_url` | string | 落地页 |
| `pdf_artifact_id` | string | PDF artifact |
| `status` | string | `metadata_collected|pdf_downloaded|pdf_unavailable|failed` |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

## 五、API 设计

所有 Worker API 由 `backend-api` 提供，路径建议使用 `/worker/*`，对象使用 automation 命名。

### 1. 创建设备配对

```http
POST /worker/devices/pairing
```

请求：

```json
{
  "worker_version": "0.1.0",
  "platform": "windows-amd64",
  "hostname_hash": "sha256:...",
  "display_name": "Customer PC"
}
```

响应：

```json
{
  "pairing_id": "pair_123",
  "pairing_code": "ABCD-1234",
  "verification_uri": "https://app.example.com/worker/pair",
  "expires_at": "2026-06-17T12:00:00Z"
}
```

本地单租户模式会自动批准配对。`REQUIRE_TENANT_IDENTITY=true` 时保持 `pending`，必须由已登录租户所有者批准：

```http
POST /web/worker/pairings/{pairing_code}/approve
Authorization: Bearer <user_jwt>
```

Gateway 从 JWT 注入租户 actor，API 将 `tenant_id` 和批准用户写入 pairing/device；`tenant_viewer` 和普通 `tenant_member` 不能批准设备。

### 2. 查询配对状态

```http
GET /worker/devices/pairing/{pairing_id}
```

响应：

```json
{
  "status": "approved",
  "device_id": "dev_123",
  "device_token": "device_token_once",
  "device": {
    "id": "dev_123",
    "tenant_id": "tenant_123",
    "status": "active"
  }
}
```

租户所有者或平台管理员可查看并立即撤销本租户设备：

```http
GET /web/worker/devices
POST /web/worker/devices/{device_id}/revoke
```

### 3. 设备心跳与能力上报

```http
POST /worker/devices/{device_id}/heartbeat
Authorization: Bearer <device_token>
```

请求：

```json
{
  "worker_version": "0.1.0",
  "capabilities": [
    "browser.playwright.chromium",
    "browser.cdp.connect",
    "adapter.generic.browser.agent"
  ],
  "current_run_id": "run_123",
  "metrics": {
    "cpu_percent": 12.5,
    "memory_mb": 512,
    "browser_alive": true
  }
}
```

响应：

```json
{
  "status": "ok",
  "server_time": "2026-06-17T12:00:00Z",
  "commands": []
}
```

### 4. 领取下一个任务

```http
GET /worker/automation/jobs/next
Authorization: Bearer <device_token>
```

响应：

```json
{
  "job_id": "job_123",
  "run_id": "run_123",
  "job_type": "generic.browser.agent",
  "adapter": "generic.browser.agent",
  "target": {
    "url": "https://scholar.google.com",
    "allowed_domains": ["scholar.google.com", "*.google.com"]
  },
  "input": {
    "query": "DFT+U magnetic materials",
    "max_results": 20
  },
  "policy": {
    "requires_human_login": true,
    "requires_human_confirmation_before_publish": false,
    "max_duration_seconds": 1800,
    "allowed_actions": ["navigate", "click", "fill", "download", "screenshot"],
    "artifact_policy": {
      "allow_pdf": true,
      "allow_html": true,
      "allow_screenshot": true,
      "allow_har": false,
      "allow_cookie_upload": false
    }
  },
  "cursor": null
}
```

无任务返回 `204 No Content`。

### 5. Run 心跳

```http
POST /worker/automation/runs/{run_id}/heartbeat
Authorization: Bearer <device_token>
```

请求：

```json
{
  "job_id": "job_123",
  "status": "running",
  "stage": "collect_results",
  "progress": {
    "page_index": 2,
    "items_collected": 13
  }
}
```

### 6. 上报 checkpoint

```http
POST /worker/automation/runs/{run_id}/checkpoint
Authorization: Bearer <device_token>
Idempotency-Key: run_123:checkpoint:12
```

请求：

```json
{
  "job_id": "job_123",
  "sequence": 12,
  "stage": "collect_results",
  "cursor": {
    "page_index": 2,
    "result_index": 3
  },
  "progress": {
    "items_collected": 13
  },
  "result": {
    "title": "Example paper",
    "landing_url": "https://example.org/paper"
  }
}
```

### 7. 创建人工动作

```http
POST /worker/automation/runs/{run_id}/manual-actions
Authorization: Bearer <device_token>
```

请求：

```json
{
  "job_id": "job_123",
  "type": "captcha_required",
  "prompt": "请在本机浏览器中完成 搜索平台 验证，然后回到 Worker 继续。",
  "details": {
    "page_url": "https://scholar.google.com/sorry/...",
    "screenshot_artifact_id": "art_123"
  },
  "expires_at": "2026-06-17T12:30:00Z"
}
```

响应：

```json
{
  "manual_action_id": "ma_123",
  "status": "pending"
}
```

### 8. 查询人工动作状态

```http
GET /worker/automation/manual-actions/{manual_action_id}
Authorization: Bearer <device_token>
```

响应：

```json
{
  "status": "resolved",
  "resolved_at": "2026-06-17T12:12:00Z"
}
```

### 9. 注册 artifact

P1 可先用 multipart 一步上传；P2 可升级为 presigned URL。

```http
POST /worker/automation/runs/{run_id}/artifacts
Authorization: Bearer <device_token>
Idempotency-Key: run_123:artifact:sha256
Content-Type: multipart/form-data
```

表单字段：

| 字段 | 说明 |
| --- | --- |
| `job_id` | job ID |
| `result_id` | 业务结果 ID，可空 |
| `artifact_type` | artifact 类型 |
| `filename` | 原始文件名 |
| `content_type` | MIME |
| `sha256` | 文件 sha256 |
| `metadata_json` | JSON 字符串 |
| `file` | 文件内容 |

响应：

```json
{
  "artifact_id": "art_123",
  "storage_key": "automation/job_123/run_123/art_123.pdf",
  "status": "uploaded"
}
```

### 10. 完成 run

```http
POST /worker/automation/runs/{run_id}/complete
Authorization: Bearer <device_token>
Idempotency-Key: run_123:complete
```

请求：

```json
{
  "job_id": "job_123",
  "status": "completed",
  "summary": {
    "items_collected": 20,
    "artifacts_uploaded": 31,
    "manual_actions": 1
  }
}
```

失败时：

```json
{
  "job_id": "job_123",
  "status": "failed",
  "error_code": "CAPTCHA_TIMEOUT",
  "error_message": "User did not complete captcha before manual action expired",
  "retryable": true
}
```

## 六、任务 payload 示例

### 1. QIYUAN 搜索平台

```json
{
  "job_type": "generic.browser.agent",
  "adapter": "generic.browser.agent",
  "target": {
    "url": "https://scholar.google.com",
    "allowed_domains": ["scholar.google.com", "*.google.com"]
  },
  "input": {
    "query": "DFT+U magnetic materials",
    "max_results": 20,
    "download_pdf": true
  },
  "policy": {
    "requires_human_login": false,
    "requires_human_confirmation_before_publish": false,
    "allowed_actions": ["navigate", "click", "fill", "download", "screenshot"],
    "artifact_policy": {
      "allow_pdf": true,
      "allow_html": true,
      "allow_screenshot": true,
      "allow_har": false
    }
  }
}
```

### 2. YouTube 上传

```json
{
  "job_type": "social.youtube.upload_video",
  "adapter": "social.youtube_studio",
  "target": {
    "url": "https://studio.youtube.com",
    "allowed_domains": ["studio.youtube.com", "*.google.com", "accounts.google.com"]
  },
  "input": {
    "video_artifact_id": "art_video_123",
    "thumbnail_artifact_id": "art_thumb_123",
    "title": "Demo title",
    "description": "Demo description",
    "visibility": "private",
    "publish_mode": "draft"
  },
  "policy": {
    "requires_human_login": true,
    "requires_human_confirmation_before_publish": true,
    "allowed_actions": ["navigate", "click", "fill", "upload_file", "screenshot"],
    "artifact_policy": {
      "allow_screenshot": true,
      "allow_html": false,
      "allow_har": false,
      "allow_cookie_upload": false
    }
  }
}
```

### 3. TikTok 上传

```json
{
  "job_type": "social.tiktok.upload_video",
  "adapter": "social.tiktok_web",
  "target": {
    "url": "https://www.tiktok.com/upload",
    "allowed_domains": ["www.tiktok.com", "*.tiktok.com"]
  },
  "input": {
    "video_artifact_id": "art_video_123",
    "caption": "Demo caption #materials",
    "visibility": "private",
    "publish_mode": "draft"
  },
  "policy": {
    "requires_human_login": true,
    "requires_human_confirmation_before_publish": true,
    "allowed_actions": ["navigate", "click", "fill", "upload_file", "screenshot"],
    "artifact_policy": {
      "allow_screenshot": true,
      "allow_html": false,
      "allow_har": false,
      "allow_cookie_upload": false
    }
  }
}
```

## 七、错误码

通用错误码：

| 错误码 | 含义 | 是否可重试 |
| --- | --- | --- |
| `DEVICE_REVOKED` | 设备已撤销 | 否 |
| `CAPABILITY_MISMATCH` | Worker 缺少能力 | 否 |
| `POLICY_BLOCKED` | 本地策略阻断 | 否 |
| `BROWSER_START_FAILED` | 浏览器启动失败 | 是 |
| `BROWSER_DISCONNECTED` | 浏览器断开 | 是 |
| `NAVIGATION_TIMEOUT` | 页面导航超时 | 是 |
| `LOGIN_REQUIRED` | 需要登录 | 是，人工处理后 |
| `MFA_REQUIRED` | 需要二次验证 | 是，人工处理后 |
| `CAPTCHA_REQUIRED` | 需要验证码 | 是，人工处理后 |
| `MANUAL_ACTION_TIMEOUT` | 人工动作超时 | 是 |
| `UPLOAD_FAILED` | artifact 上传失败 | 是 |
| `ADAPTER_UNSUPPORTED` | adapter 不支持任务 | 否 |
| `ADAPTER_VERSION_OUTDATED` | adapter 版本过旧 | 否 |
| `TARGET_SITE_CHANGED` | 目标页面结构变化 | 是，升级 adapter 后 |

文献业务错误码：

| 错误码 | 含义 |
| --- | --- |
| `PDF_UNAVAILABLE` | 未找到 PDF |
| `SCHOLAR_UNUSUAL_TRAFFIC` | 搜索平台 unusual traffic |
| `RESULT_PARSE_FAILED` | 搜索结果解析失败 |

社媒业务错误码：

| 错误码 | 含义 |
| --- | --- |
| `VIDEO_UPLOAD_FAILED` | 视频上传失败 |
| `PUBLISH_CONFIRMATION_REQUIRED` | 发布前确认 |
| `PUBLISH_FAILED` | 发布失败 |
| `DRAFT_SAVE_FAILED` | 保存草稿失败 |

## 八、鉴权与安全

1. 客户 `/web/*` 与平台 `/admin/*` 由 Gateway 校验 JWT；Gateway 必须删除外部伪造的 `X-Tenant-ID`、`X-User-UUID`、`X-Tenant-Role` 后重新注入，并通过内部 secret 与 API 建立信任。
2. 生产 API 必须设置 `REQUIRE_TENANT_IDENTITY=true`，且不得把 API 端口直接暴露公网；`/internal/*` 不注册到公网 Gateway。
3. Worker 使用 device token 调用 `/worker/*`；token 可撤销、可轮换。
4. 每次领取任务校验 job tenant、设备 tenant、设备状态和 capability；每次 run 回写还要校验 device_id 与 run 一致。
5. 所有列表、详情、下载、取消、人工动作和设备操作必须以 actor 的 `tenant_id` 查询或执行 ownership 校验，跨租户统一返回不可见/拒绝。
6. Worker 本地必须校验 `allowed_domains` 和 `allowed_actions`。
7. 平台不接受 cookie/storage artifact，除非未来有单独的显式授权和加密设计。
8. HTML/HAR/console log 默认按 policy 控制；社媒后台任务默认不上传完整 HTML/HAR。
9. 所有高风险动作写 `automation_audit_event`。

## 九、实现优先级

P1：

1. 设备配对和 device heartbeat。
2. `automation_job`、`automation_run`、`automation_checkpoint`、`automation_artifact`。
3. `GET /worker/automation/jobs/next`。
4. multipart artifact 上传。
5. Browser Agent adapter 兼容。

P2：

1. `automation_manual_action`。
2. capabilities 分配。
3. YouTube/TikTok 草稿上传 PoC。
4. artifact 脱敏策略。

P3：

1. presigned artifact 上传。
2. Browser Agent 任务。
3. 工作流编排。
4. 多 adapter 包和版本管理。
