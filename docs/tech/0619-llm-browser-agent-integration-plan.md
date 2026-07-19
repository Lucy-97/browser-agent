# LLM Browser Agent 接入计划

> **状态说明（2026-07-19）**
>
> 仓库现为单一 Browser Agent 项目，`main` 是唯一长期主线。本文保留 LLM Browser Agent 的设计与落地记录；旧产品线示例仅作历史兼容参考，当前能力以代码、[BRD](../brd/0627-browser-agent-automation-brd.md) 和项目交接文档为准。



## Changelog

- 2026-07-19：同步单一 Browser Agent 项目现状，移除已废弃的双分支解耦说明。
- 2026-06-19：推进平台联动阶段：go-api 增加 checkpoints、trace、policy templates、run cancel、worker run status 和 Web generic browser job 接口；Admin 增加 run detail timeline/artifact/manual action/cancel；Web 增加通用 Browser Agent 任务入口；Worker action executor 支持 cancel 检查。
- 2026-06-19：完成 L1-L3 Worker 主链路：`llm_plan` 模式、可配置 LLM provider、受控 action executor、逐步 checkpoint、trace 脱敏、元素编号 overlay、`click_element(index)`、PDF download 工具和本地端到端测试；补充产品线 policy template 契约和 social draft/manual-only adapter 骨架。
- 2026-06-19：落地 Worker 产品线能力 SDK 化注册、`enabled_products` 配置、`allowed_actions`/`action_timeout_seconds` 执行约束、LLM Planner JSON action schema 校验，以及 worker mock 端到端测试。
- 2026-06-19：新增 LLM Browser Agent 接入计划，明确 deterministic adapter 与 LLM Agent 的边界、技术架构、分阶段路线、未完成清单和验收标准。

## 一、背景

当前 `generic.browser.agent` 是确定性 adapter：

1. Playwright 打开 搜索平台。
2. 解析固定 HTML/DOM 结构。
3. 抽取标题、来源 URL、PDF URL。
4. 下载 PDF artifact。
5. 遇到验证码或异常流量进入 `manual_action`。

这种方式适合 搜索平台 这类目标明确、结构相对稳定、可单测的任务。它成本低、可控、可审计，也便于合规限速和失败重试。

但未来的通用网页任务、TikTok/YouTube 上传、企业后台表单和未知网页下载，不能全部靠写死 DOM。需要引入 LLM 理解页面、规划动作、处理变化，并在高风险动作前暂停给用户确认。

## 二、核心原则

1. LLM Agent 是增强层，不替代稳定 deterministic adapter。
2. 版权检测、产物采集这类稳定业务优先用 deterministic adapter。
3. 未知网页、多步骤后台、页面结构变化大的任务使用 LLM Agent。
4. LLM 只能通过受控工具操作浏览器，不能直接绕过 policy。
5. 发布、付款、授权、删除、上传外部内容必须 `manual_action`。
6. 每一步 observation、action、screenshot、result 都必须可审计。
7. 失败时回退到人工动作或 deterministic adapter。

## 三、目标架构

```text
backend-api
  automation_job / automation_run / policy / artifact / audit
        |
        v
worker/local-cli
  runtime.JobRunner
        |
        v
  generic.browser_agent
        |
        +-- Observe Layer
        |     DOM text
        |     accessibility tree
        |     visible controls
        |     screenshot
        |
        +-- Planner Layer
        |     deterministic plan
        |     LLM action plan
        |     risk classifier
        |
        +-- Tool Layer
        |     goto / click / fill / select
        |     extract / download / screenshot
        |     wait / upload_file
        |
        +-- Guardrail Layer
              allowed_domains
              allowed_actions
              rate limit
              download type allowlist
              manual confirmation
              audit trace
```

## 四、LLM 输入

每一步给 LLM 的上下文应是可控、裁剪后的页面状态：

1. 当前 URL、title、任务目标。
2. 页面正文摘要。
3. 可交互控件列表：selector、role、name、text、visibility。
4. 截图或局部截图。
5. 上一步动作和结果。
6. policy：允许域名、允许动作、禁止动作、风险动作。

不应直接把完整 cookie、token、密码、个人隐私字段送入 LLM。页面文本和截图需要做敏感字段脱敏或最小化裁剪。

## 五、工具集设计

P1 工具：

1. `observe_page`
2. `click(selector)`
3. `fill(selector, value)`
4. `press(key)`
5. `extract(selector | instruction)`
6. `download(selector | url)`
7. `screenshot()`
8. `wait_for(condition)`

P2 工具：

1. `select_option(selector, value)`
2. `upload_file(selector, artifact_id)`
3. `scroll(direction)`
4. `open_new_tab(url)`
5. `switch_tab(index)`
6. `request_manual_action(reason, payload)`

所有工具调用前都经过 policy gate。

## 六、阶段计划

### L0：确定性 Agent 基线

已完成部分：

1. `generic.browser_agent` PoC。
2. observe/fill/submit/extract/trace/screenshot。
3. 本地 fixture demo。
4. `allowed_domains` 基础校验。

仍需补齐：

1. 每一步 checkpoint。
2. 更完整的 trace schema。
3. `allowed_actions`。
4. 速率限制和 action timeout。
5. 工具错误分类。

### L1：LLM Planner 首版

目标：

1. 接入一个可配置 LLM provider。
2. 输入 DOM/文本/控件树，输出 JSON action plan。
3. Worker 只执行 JSON schema 校验通过的 action。

任务：

1. 增加 `agent/planner.py`。
2. 增加 `agent/prompts/`。
3. 增加 `LLM_PROVIDER`、`LLM_MODEL`、`LLM_API_KEY` 配置注入。
4. 增加 action JSON schema。
5. 增加 planner 单测：无效 JSON、越权域名、禁止动作、正常动作。

验收：

1. 在本地 mock 页面上，LLM 能规划搜索、点击和抽取。
2. 禁止动作不会执行。
3. Prompt 和响应写入 trace，但敏感字段被脱敏。

### L2：截图/视觉理解

目标：

1. 当 DOM 不足以定位控件时，引入截图。
2. 支持截图裁剪、元素标号、可视区域描述。

任务：

1. 页面截图生成元素 overlay。
2. 控件编号映射 selector。
3. LLM 可返回 `click_element(index)`。
4. 截图 artifact 上传。

验收：

1. 在 DOM selector 不稳定页面上完成按钮点击。
2. 截图中敏感区域可遮罩。
3. 每次视觉动作有截图证据。

### L3：通用下载任务

目标：

支持未知网页中的文件下载。

任务：

1. Agent 识别下载链接、PDF 链接、附件按钮。
2. MIME/type allowlist。
3. 下载后校验文件头、size、sha256。
4. 失败时尝试候选链接。

验收：

1. 未知论文页能下载 PDF。
2. 非 PDF 文件不会被误当 PDF。
3. Cloudflare/403/验证码进入失败或 manual action，不绕过。

### L4：社交平台上传 PoC

目标：

验证 YouTube/TikTok 上传流程。

任务：

1. `social.youtube.upload_video` adapter。
2. `social.tiktok.upload_video` adapter。
3. 本地文件选择和上传进度记录。
4. 发布前 `manual_action`。

验收：

1. YouTube 上传测试视频并停在 private/draft。
2. TikTok 上传测试视频并停在发布确认前。
3. 没有人工确认不执行发布。

### L5：平台化 Agent 工作流

目标：

让 Admin/Web 能创建、查看和复跑 Agent 工作流。

任务：

1. Admin 展示 action trace。
2. Admin 支持中止 run。
3. Web 支持提交通用网页任务。
4. Run 详情展示截图时间线。
5. Agent policy 模板化。

验收：

1. 不看终端也能复盘 Agent 每一步。
2. 用户能看到任务为什么停住、需要自己做什么。
3. 高风险动作都能审计。

## 七、当前未完成清单

平台侧：

1. 真实账号、角色、租户和资源 ownership 校验。
2. Agent policy 持久化和模板化。
3. Run 取消、中止、超时和重试策略。
4. Agent trace UI。
5. Artifact 留存、删除、脱敏和权限控制。
6. LLM 调用审计、成本统计和 prompt 版本管理。

Worker 侧：

1. 更完整的工具级 retry 和细分 error code。
2. CDP 连接本机 Chrome。
3. 浏览器 session/profile 管理 UI。
4. 真实账号环境下的 LLM provider 联调和成本审计。

业务 adapter：

1. 搜索平台 翻页、去重、更多 PDF fallback。
2. YouTube 上传 PoC。
3. TikTok 上传 PoC。
4. 企业后台表单 PoC。

合规与安全：

1. robots/服务条款提示。
2. 数据源授权记录。
3. 高频访问限速。
4. 高风险动作强制人工确认。
5. 个人信息识别、脱敏和删除。
6. 合规审计报表。

## 八、2026-06-19 编码落地状态

已完成：

1. Worker 能力 SDK：新增 `qiyuan_worker.sdk.WorkerExtension`，支持产品线、adapter、capability 和 Playwright 依赖声明。
2. 内置产品线能力包：`core.mock`、`browser_agent.generic`、`literature.google_scholar` 均通过同一套扩展协议注册。
3. 产品线隔离配置：Worker config 新增 `enabled_products`，默认开启 `core,browser_agent,literature`，客户交付时可收敛到指定产品线。
4. 能力上报隔离：worker heartbeat capability 由运行时能力和已启用扩展能力合并生成，adapter capability 不再硬编码在 job loop 中。
5. L0 执行 guardrail：Browser Agent 每个 action 执行前校验 `allowed_actions`，并受 `action_timeout_seconds` 控制。
6. L1 Planner 基线：新增 `BrowserAgentPlanner`、planner prompt 和 JSON action schema，支持无效 JSON、越权 action、越权下载域名和正常 action plan 的单测。
7. 端到端验证：新增 `run_once` mock job E2E 测试，覆盖领取任务、按产品线注册 adapter、执行、checkpoint、artifact 和 complete run。
8. L1 执行闭环：`GenericBrowserAgentAdapter` 支持 `mode=llm_plan`，执行 planner action plan，并为 planner request/response、action start/result/error 写入 trace。
9. L2 视觉基础：`observe_page` 给可交互控件生成元素编号和 selector，`screenshot(overlay=true)` 生成编号截图，`click_element(index)` 可通过编号点击控件。
10. L3 通用下载：新增 `download(selector | url)`，校验 allowed domain、PDF 文件头、size、sha256，并将下载文件作为 artifact。
11. 平台化契约：`WorkerExtension` 新增 `PolicyTemplate`，内置产品线声明可复用的 job type、adapter、target 和 policy 模板，为 Admin/Web 创建任务提供契约。
12. L4 安全边界：新增 `social.youtube.upload_video` 和 `social.tiktok.upload_video` draft/manual-only adapter 骨架，校验视频输入后强制进入 manual action，不自动发布。

仍未完成：

1. YouTube/TikTok 真实上传 PoC 需要真实测试账号、素材和人工发布确认流程，当前完成 worker adapter 边界和高风险动作强制 manual action。
2. 产品线物理拆包仍需发布流水线支持；当前已完成 SDK、entry point、policy template 和平台模板接口契约，后续可迁移到独立 package。
3. 真实 LLM 成本、prompt 版本和 token 用量审计仍需接 provider 计量数据。

## 九、当前可用 job payload 示例

LLM plan 模式：

```json
{
  "job_type": "generic.browser.agent",
  "adapter": "generic.browser_agent",
  "target": {
    "allowed_domains": ["example.com"]
  },
  "input": {
    "url": "https://example.com/search",
    "task": "Search LiFePO4 and extract result titles",
    "mode": "llm_plan"
  },
  "policy": {
    "allowed_actions": [
      "observe_page",
      "fill",
      "click",
      "click_element",
      "press",
      "extract",
      "screenshot",
      "wait_for",
      "download"
    ],
    "action_timeout_seconds": 30,
    "max_download_bytes": 26214400
  }
}
```

Worker 本地配置：

```yaml
enabled_products: core,browser_agent,literature
llm_provider: openai_compatible
llm_model: <model-name>
```

密钥只从环境变量注入：

```bash
export LLM_API_KEY=...
export LLM_BASE_URL=https://api.openai.com/v1
```

## 十、建议下一步

1. 用真实 `openai_compatible` provider 在本地 mock HTML 页面做一次人工联调，校准 prompt。
2. 增加 provider token/cost 统计字段，并在 Admin run detail 展示。
3. 为 YouTube/TikTok 增加真实 draft 上传步骤，发布动作继续强制 manual action。
4. 将 `browser_agent`、`literature` 能力迁移成独立 Python package，并通过 `qiyuan_worker.extensions` entry point 加载。
