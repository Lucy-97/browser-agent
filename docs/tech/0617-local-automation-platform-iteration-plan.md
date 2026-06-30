# QIYUAN 0617 通用 Automation 平台整体迭代计划

> **⚠️ 架构解耦声明 (2026-06-27)**
> 
> 根据最新的 [0627-browser-agent-automation-brd.md](../brd/0627-browser-agent-automation-brd.md) 业务解耦策略：
> - **AI 4 Science 相关业务**（版权检索 `generic.browser.agent`、页面解析、知识图谱等）已拆分至独立的 `feature/qiyuan` 分支。
> - 当前的 **`feature/browser-agent` 分支** 仅专注于通用底层基础能力（Playwright、调度机制）以及泛自动化运营场景（如版权监测、社交媒体维护）。
> 
> *注：本文档部分内容可能包含早期混杂的版权检测或知识图谱示例，请结合上述分支隔离原则进行阅读，具体实现以各自分支的代码为准。*



## Changelog

- 2026-06-18：增强人工介入恢复和通用 Browser Agent Demo。Browser Agent adapter 现在会在验证码/登录拦截时采集 blocked HTML、截图和错误上下文，创建 `manual_action` 后等待平台侧 resolved，再复用同一浏览器页面继续解析；新增 `generic.browser_agent` adapter，具备 observe/fill/submit/extract/screenshot/trace 最小能力，并通过 `deploy-local/integration-test/30-browser-agent-demo.sh` 在本地 fixture 上完成可验收 demo。
- 2026-06-18：完成真实 搜索平台 产物采集实战验证。第一次 headless run 命中验证码并进入 `needs_manual_action`；headed run 可检索到真实 PDF 链接。修复 Worker 产物采集逻辑：直接 `.pdf` URL 优先使用 Python HTTP fallback 下载，避免 Chromium PDF viewer 保存成 HTML；新增单测覆盖浏览器 502 后 fallback。最终真实任务 `job_2c8af88224c26c4e` 生成 4,990,095 bytes PDF artifact，平台下载校验 `%PDF-1.3` 和 sha256 通过。
- 2026-06-18：补基础接口鉴权边界。`backend-api` 新增路径级 Admin/Web token guard，`ADMIN_API_TOKEN` 和 `WEB_API_TOKEN` 为空时保持本地开发免 token；配置后 `/admin/*` 和 `/web/*` 分别要求 `X-Admin-Token`/`X-Web-Token` 或 Bearer token；Admin/Web 前端已支持透传 token，Go 测试覆盖 401/200。
- 2026-06-18：补齐 Web 用户入口首版。新增无依赖 `frontend-web` 静态 SPA，支持创建 搜索平台 版权检测任务、查看任务和解析结果，并展示本地 Worker 启动命令；新增 `run-web-host-local.sh`；`backend-api` 新增 `/web/*` 任务/结果接口和 localhost CORS，已通过桌面/移动 Playwright 页面检查。
- 2026-06-18：增强 Admin 可观测。`frontend-admin` 新增 Literature tab，展示 页面解析状态、DOI、任务关联和 retry 操作；Runs 的 artifact 列表新增下载入口；移动端折叠导航补充 `aria-label`，已通过桌面/移动 Playwright 页面检查。
- 2026-06-18：启动 P4 页面解析队列后端闭环。新增 `literature` model/engine/repository/handler 分层；PDF artifact 创建后自动生成 `qiyuan_literature_result` pending 解析任务；新增 Admin literature result 查询/重试接口和 Internal parse task 领取/写回接口，MySQL smoke 已覆盖 pending -> in_progress -> parsed。
- 2026-06-18：补齐本地 artifact 文件上传/下载闭环。`backend-api` 新增 Worker multipart 文件上传接口和 Admin artifact 下载接口，支持落盘到 `ARTIFACT_DIR` 并记录 sha256/size/metadata；Worker runtime 对存在本地文件的 artifact 自动上传文件；mock/MySQL smoke 已覆盖上传后下载校验。
- 2026-06-18：启动 P5 Admin 可观测首版。新增 `frontend-admin` Vite/React 控制台，支持查看 jobs、runs、devices、manual actions、run artifacts，创建 mock job、resolve manual action 和 revoke device；新增 `run-admin-host-local.sh`，并将 `run-api-host-local.sh` 改为 tmux 常驻模式。
- 2026-06-18：补齐 P3 持久化和可观测前置。新增本地 MySQL/Redis Docker Compose、schema apply 脚本和 MySQL/Redis smoke test；`backend-api` 补齐 Admin job/run/manual action 列表、manual action resolve、Worker device 列表和 revoke；修复 MySQL job claim 中 capabilities 读取和 `SELECT ... FOR UPDATE` rows 未关闭导致的 bad connection；Browser Agent adapter 增加受控 产物采集并登记 PDF artifact。
- 2026-06-18：接入 MySQL repository 首版。`backend-api` 新增 MySQL driver、连接池配置、worker/automation MySQL repository；配置 `MYSQL_DSN` 时使用 MySQL 持久化，未配置时继续使用内存 repository。新增 `worker_device.device_token_hash` schema 迁移，用于持久化 Worker token 鉴权。
- 2026-06-18：补充 Admin 调试查询接口。`backend-api` 新增 job detail、run detail、run artifacts、run manual actions 查询能力；automation/worker engine 已进一步面向 repository interface，便于后续替换 MySQL repository。
- 2026-06-18：引入 Redis 作为平台并发控制基础。`backend-api` 新增 Redis/no-op locker 抽象和最小 Redis RESP client，在 `automation:jobs:claim` 上使用短租约锁保护 job claim；`REDIS_ADDR` 未配置时保持本地 mock 流程可运行。MySQL repository 仍为下一步。
- 2026-06-18：推进 P3 真实浏览器闭环代码骨架。新增 Playwright persistent context 打开接口、`generic.browser.agent` adapter 首版、搜索平台 HTML 解析、PDF 链接识别、HTML/screenshot/metadata artifact 采集路径和 fake page 单元测试；已用本地 `data:` 页面验证 headless Chromium 可启动。真实 搜索平台 联网 E2E 与 产物采集尚未执行。
- 2026-06-18：完成 P3 前置第一批代码。新增 `database/init.sql` 与 `database/migrations/20260618_automation_platform_schema.sql`，覆盖 `worker_*`、`automation_*` 与 `qiyuan_literature_result`；新增 `deploy-local` 的 backend-api 本机启动脚本和 Automation mock smoke test；新增 Worker browser runtime doctor 骨架和可选 Playwright extra。真实 Browser Agent adapter 与 MySQL repository 仍属于后续 P3 实现。
- 2026-06-18：完成 P1/P2 首版代码骨架。`backend-api` 已提供内存版 worker/automation API、设备配对、任务领取、run heartbeat/checkpoint/artifact/manual action/complete 和 Go 生命周期测试；`worker/local-cli` 已完成 runtime/adapters/artifacts/protocols 分层、mock adapter 和 Python runtime 测试。数据库 migration、Admin/Web 页面和真实 Playwright adapter 进入后续阶段。
- 2026-06-17：新增通用 Automation 平台整体迭代计划。本文把 Local Automation Worker、平台 go-api、数据库、Admin、Web、AI/页面解析、部署验证和后续 YouTube/TikTok 自动化统一到一条端到端路线中，避免只从 Worker 视角推进。

> 上游文档：
> - `docs/tech/0617-local-automation-worker-architecture.md`
> - `docs/tech/0617-local-automation-worker-api-schema.md`
> - `docs/tech/0617-local-automation-worker-implementation-plan.md`
>
> 定位说明：`0617-local-automation-worker-implementation-plan.md` 是 Worker 子计划；本文是平台级总计划。后续排期、提交和验收优先按本文拆分，Worker 内部实现细节再落到子计划。

## 一、总目标

本阶段不是单独做一个本地爬虫脚本，而是搭出一套可复用的浏览器自动化平台：

1. 平台可以创建、调度、观察和审计通用 `automation_job`。
2. 用户本地 Worker 可以绑定设备、领取任务、操作本机 Chromium、上传 artifact、请求人工介入。
3. QIYUAN 版权检测和 产物采集作为首个业务闭环跑通。
4. 页面解析、结构化抽取、知识图谱入库继续留在平台后端，不下沉到 Worker。
5. 后续 YouTube/TikTok 上传、企业后台表单、通用 Browser Agent 可复用同一套设备、任务、artifact、人工确认和审计模型。

## 二、核心原则

1. `backend-api` 是唯一业务写入节点，所有 MySQL 写入、任务状态流转、artifact 元数据和审计都在 Go API 完成。
2. Worker 主动 pull 平台任务，平台不反向连接用户机器。
3. 用户的第三方账号密码默认只进入本机浏览器或本地系统凭证，不上传平台。
4. Worker 不是只服务文献爬虫，而是平台的本地可信执行节点。
5. P1 优先确定性 adapter，不急着让 LLM 接管高风险操作。
6. 发布、删除、付款、授权等敏感动作必须支持人工确认和审计。
7. 每个阶段都必须能独立验证，不依赖“大一统完成后再联调”。

## 三、目标目录形态

### 1. 平台后端

```text
backend-api/
  internal/
    handler/
      automation/
      worker/
    engine/
      automation/
      worker/
      literature/
    repository/
      automation/
      worker/
      literature/
    model/
      automation/
      worker/
      literature/
```

说明：

1. `worker` 负责设备绑定、heartbeat、能力声明、设备撤销。
2. `automation` 负责通用 job/run/checkpoint/artifact/manual action/audit。
3. `literature` 负责 QIYUAN 文献业务结果表、页面解析状态、Neo4j 入库状态。
4. Go API 仍保持 Handler -> Engine -> Repository 分层。

### 2. 本地 Worker

```text
worker/local-cli/qiyuan_worker/
  adapters/
    base.py
    registry.py
    qiyuan/google_scholar.py
    social/
    generic/
  artifacts/
    models.py
    collector.py
  browser/
    runtime.py
    policy.py
  runtime/
    context.py
    runner.py
    policy.py
  protocols/
    models.py
```

说明：

1. `runtime` 是任务执行主干。
2. `adapters` 承载 搜索平台、YouTube、TikTok、通用表单等业务能力。
3. `browser` 封装 Playwright/Chromium/CDP，不让 adapter 直接管理浏览器生命周期。
4. `artifacts` 统一处理截图、HTML、PDF、下载文件、日志、trace。
5. `protocols` 对齐平台 API payload 和状态枚举。

### 3. Admin 与 Web

```text
frontend-admin/
  automation/
    jobs
    runs
    devices
    artifacts
    manual-actions
    literature

frontend-web/
  worker onboarding
  user job submit
  result preview
```

说明：

1. Admin 优先做内部运营、调试和可观测性。
2. Web 后做用户可见的设备绑定、任务提交、结果查看。
3. 早期不追求复杂 UI，先保证任务闭环可操作、可排障。

## 四、阶段路线

```text
P0 文档与边界冻结
  -> P1 平台最小骨架
  -> P2 Worker runtime 最小闭环
  -> P3 搜索平台 + PDF artifact 闭环
  -> P4 页面解析与文献结构化结果
  -> P5 Admin 可观测与人工介入
  -> P6 Web 用户入口与体验收敛
  -> P7 YouTube/TikTok 自动化 PoC
  -> P8 Browser Agent 增强
```

## 五、P0：文档与边界冻结

### 目标

统一后续代码主线，避免继续扩大旧版 `crawl_*` 设计。

### 任务

1. 保留 0614 文档作为 QIYUAN 文献 P1 参考。
2. 新代码优先使用 `automation_*` 模型。
3. 明确 Worker、go-api、AI/PDF、Admin/Web 的边界。
4. README 文档索引同步。

### 验收

1. 新增任务和代码评审时能明确落到 `automation` 主线。
2. 搜索平台 被定义为 adapter，而不是平台总称。
3. 页面解析不下沉到本地 Worker。

## 六、P1：平台最小骨架

### 目标

先让平台可以表达设备、任务、run、artifact 和人工动作，即使早期 repository 可以用内存或轻量持久化实现。

### Go API

1. 新增 `worker_device` 设备绑定和 heartbeat 接口。
2. 新增 `automation_job` 创建、查询、领取接口。
3. 新增 `automation_run` heartbeat、checkpoint、complete 接口。
4. 新增 `automation_artifact` 元数据登记接口。
5. 新增 `automation_manual_action` 创建和 resolve 接口。
6. 新增基础 audit event 记录。

### 数据库

1. 新增幂等 migration。
2. 同步 `database/init.sql`。
3. 表结构覆盖：
   - `worker_device`
   - `worker_pairing`
   - `automation_job`
   - `automation_run`
   - `automation_checkpoint`
   - `automation_artifact`
   - `automation_manual_action`
   - `automation_audit_event`
   - `qiyuan_literature_result`

### Admin

1. 先提供内部调试入口或最小页面。
2. 支持创建测试 job。
3. 支持查看 job/run 状态和最近错误。

### 验收

1. 通过 curl 或测试客户端可以创建 job。
2. Worker token 合法时可以领取 job。
3. revoked device 不能领取任务。
4. 同一个 job 同一时间只有一个 active run。

## 七、P2：Worker runtime 最小闭环

### 目标

Worker 不接真实 Playwright，也能完成“领取任务 -> adapter 执行 -> checkpoint -> artifact 元数据 -> complete”的通用闭环。

### Worker

1. 按 `runtime/adapters/artifacts/browser/protocols` 分层。
2. 定义 `AutomationAdapter`、`JobContext`、`AdapterResult`。
3. 实现 adapter registry 和 capability matching。
4. 实现 mock adapter。
5. `job_loop.py` 改为调度 runtime，不直接写业务逻辑。

### 平台配合

1. Go API 支持 mock job 下发。
2. Go API 接受 checkpoint 和 complete。
3. Admin 能看到 run 状态变化。

### 测试

1. 未知 adapter 返回 `ADAPTER_UNSUPPORTED`。
2. capability 不匹配返回 `CAPABILITY_MISMATCH`。
3. mock adapter 成功上报 completed。
4. mock adapter 失败上报 failed。
5. Worker 被 revoke 后停止领取新任务。

### 验收

1. 不启动浏览器也能跑通端到端任务生命周期。
2. Worker 主循环里没有 搜索平台 业务逻辑。

## 八、P3：搜索平台 + PDF artifact 闭环

### 目标

把 QIYUAN 版权检测作为第一个真实 adapter，证明浏览器自动化和 artifact 上传可用。

### Worker

1. 引入 Playwright async。
2. `doctor` 检查 Python、Playwright、Chromium、下载目录和平台连通性。
3. Browser runtime 支持 headed Chromium、persistent context、downloads path、screenshot、HTML snapshot。
4. `generic.browser.agent` adapter 支持关键词检索、翻页、结果抽取、PDF 链接识别、产物采集。
5. 遇到验证码、登录、异常跳转时创建 manual action，不在脚本里死循环。

### 平台

1. 支持 PDF、HTML、截图、运行日志 artifact 元数据。
2. 支持 artifact 去重和幂等上传。
3. 支持将下载的 PDF 标记为待解析。
4. 支持文献结果初步入 `qiyuan_literature_result`。

### Admin

1. 展示版权检测任务详情。
2. 展示搜索关键词、结果数、PDF 数、失败原因。
3. 能下载或预览 artifact。

### 验收

1. 输入关键词后，Worker 能打开本机 Chromium 执行 搜索平台 检索。
2. 能上传至少 HTML snapshot、截图和 PDF artifact。
3. 平台能看到 run、checkpoint、artifact 和文献结果。
4. 验证码场景能停在 `needs_manual_action`，用户处理后继续。

## 九、P4：页面解析与文献结构化结果

### 目标

把平台收到的 页面解析成 QIYUAN 后续知识图谱需要的结构化数据。

### AI/PDF 服务

1. PDF 文本抽取。
2. 元数据抽取：title、authors、year、doi、venue、abstract。
3. DFT+U、材料体系、计算方法、参数、结论等字段抽取。
4. 失败 PDF 进入可重试状态。

### Go API

1. 触发 页面解析任务。
2. 保存解析状态和结构化结果。
3. 统一管理 artifact 与解析结果的关联。
4. 对 Neo4j 入库保留独立状态，不在解析时直接混写。

### 数据

1. MySQL 保存解析结果、状态和审计。
2. Neo4j 入库作为后续阶段或独立 pipeline。
3. 保持原始 PDF、HTML、截图可追溯。

### 验收

1. 已下载 PDF 能进入解析队列。
2. 解析结果可在 Admin 查看。
3. 解析失败可重试并保留错误原因。
4. 不在 Worker 内做 页面解析。

## 十、P5：Admin 可观测与人工介入

### 目标

让内部运营和研发能看清 Worker、任务、artifact、人工动作和错误，不靠日志猜状态。

### Admin

1. Worker 设备列表：在线状态、版本、能力、最近 heartbeat、撤销。
2. Job 列表：job type、adapter、状态、优先级、创建人。
3. Run 详情：checkpoint、artifact、错误、耗时、重试次数。
4. Manual action 列表：登录、验证码、确认发布、文件选择等。
5. Literature 结果页：文献、PDF、解析状态、抽取字段。

### Go API

1. Admin 查询接口。
2. 人工动作 resolve 接口。
3. 基础权限和审计。

### 验收

1. 不看终端日志也能定位一个任务卡在哪。
2. 能从 Admin 撤销 Worker 设备。
3. 能从 Admin 处理或关闭 manual action。

## 十一、P6：Web 用户入口与体验收敛

### 目标

把内部验证能力转成用户可用入口。

### Web

1. Worker onboarding：下载安装说明、pairing code、绑定状态。
2. 创建版权检测任务：关键词、年份、期刊、最大结果数。
3. 查看任务状态：排队、运行中、需要人工、完成、失败。
4. 查看结果：文献列表、PDF 状态、解析摘要。

### 平台

1. 用户权限绑定 job 和 device。
2. 任务配额、限速、取消。
3. 错误提示面向用户，不暴露内部堆栈。

### 验收

1. 用户能完成设备绑定。
2. 用户能提交版权检测任务并看到结果。
3. 用户能处理需要本地登录或验证码的状态。

## 十二、P7：YouTube/TikTok 自动化 PoC

### 目标

验证 Worker 不只是爬虫代理，而是通用浏览器自动化节点。

### Worker

1. 新增 `social.youtube.upload_video` adapter。
2. 新增 `social.tiktok.upload_video` adapter。
3. 支持本地文件选择、上传进度、草稿保存、截图证明。
4. 发布前必须走 manual confirmation。

### 平台

1. job payload 支持视频 artifact、标题、描述、标签、visibility、发布时间。
2. artifact 支持视频源文件、上传截图、发布结果页面。
3. audit event 记录敏感动作。

### Admin/Web

1. PoC 先在 Admin 或内部入口创建任务。
2. 展示上传状态、确认动作和结果链接。

### 验收

1. 能把测试视频上传到 YouTube Studio 并保存为 private 或 draft。
2. 能把测试视频上传到 TikTok 并停在发布确认前。
3. 发布动作默认不自动执行，必须人工确认。

## 十三、P8：Browser Agent 增强

### 目标

在确定性 adapter 稳定后，引入可控的 Browser Agent 能力，用于低风险通用网页任务。

### 能力

1. 受限工具集：goto、click、fill、extract、screenshot、download。
2. 域名和动作白名单。
3. 每一步 checkpoint 和 screenshot。
4. 高风险动作前暂停等待人工确认。
5. Agent 失败时回落到人工动作或 deterministic adapter。

### 验收

1. Agent 只能在 policy 允许的域名和动作内运行。
2. 每个动作可审计、可回放、可中止。
3. 不允许 Agent 绕过发布、付款、授权等人工确认。

## 十四、并行工作流

### 1. 平台主线

```text
schema/migration -> go-api automation API -> Admin 可观测 -> Web 用户入口
```

### 2. Worker 主线

```text
runtime 分层 -> mock adapter -> Playwright runtime -> 搜索平台 -> social adapters
```

### 3. 数据主线

```text
artifact -> PDF parser -> structured literature result -> Neo4j ingest -> MCP query
```

### 4. 运维主线

```text
deploy-local env -> compose/scripts -> smoke test -> logs/healthcheck -> release package
```

## 十五、近期建议顺序

### 第 1 个提交：Worker runtime 分层

范围：

1. `worker/local-cli/qiyuan_worker/runtime/`
2. `worker/local-cli/qiyuan_worker/adapters/`
3. `worker/local-cli/qiyuan_worker/artifacts/`
4. `worker/local-cli/qiyuan_worker/protocols/`
5. mock adapter 测试

理由：先稳住 Worker 内部主干，后面 搜索平台、YouTube、TikTok 都能按 adapter 扩展。

### 第 2 个提交：Go API automation 骨架

范围：

1. `backend-api/internal/{handler,engine,repository,model}/automation/`
2. `backend-api/internal/{handler,engine,repository,model}/worker/`
3. 内存 repository 或最小 MySQL repository。
4. Worker API 路由和基础测试。

理由：让 Worker 有真实平台可以对接，不再只跑本地 mock。

### 第 3 个提交：数据库 migration 与 init.sql

范围：

1. `database/migrations/`
2. `database/init.sql`
3. schema 幂等验证脚本。

理由：平台状态从内存走向持久化，保证本地和后续部署一致。

### 第 4 个提交：端到端 smoke test

范围：

1. 创建 mock automation job。
2. Worker 领取并完成。
3. Go API 保存 checkpoint/artifact/run 状态。
4. 提供本地验证脚本。

理由：在引入真实浏览器前先证明协议和状态机正确。

### 第 5 个提交：搜索平台 Playwright MVP

范围：

1. Browser runtime。
2. Browser Agent adapter。
3. HTML/screenshot/PDF artifact。
4. manual action。

理由：第一个真实业务闭环。

## 十六、阶段验收矩阵

| 阶段 | 平台 Go API | Worker | Admin/Web | 数据与解析 | 验收重点 |
| --- | --- | --- | --- | --- | --- |
| P1 | automation API 骨架 | 无真实浏览器依赖 | Admin 调试入口 | migration 草案 | job/run/artifact 可表达 |
| P2 | mock job 下发和状态接收 | runtime + mock adapter | run 状态可见 | 无 | 端到端生命周期 |
| P3 | artifact 和 manual action | 搜索平台 Playwright | 文献任务可观测 | PDF 待解析 | 浏览器采集闭环 |
| P4 | 解析状态管理 | 上传原始 artifact | 解析结果可见 | 页面解析与结构化 | 平台处理文献数据 |
| P5 | Admin 查询和审计 | 可撤销、可中止 | 设备/任务/人工动作页面 | 错误可追溯 | 运营可排障 |
| P6 | 用户权限和配额 | 用户设备绑定 | Web 用户入口 | 结果展示 | 用户可自助使用 |
| P7 | social job payload | YouTube/TikTok adapter | 内部 PoC 入口 | 上传 artifact | 验证通用自动化 |
| P8 | Agent policy | Browser Agent tools | 风险动作确认 | 动作审计 | 可控 Agent 化 |

## 十七、关键风险

1. 平台 schema 过早绑定 搜索平台，会限制 YouTube/TikTok 等后续任务。
2. Worker 直接保存或上传第三方账号密码，会带来安全和合规风险。
3. 没有 Admin 可观测性，会导致浏览器任务失败后无法定位。
4. 页面解析如果下沉到本机 Worker，会破坏平台数据一致性和可追溯性。
5. 没有人工确认机制时，自动发布、删除、授权等动作风险过高。
6. 过早引入通用 Agent，容易在稳定性和安全边界上失控。

## 十八、当前下一步

P1/P2 首版代码骨架、P3 前置第一批代码和 P3 浏览器 adapter 骨架已完成，当前实现状态：

1. `backend-api` 已有可编译的标准库 HTTP 服务骨架。
2. 平台侧已有内存版 worker device、pairing、automation job/run/checkpoint/artifact/manual action repository。
3. Worker 已从旧 mock job loop 调整为 adapter runtime 调度。
4. P2 mock adapter 已能验证 success、failure、unsupported adapter、capability mismatch 和 policy allowed domains。
5. 数据库基线和 migration 已覆盖 `worker_*`、`automation_*` 和 `qiyuan_literature_result`。
6. `deploy-local` 已支持本机启动 `backend-api` 和运行 automation mock smoke test。
7. Worker `doctor` 已能检查 browser profile/downloads 目录和 Playwright 包状态。
8. Worker BrowserRuntime 已有 Playwright persistent context 打开接口，并已用本地 `data:` 页面验证 headless Chromium 可启动。
9. `generic.browser.agent` adapter 已能在 fake page 测试中完成搜索页 HTML 解析、PDF 链接识别和 HTML/screenshot/metadata artifact 采集。
10. `backend-api` 已引入 Redis/no-op locker 抽象，Redis 配置后会用 `automation:jobs:claim` 短租约锁保护 job claim。
11. Admin 调试接口已能查询 job、run、run artifacts 和 run manual actions。
12. `backend-api` 已有 MySQL repository 首版，配置 `MYSQL_DSN` 后可持久化 worker device、pairing、automation job/run/checkpoint/artifact/manual action。
13. `deploy-local` 已有 MySQL/Redis infra compose、schema apply 脚本和 MySQL/Redis 持久化 smoke test，已验证持久化模式下 pairing、heartbeat、job claim、checkpoint、artifact、manual action resolve、列表查询和设备撤销。
14. Admin 调试接口已补齐 job list、run list、manual action list/resolve、worker device list/revoke。
15. `generic.browser.agent` adapter 已支持从 PDF 链接进行受控下载，并将下载文件登记为 `pdf` artifact；fake page 测试已覆盖 产物采集路径。
16. `frontend-admin` 已有 Automation 控制台首版，可查看 jobs/runs/devices/manual actions/artifacts，并支持创建 mock job、resolve manual action、revoke device。
17. 平台已具备 P4 后端最小闭环：PDF artifact 自动进入 `qiyuan_literature_result` 解析队列，内部 PDF 服务可领取任务并写回 `parsed/failed` 状态，Admin API 可查询和重试。
18. Admin 已新增 Literature 结果页和 artifact 下载入口，可直接观察 页面解析状态并触发重试。
19. Web 已有本地用户入口首版，可提交 `generic.browser.agent` 版权检测任务并查看 jobs/results；本地启动脚本与 Admin/API 一样使用 tmux。
20. Admin/Web/Internal/Worker 已有初步接口边界：Worker 继续使用 device token，Internal 使用 `X-Internal-Secret`，Admin/Web 可配置独立 token。
21. 真实 搜索平台 产物采集已完成一次实战验证：headed Worker run 抓取 5 条结果、发现 1 个 PDF 链接、下载并上传 1 个 4.99MB PDF artifact；headless 模式仍可能触发 Google 验证码，需要 manual action。
22. 搜索平台 manual action 已支持“平台 resolved 后恢复”：Worker 会在拦截页上传 HTML、截图和 error context，等待用户在本地浏览器处理验证码/登录，再检查同一页面是否解除拦截并继续后续结果解析。
23. `generic.browser_agent` 首版已接入 Worker registry/capability，支持受 policy 限制的页面观察、搜索输入填充、提交、结果抽取、trace 和最终截图。
24. 本地 Browser Agent demo 已通过 `deploy-local/integration-test/30-browser-agent-demo.sh` 验收：创建 `generic.browser.agent` job，Worker 控制 Chromium 打开本地 fixture，完成搜索并上传 `agent_trace` 与 `screenshot` artifact。

建议下一步继续补齐用户与运营入口：

1. 增强 `generic.browser.agent` adapter 的翻页、结果去重、失败 PDF 重试和更稳定的 PDF 文件命名。
2. 把 Browser Agent 从搜索 PoC 扩展到可配置 action plan：click、fill、extract、download、manual confirmation 和每步 checkpoint。
3. 继续增强 Web 用户入口的设备绑定状态、用户权限、取消任务、错误文案和结果详情。
4. 把当前 token guard 升级为真实账号、角色、租户和资源 ownership 校验。
5. 在现有本地 `ARTIFACT_DIR` 上传/下载接口基础上，后续抽象 storage backend，切换到对象存储和短期预览 URL。
