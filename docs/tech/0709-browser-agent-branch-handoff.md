# Browser Agent 分支项目工作交接文档

## Changelog

- 2026-07-19：同步 `feature/browser-agent` 已合入 `main` 的现状，记录 Windows Web → Worker → Admin 端到端验收脚本和已完成的干净环境验证。
- 2026-07-09：首次整理 `feature/browser-agent` 分支交接说明，覆盖分支定位、当前实现状态、本地启动、核心代码地图、任务链路、已落地 adapter、验证命令、风险边界和后续待办。

---

## 一、交接范围

本文面向后续接手 Browser Agent 主线的研发、运维或编码 Agent，用于快速理解业务定位、代码入口、运行方式和风险边界。`feature/browser-agent` 的实现已合入 `main`，新工作应以当前任务指定的目标分支为准。

`feature/browser-agent` 当前定位是 **通用 Browser Agent 底座 + 泛互联网自动化运营分支**，主要承载：

1. 通用 Automation 平台：设备绑定、任务调度、run、checkpoint、artifact、manual action、trace。
2. 本地 Browser Worker：用户本机可信执行节点，控制真实 Chromium / Chrome 执行任务。
3. 通用浏览器 Agent：`generic.browser_agent`、`browser.act` 等网页操作能力。
4. 泛运营 adapter：短剧版权检索取证、社媒视频上传、微信群资料同步等。

与 `feature/qiyuan` 的边界：

- `feature/qiyuan` 专注 AI for Science、文献检索、PDF 解析、知识抽取、Neo4j/MCP。
- `feature/browser-agent` 专注通用自动化执行、社媒/内容运营、版权取证、微信资料同步。
- 两个分支共享 Playwright、Worker 调度、artifact、manual action 等底层能力，底座修复可通过 cherry-pick 互相同步。
- 本分支仍有部分历史文档和代码命名保留 `QIYUAN` 字样，接手时应按当前 README 的分支解耦策略理解，不要把 AI 4 Science 业务继续扩进本分支。

---

## 二、当前状态概览

| 模块 | 当前状态 | 主要说明 |
|------|----------|----------|
| 本地部署 | 已有脚本 | 支持 `QIYUAN_ENV=browser-agent` 多环境隔离；API/Web/Admin/Worker 通过 tmux 管理 |
| go-api | 已实现 | Automation job/run/artifact/manual action/trace、Worker 配对、Web/Admin 接口 |
| Worker CLI | 已实现 | Python 3.12，本机运行，支持设备绑定、能力上报、任务领取、Playwright 执行、artifact 上传 |
| Generic Browser Agent | 已实现首版 | 支持受 policy 约束的 observe、click、fill、extract、screenshot、trace、LLM planner |
| Browser Act | 已接入 | `generic.browser.act` / `browser.act` 作为浏览器动作型 adapter |
| 社媒上传 | 已实现多平台 adapter | Douyin、YouTube、TikTok、Instagram；近期重点修复 Instagram/TikTok 登录与发布流程 |
| 微信资料同步 | 已实现 MVP | `weixin.desktop_sync` 扫描本机微信下载/附件目录，上传文件 artifact 和 manifest |
| Web | 已实现业务入口 | 版权检索、社媒运营/上传、微信群同步、本地 Worker 引导、任务与 artifact 查看 |
| Admin | 已实现运维入口 | Jobs、Runs、Devices、Manual Actions、Run trace、artifact 下载、cancel/revoke |
| 验证脚本 | 已覆盖基础与 Windows 浏览器链路 | `10-automation-mock.sh`、`20-automation-mysql-smoke.sh`、`30-browser-agent-windows.ps1` |

---

## 三、本地启动与环境

### 3.1 推荐启动顺序

建议显式指定环境，避免与 `feature/qiyuan` 的本地数据、端口和 Worker 凭证混用：

```bash
export QIYUAN_ENV=browser-agent

bash deploy-local/tools/run-infra-local.sh start
bash deploy-local/tools/db-apply.sh all

bash deploy-local/tools/run-api-host-local.sh start
bash deploy-local/tools/run-admin-host-local.sh start
bash deploy-local/tools/run-web-host-local.sh start

bash deploy-local/tools/run-worker-host-local.sh init
bash deploy-local/tools/run-worker-host-local.sh pair
bash deploy-local/tools/run-worker-host-local.sh start
```

常用状态命令：

```bash
bash deploy-local/tools/run-api-host-local.sh status
bash deploy-local/tools/run-admin-host-local.sh status
bash deploy-local/tools/run-web-host-local.sh status
bash deploy-local/tools/run-worker-host-local.sh status
bash deploy-local/tools/run-worker-host-local.sh doctor
```

### 3.2 默认端口

README 中仍列出默认端口基线；`deploy-local/guide.md` 说明 `browser-agent` 环境有独立端口偏移，真实端口以启动脚本输出和 `deploy-local/run/*.port` 为准。

| 服务 | 默认说明 |
|------|----------|
| Web | `deploy-local/run/frontend-web.port` 记录真实端口 |
| Admin | `deploy-local/run/frontend-admin.port` 记录真实端口 |
| Go API | 默认配置来自 `deploy-local/.env` 或 `.env.browser-agent` |
| MySQL / Redis | 由 `run-infra-local.sh` 按 `QIYUAN_ENV` 隔离启动 |

### 3.3 常用验证

```bash
bash deploy-local/integration-test/10-automation-mock.sh
bash deploy-local/integration-test/20-automation-mysql-smoke.sh
```

说明：

- 只改文档时不需要启动服务或跑集成测试。
- 修改 `backend-api` 或 `backend-gateway` 后，按 README 要求运行 `bash deploy-local/tools/reload-go-local.sh`。
- 修改 Worker 后，优先跑 `worker/local-cli` 下相关 pytest；涉及真实平台登录和上传时，必须说明手工验证环境与账号状态。
- 修改前端时，不要在同一工作区同时运行 `next dev` 和 `next build`；优先复用脚本启动的 Web/Admin 端口做页面验证。

---

## 四、核心代码地图

### 4.1 go-api：Automation 控制面

| 路径 | 说明 |
|------|------|
| `backend-api/internal/handler/automation/handler.go` | Admin/Web/Worker Automation API、policy templates、artifact 下载、trace、manual action |
| `backend-api/internal/engine/automation/` | Job/run/checkpoint/artifact/manual action 业务编排 |
| `backend-api/internal/repository/automation/` | MySQL / memory repository |
| `backend-api/internal/model/automation/` | Automation 数据模型和 Web DTO |
| `backend-api/internal/handler/worker/` | Worker 设备配对、心跳、任务领取入口 |
| `backend-api/internal/engine/worker/` | Worker 设备和 token 生命周期 |

关键接口：

- Web 创建任务：`POST /web/automation/browser-agent-jobs`
- Web 创建 Browser Act 任务：`POST /web/automation/browser-act-jobs`
- Web 创建社媒上传任务：`POST /web/automation/social-upload-jobs`
- Web 创建微信同步任务：`POST /web/automation/weixin-desktop-sync-jobs`
- Web 版权取证报告：`GET /web/automation/reports/copyright-evidence`
- Admin policy 模板：`GET /admin/automation/policy-templates`
- Admin run trace：`GET /admin/automation/runs/{run_id}/trace`
- Admin artifact 下载：`GET /admin/automation/artifacts/{artifact_id}/download`
- Worker 人工动作：`POST /worker/automation/runs/{run_id}/manual-actions`

### 4.2 Worker：本地执行节点

| 路径 | 说明 |
|------|------|
| `worker/local-cli/qiyuan_worker/cli.py` | CLI 命令入口 |
| `worker/local-cli/qiyuan_worker/job_loop.py` | 常驻轮询和任务执行循环 |
| `worker/local-cli/qiyuan_worker/runtime/runner.py` | JobRunner 主执行逻辑 |
| `worker/local-cli/qiyuan_worker/runtime/policy.py` | 任务策略约束 |
| `worker/local-cli/qiyuan_worker/browser/runtime.py` | Playwright 浏览器运行时 |
| `worker/local-cli/qiyuan_worker/artifacts/collector.py` | 本地 artifact 收集和上传准备 |
| `worker/local-cli/qiyuan_worker/http_client.py` | Worker 与 go-api 通信 |
| `worker/local-cli/qiyuan_worker/builtin_extensions.py` | 内置产品线、adapter、能力和 policy template 注册 |

当前内置能力：

- `core.mock`：`mock.echo`
- `browser_agent.generic`：`generic.browser_agent`
- `browser_agent.browser_act`：`browser.act`
- `manual.upload`：`manual`
- `social.upload`：Douyin / YouTube / TikTok / Instagram 视频上传
- `weixin.desktop_sync`：本机微信资料同步

### 4.3 Agent 与 adapter

| 路径 | 说明 |
|------|------|
| `worker/local-cli/qiyuan_worker/agent/` | planner、action schema、executor、tools、observer、trace、redaction、LLM provider |
| `worker/local-cli/qiyuan_worker/adapters/generic/browser_agent.py` | 通用 Browser Agent adapter |
| `worker/local-cli/qiyuan_worker/adapters/browser_act.py` | Browser Act adapter |
| `worker/local-cli/qiyuan_worker/adapters/social/upload.py` | 社媒上传编排器，负责五步协议和失败转人工动作 |
| `worker/local-cli/qiyuan_worker/adapters/social/base.py` | 平台 uploader 抽象：navigate、upload_file、fill_metadata、set_visibility、submit |
| `worker/local-cli/qiyuan_worker/adapters/social/douyin.py` | 抖音上传实现 |
| `worker/local-cli/qiyuan_worker/adapters/social/youtube.py` | YouTube 上传实现 |
| `worker/local-cli/qiyuan_worker/adapters/social/tiktok.py` | TikTok 上传实现 |
| `worker/local-cli/qiyuan_worker/adapters/social/instagram.py` | Instagram Reels 上传实现 |
| `worker/local-cli/qiyuan_worker/adapters/weixin/desktop_sync.py` | 微信本机目录扫描与 manifest 生成 |
| `worker/local-cli/qiyuan_worker/adapters/manual/upload.py` | 人工上传 adapter |

### 4.4 前端

| 路径 | 说明 |
|------|------|
| `frontend-web/src/app/page.tsx` | 用户入口聚合页 |
| `frontend-web/src/components/CopyrightDetectionForm.tsx` | 版权检索 / Browser Act 任务入口 |
| `frontend-web/src/components/SocialMediaOpsForm.tsx` | 通用 Browser Agent 任务入口 |
| `frontend-web/src/components/SocialUploadForm.tsx` | 社媒视频上传入口 |
| `frontend-web/src/components/WeixinSyncPanel.tsx` | 微信群资料同步入口 |
| `frontend-web/src/components/JobsTable.tsx` | 用户侧任务、run、artifact、trace 查看 |
| `frontend-web/src/components/CopyrightEvidenceReport.tsx` | 版权取证报告 |
| `frontend-admin/src/app/runs/page.tsx` | Admin run 详情、trace、artifact、manual action、cancel |
| `frontend-admin/src/app/manual/page.tsx` | 人工动作处理 |
| `frontend-admin/src/app/devices/page.tsx` | Worker 设备管理与 revoke |

---

## 五、关键任务链路

### 5.1 通用 Automation 生命周期

```text
Web/Admin 创建 automation_job
  -> Worker 上报 capability 并领取匹配任务
  -> go-api 创建 automation_run
  -> Worker 执行 adapter
  -> checkpoint / trace / artifact / manual_action 持续回传
  -> run completed / failed / needs_manual_action / cancelled
  -> Web/Admin 查询结果、下载 artifact、处理人工动作
```

重点对象：

- `worker_device`：设备绑定、token hash、能力、状态、最后心跳。
- `automation_job`：任务类型、adapter、target、input、policy、cursor。
- `automation_run`：一次执行实例，承载设备、状态、开始/结束时间、错误。
- `automation_checkpoint`：阶段性进度、trace 摘要、断点游标。
- `automation_artifact`：截图、trace、视频、manifest、上传文件等产物。
- `automation_manual_action`：登录、验证码、安全验证、上传失败后人工接管。

### 5.2 Generic Browser Agent

```text
Web /web/automation/browser-agent-jobs
  -> job_type=generic.browser.agent, adapter=generic.browser_agent
  -> Worker 打开 Chromium
  -> observe 页面、执行 planner/action、截图、抽取数据
  -> 生成 agent_trace / screenshot artifact
  -> Web/Admin 展示 trace 和结果摘要
```

关键边界：

- LLM 只能输出 JSON action plan，Worker 必须做 schema 校验。
- 所有动作经过 policy gate，受 `allowed_domains`、`allowed_actions`、timeout 等约束。
- prompt、响应、页面观察和截图应做敏感信息最小化或脱敏。
- 发布、付款、删除、授权、外部上传等高风险动作必须转 `manual_action` 或由业务 adapter 明确控制。

### 5.3 社媒视频上传

```text
Web /web/automation/social-upload-jobs
  -> social.<platform>.upload_video
  -> Worker 使用持久浏览器 profile 打开平台上传页
  -> navigate -> upload_file -> fill_metadata -> set_visibility -> submit
  -> 失败或登录阻断时创建 manual_action 并保留截图
  -> 成功后上传 final screenshot artifact
```

当前平台：

- `social.douyin.upload_video`
- `social.youtube.upload_video`
- `social.tiktok.upload_video`
- `social.instagram.upload_video`

近期实现现状：

- 社媒上传 policy 当前默认 `manual_publish_required=false`，也就是具备自动发布路径。
- TikTok / Instagram 近期重点修复登录等待、创建入口、裁剪页继续按钮、Reels 提示弹窗和发布按钮识别。
- 真实平台 DOM 变化频繁，任何 selector 调整都需要配套单测和手工验证说明。

### 5.4 微信资料同步

```text
Web /web/automation/weixin-desktop-sync-jobs
  -> job_type=weixin.desktop_sync, adapter=weixin.desktop_sync
  -> Worker 扫描本机微信下载/附件目录
  -> 按群名/关键词、mtime、文件类型、大小过滤
  -> 拷贝到 Worker job work_dir
  -> 上传 weixin_file artifact 和 weixin_manifest artifact
```

当前边界：

- 只扫描本机已落盘文件，不读取微信聊天数据库。
- 不直接写业务资料库，资料入库应后续通过 go-api 受控入口处理。
- 群文件可能包含隐私和版权资料，生产化前必须补授权、权限、删除和审计机制。

---

## 六、重要文档索引

| 文档 | 用途 |
|------|------|
| `README.md` | 当前分支事实入口、本地启动、目录归属、文档治理 |
| `docs/brd/0627-browser-agent-automation-brd.md` | Browser Agent 商业场景扩展和分支解耦 BRD |
| `docs/tech/0617-local-automation-worker-architecture.md` | 通用本地 Automation Worker 架构 |
| `docs/tech/0617-local-automation-worker-api-schema.md` | `automation_*` 数据模型、Worker API、人工动作、artifact、错误码 |
| `docs/tech/0617-local-automation-worker-implementation-plan.md` | Worker 子计划 |
| `docs/tech/0617-local-automation-platform-iteration-plan.md` | 平台级总计划与历史落地记录 |
| `docs/tech/0619-llm-browser-agent-integration-plan.md` | LLM Browser Agent 接入计划和未完成清单 |
| `docs/tech/0702-weixin-group-file-sync-agent-plan.md` | 微信群资料同步 Agent 技术方案 |
| `docs/tech/0626-web-operation-manual.md` | Web 操作手册，部分 QIYUAN/文献内容需后续清理 |
| `docs/tech/0626-admin-operation-manual.md` | Admin 操作手册，部分任务类型描述需后续同步 |

---

## 七、接手后的优先待办

### P0 / 近期

- 清理或标注文档中的旧 QIYUAN/文献/知识图谱表述，避免和 `feature/qiyuan` 职责混淆。
- 为社媒自动发布能力补安全确认：哪些平台允许自动发布、哪些场景必须人工确认、如何记录审计。

已于 2026-07-19 完成 Windows 干净环境启动和 Web → Worker → Admin 验收；`deploy-local/integration-test/30-browser-agent-windows.ps1` 会创建确定性 Browser Agent 任务并校验 trace、截图。

### P1 / 中期

- 为 TikTok / Instagram / YouTube / Douyin 各自补稳定的 fixture 单测和手工验证清单。
- 完善 social upload 的错误码、登录状态识别、重试边界和人工接管体验。
- 将 `manual_publish_required`、平台 visibility、账号 profile、allowed domains 做成更明确的 policy template。
- 完善 Generic Browser Agent 的 trace UI、prompt/token 成本审计和 provider 计量。
- 微信同步进入第二阶段：通过桌面 UI 自动化打开群文件、触发下载，再复用本地扫描链路。

### P2 / 生产化

- 建立账号/租户/resource ownership，避免一个用户查看或下载另一个用户的 artifact。
- 为 artifact 增加留存、删除、脱敏、敏感文件扫描和权限控制。
- 为 Worker 增加离线告警、run 超时、任务堆积、失败率和平台登录异常监控。
- 梳理真实平台自动化合规边界，尤其是社媒发帖、平台条款、版权取证和微信资料同步。
- 拆分产品线 package 或 extension 发布机制，减少所有 adapter 都进入同一个 Worker 安装包。

---

## 八、接手注意事项

1. 先确认分支：Browser Agent 已合入 `main`，以当前任务约定的目标分支为准，不再强制要求 `feature/browser-agent`。
2. 先读 `README.md`、本文和 `docs/brd/0627-browser-agent-automation-brd.md`，再进入具体模块。
3. 不要把 AI 4 Science / 文献解析 / Neo4j / MCP 新需求继续扩进本分支。
4. 不要绕过 go-api 写 MySQL；Worker 只通过 API 上报状态、artifact 和 manual action。
5. 不要上传或记录第三方平台账号密码、Cookie、Session 明文。
6. 真实社媒/微信任务涉及用户账号和外部平台，改动后需要说明验证账号、平台状态、是否执行发布。
7. 不要在前端目录新增 `.env*`；环境变量归口 `deploy-local` / 部署目录。
8. 提交前用 `git status --short`、`git diff --stat`、`git diff --cached --stat` 复核；只暂存本次相关文件。

---

## 九、建议接手检查清单

- [ ] 当前分支符合本次任务约定，且不是误在其他产品分支修改。
- [ ] 工作区没有用户未提交改动，或已明确哪些改动不能触碰。
- [ ] `export QIYUAN_ENV=browser-agent` 后启动本地服务。
- [ ] Web 和 Admin 均可访问，真实端口已从 `deploy-local/run/*.port` 确认。
- [ ] Worker 完成 `init` / `pair` / `doctor`，能力上报包含目标 adapter。
- [ ] 跑通 `10-automation-mock.sh` 和 `20-automation-mysql-smoke.sh`。
- [ ] 从 Web 创建至少一个 Browser Agent 或 mock 类任务，Admin 能看到 run、trace、artifact。
- [ ] 对社媒/微信任务，确认账号、文件、发布动作和授权边界后再做真实验证。
