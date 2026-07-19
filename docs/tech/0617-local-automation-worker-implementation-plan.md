# Browser Agent 通用本地 Automation Worker 实施计划

> **状态说明（2026-07-19）**
>
> 仓库现为单一 Browser Agent 项目，`main` 是唯一长期主线。本文是早期实施记录，其中旧产品和文献相关阶段仅作历史参考；当前优先级以 [BRD](../brd/0627-browser-agent-automation-brd.md) 和项目交接文档为准。



## Changelog

- 2026-07-19：同步单一 Browser Agent 项目现状，移除已废弃的双分支解耦说明。
- 2026-06-17：新增通用 Automation Worker 实施计划。将旧版 搜索平台/PDF P1 调整为“通用 automation runtime 优先、QIYUAN 文献 adapter 首发、YouTube/TikTok 后续 PoC”的实施路线。

> 上游文档：
> - `docs/tech/0617-local-automation-worker-architecture.md`
> - `docs/tech/0617-local-automation-worker-api-schema.md`
>
> 旧版参考：
> - `docs/tech/0614-local-worker-p1-implementation-plan.md`
> - `docs/tech/0614-local-worker-m1-go-api-implementation.md`
> - `docs/tech/0614-local-worker-m2-cli-worker-implementation.md`
> - `docs/tech/0614-local-worker-m3-google-scholar-adapter.md`

## 一、实施目标

本计划的目标不是只完成 QIYUAN 版权检测，而是先搭出可复用的 Local Automation Worker 基座：

1. 平台能创建、调度、观察通用 automation job。
2. 本地 Worker 能领取任务、校验策略、选择 adapter、操作浏览器、上传 artifact。
3. 搜索平台 版权检测作为第一个 adapter 跑通端到端闭环。
4. 人工登录、验证码、发布前确认等能力被建模为通用状态，不写进某个爬虫脚本。
5. 后续 YouTube/TikTok 上传可复用同一套设备、任务、artifact、session 和人工确认协议。

## 二、阶段总览

```text
A0 文档冻结与旧文档标注
  -> A1 Worker runtime 分层改造
  -> A2 Automation API 与内存版后端骨架
  -> A3 Browser Agent adapter Playwright MVP
  -> A4 artifact 与 checkpoint 端到端
  -> A5 manual action / confirmation
  -> A6 YouTube/TikTok 上传 PoC
  -> A7 Browser Agent 增强
```

## 三、A0：文档冻结与旧文档标注

### 目标

避免后续代码在 `crawl_*` 和 `automation_*` 两套概念之间摇摆。

### 已完成或本阶段任务

1. 新增 `0617-local-automation-worker-architecture.md`。
2. 新增 `0617-local-automation-worker-api-schema.md`。
3. 新增本文实施计划。
4. 给 0614 旧文档增加状态说明，保留其 QIYUAN 文献 P1 参考价值。
5. README 索引同步。

### 验收

1. 后续新代码优先使用 `automation` 命名。
2. 搜索平台 只作为 adapter，不作为 Worker 平台总称。
3. 旧文档不会误导后续开发继续扩大 `crawl_*` 模型。

## 四、A1：Worker runtime 分层改造

### 目标

在不接真实 Playwright 的情况下，先把现有 `worker/local-cli` 从 mock job loop 改成 adapter runtime。

### 目录调整

建议新增：

```text
worker/local-cli/qiyuan_worker/
  adapters/
    __init__.py
    base.py
    registry.py
    qiyuan/
      __init__.py
      google_scholar.py
    social/
      __init__.py
    generic/
      __init__.py
  artifacts/
    __init__.py
    models.py
    collector.py
  browser/
    __init__.py
    runtime.py
    policy.py
  runtime/
    __init__.py
    context.py
    runner.py
    policy.py
```

### 代码任务

1. 定义 `AutomationAdapter`：

```python
class AutomationAdapter:
    name: str
    supported_job_types: tuple[str, ...]
    required_capabilities: tuple[str, ...]

    async def prepare(self, context: JobContext) -> None: ...
    async def run(self, context: JobContext) -> AdapterResult: ...
    async def cleanup(self, context: JobContext) -> None: ...
```

2. 定义 `JobContext`：

```text
job_id
run_id
job_type
adapter
target
input
policy
cursor
work_dir
api_client
artifact_collector
```

3. 定义 `AdapterResult`：

```text
status
summary
cursor
artifacts
manual_action
error_code
error_message
retryable
```

4. 定义 adapter registry：

```text
adapter name -> adapter instance
job_type -> adapter candidates
capability matching
```

5. 修改 `job_loop.py`：
   - 领取 job 后不再 mock complete。
   - 根据 `job_type/adapter` 找 adapter。
   - 调用 `prepare/run/cleanup`。
   - 统一上报 heartbeat、checkpoint、artifact、complete。

### 测试

1. 未知 adapter 返回 `ADAPTER_UNSUPPORTED`。
2. capability 不匹配返回 `CAPABILITY_MISMATCH`。
3. mock adapter 成功时上报 complete。
4. mock adapter 失败时上报 failed。
5. policy 中缺少 `allowed_domains` 时拒绝运行浏览器类任务。

### 验收

1. 不需要 Playwright，也能通过 mock adapter 跑通 runtime。
2. `qiyuan_worker` 内部已经不把 搜索平台 写死在 job loop。

## 五、A2：Automation API 与后端骨架

### 目标

在 `backend-api` 搭出通用 Worker API。早期可先内存实现，随后补 MySQL migration。

### 后端模块

```text
backend-api/
  internal/
    handler/automation/
    engine/automation/
    repository/automation/
    model/automation/
```

### API 任务

1. 保留或复用设备配对：
   - `POST /worker/devices/pairing`
   - `GET /worker/devices/pairing/{pairing_id}`
   - `POST /worker/devices/{device_id}/heartbeat`

2. 新增 automation：
   - `GET /worker/automation/jobs/next`
   - `POST /worker/automation/runs/{run_id}/heartbeat`
   - `POST /worker/automation/runs/{run_id}/checkpoint`
   - `POST /worker/automation/runs/{run_id}/manual-actions`
   - `GET /worker/automation/manual-actions/{manual_action_id}`
   - `POST /worker/automation/runs/{run_id}/artifacts`
   - `POST /worker/automation/runs/{run_id}/complete`

3. 新增调试接口或 Admin 内部入口：
   - 创建 automation job。
   - 查询 job/run/artifact。
   - 人工标记 manual action resolved。

### Migration 任务

按 `0617-local-automation-worker-api-schema.md` 新增：

1. `worker_device`
2. `worker_pairing`
3. `automation_job`
4. `automation_run`
5. `automation_checkpoint`
6. `automation_artifact`
7. `automation_manual_action`
8. `automation_audit_event`
9. `qiyuan_literature_result`

要求：

1. migration 幂等。
2. 同步 `database/init.sql`。
3. 不在 migration 中写业务 seed。
4. 状态字段代码层校验。

### 测试

1. revoked device 不能领取任务。
2. capability 不满足时不分配任务。
3. 同一 job 同一时间只有一个 active run。
4. checkpoint 幂等。
5. artifact 幂等。
6. manual action 状态流转正确。

### 验收

1. 用 curl 或测试客户端可创建 job、领取 job、上报 checkpoint、上传 artifact、complete。
2. 所有 MySQL 写入只在 go-api 内。

## 六、A3：搜索平台 Adapter Playwright MVP

### 目标

把 QIYUAN 版权检测作为第一个真实 adapter，验证浏览器自动化闭环。

### Worker 任务

1. 引入 Playwright 依赖。
2. `doctor` 检查 Playwright 和 Chromium。
3. Browser runtime 支持：
   - headed Chromium。
   - persistent context。
   - downloads path。
   - screenshot。
   - HTML snapshot。
   - storage state。

4. `generic.browser.agent` adapter 支持：
   - 打开 搜索平台。
   - 检测验证码/unusual traffic。
   - 输入 query。
   - 采集搜索结果 metadata。
   - 打开落地页。
   - 尝试下载 PDF。
   - 上传 metadata、HTML、截图、PDF。

5. 遇到验证码时：
   - 创建 `manual_action`。
   - run 进入 `needs_manual_action`。
   - 用户本机处理后继续。

### 测试

1. adapter 解析本地 fixture HTML。
2. artifact collector 计算 sha256。
3. policy 拦截非 allowed domain。
4. 无 PDF 时返回 `PDF_UNAVAILABLE` 而不是失败整个任务。

### 验收

1. 能采集至少 5 条 搜索平台 metadata。
2. 能下载至少 1 个 PDF，或明确标记 `pdf_unavailable`。
3. 能上传 PDF/HTML/screenshot/metadata。
4. 验证码场景不绕过，进入人工动作。

## 七、A4：Artifact 与 checkpoint 端到端

### 目标

让 artifact 不再只是文件上传，而成为后续 页面解析、发布结果审计、任务回放的统一证据链。

### 任务

1. Worker artifact collector：
   - 统一创建 artifact metadata。
   - 计算 sha256。
   - 判断 content type。
   - 按 policy 阻断 cookie/storage/HAR。

2. go-api artifact 接收：
   - multipart P1。
   - 幂等 key。
   - 存储 key 生成。
   - artifact 索引写库。

3. QIYUAN 文献后续触发：
   - PDF artifact 上传成功后创建 PDF parse task 或事件。
   - metadata artifact 与 `qiyuan_literature_result` 关联。

### 验收

1. 重复上传同一 artifact 不产生重复记录。
2. policy 禁止的 artifact 返回明确错误。
3. PDF artifact 能进入后端解析队列。

## 八、A5：Manual Action / Confirmation

### 目标

把登录、验证码、MFA、发布确认统一处理，为 YouTube/TikTok 做准备。

### Worker 任务

1. `needs_manual_action`：
   - CLI 显示当前需要用户做什么。
   - 可选打开或聚焦浏览器。
   - 轮询 manual action 状态。

2. `waiting_for_confirmation`：
   - 显示动作摘要。
   - 上传发布前截图。
   - 用户确认后继续执行。

3. 超时处理：
   - manual action 过期后 run failed retryable。
   - 用户取消后 run cancelled。

### 平台任务

1. Admin/Web 展示 pending manual action。
2. 支持用户点击“已完成”或“取消任务”。
3. 记录 audit event。

### 验收

1. 搜索平台 验证码可进入 manual action 并恢复。
2. YouTube/TikTok 发布前确认可复用同一状态机。

## 九、A6：YouTube/TikTok 上传 PoC

### 目标

验证 Worker 不只服务 QIYUAN 版权检测，也可执行通用浏览器业务流程。

### YouTube PoC

范围：

1. 打开 YouTube Studio。
2. 用户本机登录。
3. 上传视频 artifact。
4. 填写标题、描述、visibility。
5. 默认保存草稿，不直接公开发布。
6. 最终动作前必须人工确认。
7. 上传结果截图和状态。

暂不做：

1. 自动处理风控。
2. 批量多账号。
3. 绕过 MFA。
4. 直接公开发布作为默认行为。

### TikTok PoC

范围：

1. 打开 TikTok upload。
2. 用户扫码或本机登录。
3. 上传视频 artifact。
4. 填写 caption/hashtag。
5. 默认保存草稿。
6. 最终动作前人工确认。

验收：

1. 两个 PoC 都复用 `automation_job/run/artifact/manual_action`。
2. 不新增社媒专用调度链路。
3. 不上传 cookie/storage。

## 十、A7：Browser Agent 增强

### 目标

引入 Browser Use 风格的 action registry 和局部 AI 辅助，但不让 Agent 替代确定性 adapter。

### 任务

1. 定义基础 browser actions：
   - `navigate`
   - `click`
   - `fill`
   - `select`
   - `upload_file`
   - `download`
   - `extract_text`
   - `screenshot`

2. `agent_assist`：
   - adapter 可在局部失败时调用。
   - 必须传入 allowed domains/actions。
   - 必须有 step timeout。
   - 必须输出可审计 action log。

3. `generic.browser.agent`：
   - 作为独立任务类型。
   - 默认只允许低风险动作。
   - 高风险动作必须 human confirmation。

### 验收

1. 搜索平台 下载按钮定位失败时可用 agent assist 兜底。
2. 通用 agent 不能越过 allowed domains。
3. 发布/删除/付款类动作无法无确认执行。

## 十一、推荐下一步代码任务

当前最值得做的是 A1：

1. 在 `worker/local-cli` 新增 runtime、adapter、artifact、browser 基础包。
2. 定义 `AutomationAdapter`、`JobContext`、`AdapterResult`。
3. 写一个 `MockAdapter` 跑通新 job loop。
4. 调整现有模型，让 `Job` 支持 `job_type`、`adapter`、`target`、`input`、`policy`。
5. 补单元测试。

完成 A1 后再写 Playwright 搜索平台，会比直接在现有 `job_loop.py` 里堆逻辑更稳，也能为 YouTube/TikTok 留出正确接口。
