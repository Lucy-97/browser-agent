# QIYUAN 0617 通用本地 Automation Worker 技术架构

> **⚠️ 架构解耦声明 (2026-06-27)**
> 
> 根据最新的 [0627-browser-agent-automation-brd.md](../brd/0627-browser-agent-automation-brd.md) 业务解耦策略：
> - **AI 4 Science 相关业务**（版权检索 `generic.browser.agent`、页面解析、知识图谱等）已拆分至独立的 `feature/qiyuan` 分支。
> - 当前的 **`feature/browser-agent` 分支** 仅专注于通用底层基础能力（Playwright、调度机制）以及泛自动化运营场景（如版权监测、社交媒体维护）。
> 
> *注：本文档部分内容可能包含早期混杂的版权检测或知识图谱示例，请结合上述分支隔离原则进行阅读，具体实现以各自分支的代码为准。*



## Changelog

- 2026-06-17：新增通用本地 Automation Worker 技术架构。基于 Browser Use、Crawl4AI、Skyvern 本地源码调研，将原“自动化运营 Worker”升级为可支持爬虫、网页 Agent、文件上传、后台表单操作、TikTok/YouTube 自动发布等任务的通用本地执行节点。

> 上游文档：
> - `docs/tech/0614-local-worker-tech-design.md`
> - `docs/tech/0614-local-worker-api-schema.md`
> - `docs/tech/0614-local-worker-m3-google-scholar-adapter.md`
>
> 结论：QIYUAN 当前版权检测只是第一个业务 adapter。Worker 的长期定位应是“用户本地可信执行节点 + 浏览器自动化运行时 + artifact 采集器 + 人机协同入口”，未来可复用到 TikTok、YouTube、企业后台、表单填报、文件上传等浏览器自动化任务。

## 一、调研结论摘要

### 1. Browser Use

Browser Use 的定位是“浏览器 Agent SDK / CLI”，核心价值是把浏览器抽象成 Agent 可调用的动作空间。它不是传统爬虫，而是让 LLM 通过导航、点击、输入、截图、下载、PDF、页面抽取等动作完成网页任务。

可借鉴点：

1. `Agent + BrowserSession + BrowserProfile + Tools` 的分层。
2. 持久 CLI daemon 保持一个浏览器会话，避免每次任务重启浏览器。
3. 工具注册机制，适合把不同浏览器动作开放给 Agent。
4. `allowed_domains`、`prohibited_domains`、`sensitive_data`、动作超时、CDP 断线恢复等安全和稳定性设计。
5. MCP server/client 能把浏览器操作暴露给外部 Agent。

技术特征：

1. Python SDK + CLI。
2. 新版大量使用 CDP/Rust core，配合 `cdp-use`。
3. 支持本地浏览器、云浏览器、CDP 连接、storage state、captcha watchdog、MCP。
4. License 为 MIT，适合参考和必要时局部复用。

### 2. Crawl4AI

Crawl4AI 的定位是“LLM/RAG 友好的网页爬取、清洗、抽取库”，最贴近 QIYUAN 当前版权检测和内容采集需求。它不强调自动完成复杂账号工作流，而强调把网页产出为 Markdown、HTML、截图、PDF、结构化 JSON 等可处理 artifact。

可借鉴点：

1. `BrowserConfig + CrawlerRunConfig + CrawlResult` 的配置与结果模型。
2. Playwright / Patchright / CDP 多种浏览器模式。
3. Browser profile、storage state、stealth、proxy、full-page scan、iframe、cookie popup 清理。
4. 深度爬取、并发调度、内存自适应 dispatcher、断点恢复。
5. HTML -> Markdown -> structured extraction 的 pipeline。

技术特征：

1. Python async。
2. Playwright、Patchright、playwright-stealth、aiohttp、lxml、BeautifulSoup、Pydantic、rank-bm25、LiteLLM。
3. CLI `crwl` 支持 profile、CDP、deep crawl、LLM extraction。
4. License 为 Apache-2.0，但 LICENSE 中包含 attribution requirement，直接使用前需要做开源合规确认。

### 3. Skyvern

Skyvern 的定位是完整“浏览器自动化平台”，包含后端 API、UI、任务、工作流、凭证、artifact、浏览器直播、SDK 和 MCP。它证明浏览器 Worker 可以不只是爬虫，而是可承载通用业务流程自动化。

可借鉴点：

1. `Task / Workflow / Run / Artifact / BrowserSession` 的平台对象模型。
2. Playwright Page 包装为 AI 增强页面，标准操作和自然语言操作可混用。
3. 工作流 block 体系：action、extraction、download/upload、PDF parser、login、human interaction、conditional、loop、email、Google Sheets 等。
4. 本地浏览器、CDP 连接、云浏览器、浏览器直播、artifact/video/HAR 统一管理。
5. 凭证和人工交互被当作平台能力，而不是爬虫里的临时逻辑。

技术特征：

1. FastAPI / Uvicorn / SQLAlchemy / Alembic / Postgres 或 SQLite。
2. Playwright、React/Vite、WebSocket/CDP/VNC streaming。
3. OpenAI、Anthropic、LiteLLM、Yutori、Redis、Temporal、S3/Azure/GCS。
4. License 为 AGPLv3，只适合研究架构和产品形态，不建议直接拷贝实现进入本项目。

## 二、Worker 长期定位

原 P1 文献 Worker 的定位是：

```text
用户本机 Browser Worker -> 操作 Chromium -> 爬取 搜索平台 -> 下载 PDF -> 上传平台
```

长期应升级为：

```text
Local Automation Worker
  = 用户本地可信执行节点
  + 浏览器自动化运行时
  + 网站账号登录态容器
  + 人机协同入口
  + artifact 采集与上传代理
  + 可插拔任务 adapter 平台
```

这意味着 Worker 不绑定 QIYUAN 文献业务，也不绑定“爬虫”二字。QIYUAN 只是平台上的第一个 domain package：

```text
automation-worker-core
  -> qiyuan-literature-adapter
  -> tiktok-publish-adapter
  -> youtube-publish-adapter
  -> generic-form-fill-adapter
  -> generic-browser-agent-adapter
```

## 三、总体架构

```text
                       Web / Admin
                           |
                           v
                    backend-gateway
                           |
                           v
                       backend-api
          任务、设备、权限、状态、artifact、审计、调度
                           |
          HTTPS pull       |       artifact upload
                           v
               worker/local-cli on user's machine
      +------------------------------------------------+
      | Device Pairing / Token / Heartbeat             |
      | Job Runtime / Scheduler / Checkpoint           |
      | Browser Runtime / Playwright / CDP             |
      | Session Vault / Local Keychain / Profiles      |
      | Adapter Runtime                                |
      |   - generic.browser.agent                |
      |   - social.tiktok.upload                       |
      |   - social.youtube.upload                      |
      |   - generic.browser_agent                      |
      | Artifact Collector / Redaction / Upload        |
      | Human-in-the-loop Prompt                       |
      +------------------------------------------------+
                           |
            local Chromium / Chrome / user login
                           |
          搜索平台 / Publisher / TikTok / YouTube
                           |
                           v
                    backend-ai-engine
              页面解析、LLM 抽取、内容理解
                           |
                           v
                 MySQL / Object Storage / Neo4j
```

### 核心原则

1. Worker 主动连接平台，平台不反向连接用户机器。
2. 用户的第三方账号密码只进入本机浏览器或用户授权的本地凭证源，平台默认不保存。
3. Worker 能做浏览器任务，但任务是否允许执行由平台按用户、设备、任务类型、目标域名和权限策略下发。
4. QIYUAN 文献任务、TikTok 上传、YouTube 上传使用同一套设备绑定、任务调度、artifact、checkpoint 和审计协议。
5. P1 用确定性 adapter 优先，不急着让 LLM 接管所有操作；Agent 能力作为增强层接入。

## 四、平台对象模型

建议把原 `crawl_*` 模型逐步抽象为通用 `automation_*` 模型。P1 可以继续保留 `crawl_job`，但新增代码应尽量按通用概念设计。

| 对象 | 说明 |
| --- | --- |
| `worker_device` | 用户绑定的本地 Worker 设备 |
| `automation_job` | 平台下发的通用自动化任务 |
| `automation_run` | 一次任务执行实例，支持重试和复跑 |
| `automation_checkpoint` | Worker 执行断点、游标、阶段性结果 |
| `automation_artifact` | 文件产物，如 PDF、HTML、截图、视频、上传凭证、运行日志 |
| `automation_session` | 站点登录态或浏览器 profile 的平台索引，不保存明文 cookie |
| `automation_adapter` | 可执行能力声明，如 `generic.browser.agent` |
| `automation_policy` | 目标域名、动作类型、上传类型、人工确认、限速等策略 |
| `automation_audit_event` | 任务生命周期、敏感动作、人工介入、文件上传的审计记录 |

### Job 类型

```text
generic.browser.agent
qiyuan.literature.download_pdf
qiyuan.literature.collect_metadata
social.tiktok.upload_video
social.youtube.upload_video
generic.browser.script
generic.browser.agent
generic.form.fill
generic.file.download
generic.file.upload
```

任务 payload 示例：

```json
{
  "job_type": "social.youtube.upload_video",
  "adapter": "youtube.upload",
  "target": {
    "url": "https://studio.youtube.com",
    "allowed_domains": ["studio.youtube.com", "*.google.com"]
  },
  "input": {
    "video_artifact_id": "art_123",
    "title": "Demo title",
    "description": "Demo description",
    "visibility": "private",
    "tags": ["materials", "demo"]
  },
  "policy": {
    "requires_human_login": true,
    "requires_human_confirmation_before_publish": true,
    "max_duration_seconds": 1800,
    "record_screenshot": true,
    "record_video": false
  }
}
```

## 五、Worker 内部分层

### 1. Device Layer

负责：

1. `init`、`pair`、`status`、`doctor`。
2. 设备 token 加密保存。
3. 设备心跳、版本上报、能力上报。
4. 平台撤销设备后停止领取任务。

能力上报示例：

```json
{
  "worker_version": "0.1.0",
  "platform": "darwin-arm64",
  "capabilities": [
    "browser.playwright.chromium",
    "browser.cdp.connect",
    "artifact.upload.multipart",
    "adapter.generic.browser.agent",
    "adapter.social.youtube"
  ]
}
```

### 2. Job Runtime Layer

负责：

1. 领取任务、锁定 run、续租心跳。
2. checkpoint、幂等重试、取消、超时。
3. job 状态机：`queued -> assigned -> running -> needs_manual_action -> running -> uploading -> completed`。
4. 把业务 adapter 的结果转换成统一事件和 artifact。

### 3. Browser Runtime Layer

负责：

1. 启动 bundled Chromium 或连接本机 Chrome CDP。
2. persistent context 和 profile 管理。
3. 下载目录、viewport、trace、screenshot、HAR、console log。
4. 页面动作：goto、click、fill、select、upload_file、download、wait、extract、screenshot。
5. 站点域名限制、弹窗处理、超时、重试、断线恢复。

P1 技术选择：

1. Python 3.12。
2. Playwright async。
3. headed Chromium 优先，方便用户登录、验证码和人工确认。
4. CDP 连接作为后续能力，允许用户选择自己已登录的 Chrome profile。

### 4. Session Vault Layer

负责：

1. 本地保存站点登录态、profile 映射和 adapter session。
2. macOS 优先用 Keychain 保存敏感 token。
3. 平台只保存 session 索引、状态和最后使用时间，不保存第三方账号密码。
4. 支持用户清除某个站点 session。

站点 session 示例：

```text
~/.qiyuan-worker/sessions/
  google-scholar/
    storage-state.enc
  youtube/
    user-data-dir/
  tiktok/
    user-data-dir/
```

### 5. Adapter Runtime Layer

Adapter 是 Worker 的业务插件。每个 adapter 必须声明：

1. 支持的 `job_type`。
2. 允许访问的 domain。
3. 需要的输入 schema。
4. 产出的 artifact 类型。
5. 是否需要用户登录。
6. 是否需要发布前人工确认。
7. checkpoint 粒度。
8. 失败类型和可重试策略。

接口草案：

```python
class AutomationAdapter:
    name: str
    supported_job_types: list[str]

    async def prepare(self, context: JobContext) -> None:
        ...

    async def run(self, context: JobContext) -> AdapterResult:
        ...

    async def cleanup(self, context: JobContext) -> None:
        ...
```

### 6. Artifact Layer

统一收集并上传：

1. 输入文件：待上传视频、封面图、文档、CSV。
2. 页面证据：HTML、Markdown、截图、MHTML、HAR、console log。
3. 输出文件：PDF、下载附件、发布后的页面截图。
4. 运行报告：adapter result、错误摘要、审计事件。

所有 artifact 上传必须具备：

1. `sha256`。
2. `content_type`。
3. `artifact_type`。
4. `job_id/run_id/result_id`。
5. 幂等 key。
6. 可配置脱敏规则。

### 7. Human-in-the-loop Layer

浏览器自动化长期一定会遇到登录、验证码、二次确认、发布前确认。Worker 不应伪装绕过这些环节，而应把它们建模成状态。

状态：

```text
running -> needs_manual_action -> running
running -> waiting_for_confirmation -> running
running -> failed_terminal
```

人工动作类型：

| 类型 | 场景 |
| --- | --- |
| `login_required` | 用户需要在本机浏览器登录 |
| `captcha_required` | 出现验证码或 unusual traffic |
| `mfa_required` | 需要短信、邮箱、TOTP、Passkey |
| `publish_confirmation` | TikTok/YouTube 发布前要求用户确认 |
| `file_selection_required` | 需要用户确认上传文件 |
| `policy_blocked` | 平台策略阻止继续执行 |

## 六、任务执行流程

### 1. QIYUAN 版权检测

```text
平台创建 generic.browser.agent
  -> Worker 领取任务
  -> Browser Agent adapter 启动 Chromium
  -> 用户按需登录 Google 或机构账号
  -> adapter 搜索关键词
  -> 采集标题、作者、年份、引用、落地页、PDF 链接
  -> 下载 PDF 或标记 unavailable
  -> 上传 metadata/html/screenshot/pdf
  -> 后端 PDF parser 和 extraction pipeline 继续处理
```

特点：

1. 以确定性 DOM/文本逻辑为主。
2. Agent 仅作为兜底，例如定位不稳定下载按钮。
3. 输出是科研数据和 PDF artifact。

### 2. YouTube 自动上传

```text
平台创建 social.youtube.upload_video
  -> Worker 下载或读取待上传视频 artifact
  -> Worker 打开 YouTube Studio
  -> 用户在本机 Chrome 登录 Google/YouTube
  -> adapter 上传视频文件
  -> 填写标题、描述、标签、缩略图、可见性
  -> 发布前进入 waiting_for_confirmation
  -> 用户确认后继续点击发布或保存草稿
  -> 上传发布结果截图、URL、运行日志
```

特点：

1. 强依赖用户账号、MFA 和平台风控，必须本地执行。
2. 发布是高风险动作，默认要求人工确认。
3. 不保存用户 Google 密码，最多保存本地浏览器 profile。

### 3. TikTok 自动上传

```text
平台创建 social.tiktok.upload_video
  -> Worker 打开 TikTok upload 页面
  -> 用户扫码或账号登录
  -> adapter 上传视频
  -> 填写 caption、hashtags、封面、隐私设置
  -> 发布前人工确认
  -> 发布或保存草稿
  -> 上传结果 URL、截图和状态
```

特点：

1. 页面变化频繁，需 adapter 版本化。
2. 移动端/桌面端入口可能不同，需要多策略。
3. 风控和验证码必须转人工，不做绕过。

## 七、确定性 Adapter 与 Browser Agent 的关系

不要一开始把所有任务都交给 LLM Agent。建议采用三层策略：

```text
Level 1: Deterministic Adapter
  固定流程、固定 selector、明确 wait、明确 checkpoint

Level 2: Assisted Adapter
  主流程确定性，局部用 AI/视觉/文本定位兜底

Level 3: Generic Browser Agent
  用户给自然语言目标，Agent 自主规划动作，强审计和强限制
```

P1 选择：

1. 搜索平台：Level 1 为主，必要时 Level 2。
2. YouTube/TikTok 上传：先做 Level 1/2，不建议直接 Level 3。
3. 通用临时网页任务：后续单独开放 Level 3，必须加目标域名白名单、动作白名单和人工确认。

## 八、安全与合规边界

### 1. 第三方账号

1. 平台不采集、不保存第三方账号密码。
2. 用户在本机浏览器中登录。
3. Worker 可以保存本地浏览器 profile 或 storage state，但必须可清除。
4. 对 YouTube/TikTok 这类高价值账号，默认不上传 cookie、localStorage、sessionStorage。

### 2. 发布类动作

发布视频、删除内容、提交表单、付款、授权等动作属于高风险动作。

默认策略：

1. 下发任务前平台展示动作摘要。
2. Worker 执行到最终提交前进入 `waiting_for_confirmation`。
3. 用户在本机确认后才继续。
4. 审计记录保存确认时间、任务参数摘要、结果截图。

### 3. 域名和动作限制

每个 job 必须包含：

1. `allowed_domains`。
2. `allowed_actions`。
3. `max_duration_seconds`。
4. `artifact_policy`。
5. `human_confirmation_policy`。

Worker 运行时必须执行本地校验，不能只依赖平台。

### 4. Artifact 脱敏

上传 artifact 前按类型处理：

| artifact | 默认策略 |
| --- | --- |
| PDF | 可上传 |
| HTML | 默认上传前脱敏，社媒后台页面默认不上传完整 HTML |
| Screenshot | 可上传，但发布/账号页面要允许打码 |
| HAR | 默认关闭，调试时手工开启 |
| Console log | 默认上传错误摘要，不上传完整日志 |
| Cookies/storage | 默认禁止上传 |

## 九、技术选型建议

### Worker

P1：

1. Python 3.12 CLI。
2. Playwright async。
3. macOS Keychain 保存 device token。
4. 本地 YAML/JSON 配置。
5. HTTPS pull 模型。
6. adapter 以 Python package/module 形式注册。

P2：

1. 支持 Windows / Linux。
2. 支持连接用户自己的 Chrome CDP。
3. 支持本地小型 UI 或菜单栏程序。
4. 引入 Browser Agent 能力，优先参考 Browser Use 的 tools/action registry。

P3：

1. Worker runtime 抽象独立为 `automation-worker-core`。
2. QIYUAN 文献能力成为 `qiyuan-literature` adapter。
3. TikTok/YouTube 成为独立 social adapter。
4. 可按客户部署不同 adapter 包。

### 平台

1. `backend-api` 负责设备、任务、run、artifact、策略、审计的唯一写入。
2. `backend-ai-engine` 负责 页面解析、内容理解、LLM 规划或结果校验。
3. `frontend-admin` 提供任务创建、Worker 设备管理、run 观察和人工确认记录。
4. 对象存储保存 artifact，MySQL 保存索引和状态。
5. Neo4j/MCP 仍服务 QIYUAN 科研知识图谱场景，不进入通用 Worker core。

## 十、仓库结构建议

当前保留既有横向目录风格：

```text
worker/
  local-cli/
    qiyuan_worker/
      browser/
        runtime.py
        profile.py
        actions.py
      adapters/
        qiyuan/
          google_scholar.py
        social/
          youtube.py
          tiktok.py
        generic/
          browser_script.py
          browser_agent.py
      artifacts/
        collector.py
        redactor.py
        uploader.py
      runtime/
        job_runner.py
        policy.py
        checkpoint.py
      sessions/
        vault.py

backend-api/
  internal/
    handler/automation/
    engine/automation/
    repository/automation/
    model/automation/
```

命名建议：

1. 代码层逐步使用 `automation`，不要继续扩大 `crawl` 命名。
2. QIYUAN 文献可以保留 `crawl` 作为业务动作，不作为 Worker 平台总称。
3. 任务类型用点分命名：`generic.browser.agent`、`social.youtube.upload_video`。

## 十一、实施路线

### M1：保留现有 P1，抽象命名

1. `worker/local-cli` 继续完成 搜索平台 Playwright 能力。
2. 新增 adapter registry，而不是把 搜索平台 写死在 job loop。
3. job payload 增加 `job_type`、`adapter`、`allowed_domains`、`policy`。
4. artifact 类型泛化，不只服务 PDF。

### M2：平台通用 Automation API

1. 在 `backend-api` 实现 `automation_job/run/artifact` 的最小 API。
2. 原 `crawl_job` 可作为 view 或兼容层。
3. Admin 提供任务创建和 run 观察。
4. 设备上报 capabilities，平台按 capabilities 分配任务。

### M3：浏览器 Session 和人工介入

1. Worker 支持站点 session 管理。
2. 平台支持 `needs_manual_action` 和 `waiting_for_confirmation`。
3. 前端显示当前需要用户在本机做什么。
4. Worker 在本机 terminal 或轻量 UI 中提示用户。

### M4：YouTube/TikTok 上传 PoC

1. 先支持“保存草稿”而不是直接发布。
2. 视频文件从平台 artifact 下载到本机临时目录。
3. 上传前和最终提交前都需要人工确认。
4. 只上传结果截图、状态和公开视频/草稿 URL，不上传账号页面完整 HTML。

### M5：Browser Agent 增强

1. 引入 Browser Use 风格 action registry。
2. 给 adapter 提供 `agent_assist(prompt, allowed_actions, allowed_domains)`。
3. 对通用 agent 任务提供强审计、强限制和人工确认。
4. 不把 Agent 作为确定性 adapter 的替代，而作为页面变化和不稳定流程的兜底。

## 十二、当前下一步

建议下一步不是直接写 TikTok/YouTube，而是把现有 Worker 代码改成可插拔 adapter 运行时：

1. 在 `worker/local-cli/qiyuan_worker/` 下新增 `adapters/`、`browser/`、`artifacts/`、`runtime/` 基础包。
2. 定义 `AutomationAdapter`、`JobContext`、`AdapterResult`、`ArtifactRef`。
3. 把当前 mock job loop 改成按 `job_type/adapter` 分发。
4. 实现 Browser Agent adapter 的 Playwright MVP。
5. 后端 M1 API 同步按通用 `automation` 命名设计，避免后面从 `crawl` 迁移。

