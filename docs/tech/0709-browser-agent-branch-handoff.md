# Browser Agent 项目工作交接文档

## Changelog

- 2026-07-19：同步线上客户交付路线，将账号租户隔离、生产部署、对象存储和 Worker 客户端交付提升为上线阻断项，并增加生产化技术方案索引。
- 2026-07-19：仓库收敛为单一 Browser Agent 项目，`main` 为唯一长期主线；同步单环境启动方式、Windows Web → Worker → Admin 端到端验收和后续优先级。
- 2026-07-09：首次整理 Browser Agent 交接说明，覆盖项目定位、当前实现状态、本地启动、核心代码地图、任务链路、已落地 adapter、验证命令、风险边界和后续待办。

---

## 一、交接范围

本文面向后续接手 Browser Agent 的研发、运维或编码 Agent，用于快速理解业务定位、代码入口、运行方式和风险边界。仓库只维护 Browser Agent 项目，`main` 是唯一长期开发主线。

项目当前定位是 **通用 Browser Agent 底座 + 泛互联网自动化运营平台**，主要承载：

1. 通用 Automation 平台：设备绑定、任务调度、run、checkpoint、artifact、manual action、trace。
2. 本地 Browser Worker：用户本机可信执行节点，控制真实 Chromium / Chrome 执行任务。
3. 通用浏览器 Agent：`generic.browser_agent`、`browser.act` 等网页操作能力。
4. 泛运营 adapter：短剧版权检索取证、社媒视频上传、微信群资料同步等。

通用底座与业务能力通过 extension、adapter 和 policy 分层，不再通过长期业务分支隔离。历史 Python 包名、命令名和数据库标识仍可能保留旧命名；它们属于兼容标识，不能据此推断仓库仍存在另一个产品或分支。

---

## 二、当前状态概览

| 模块 | 当前状态 | 主要说明 |
|------|----------|----------|
| 本地部署 | 已有脚本 | 单一 Browser Agent 环境；Windows 使用 PowerShell，Linux/macOS 的 API/Web/Admin/Worker 通过 tmux 管理 |
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

启动脚本会优先加载 `deploy-local/.env.browser-agent`；如需使用其他本地配置文件，可设置 `BROWSER_AGENT_ENV_FILE`：

```bash
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

默认配置使用以下端口；真实端口仍以启动脚本输出和 `deploy-local/run/*.port` 为准。

| 服务 | 默认说明 |
|------|----------|
| Web | `http://localhost:24001` |
| Admin | `http://localhost:26174` |
| Go API | `http://localhost:29001` |
| MySQL | `127.0.0.1:24307` |
| Redis | `127.0.0.1:27380` |

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
| `README.md` | 项目事实入口、本地启动、目录归属、文档治理 |
| `docs/brd/0627-browser-agent-automation-brd.md` | Browser Agent 商业场景扩展和模块解耦 BRD |
| `docs/tech/0617-local-automation-worker-architecture.md` | 通用本地 Automation Worker 架构 |
| `docs/tech/0617-local-automation-worker-api-schema.md` | `automation_*` 数据模型、Worker API、人工动作、artifact、错误码 |
| `docs/tech/0617-local-automation-worker-implementation-plan.md` | Worker 子计划 |
| `docs/tech/0617-local-automation-platform-iteration-plan.md` | 平台级总计划与历史落地记录 |
| `docs/tech/0619-llm-browser-agent-integration-plan.md` | LLM Browser Agent 接入计划和未完成清单 |
| `docs/tech/0702-weixin-group-file-sync-agent-plan.md` | 微信群资料同步 Agent 技术方案 |
| `docs/tech/0719-browser-agent-productionization-plan.md` | 线上客户交付架构、账号租户、部署、数据、Worker、安全、监控和发布门禁 |

---

## 七、接手后的优先待办

### P0 / 近期

- 建立账号、租户、membership 和 RBAC，为 Worker device、job、run、artifact、manual action 强制 resource ownership，并补跨租户越权测试。
- 统一 Gateway 与 Automation API 的生产鉴权链路；共享 Admin/Web token 只保留为本地开发或受控诊断能力。
- 建立 staging/production 数据与部署基线：托管 MySQL/Redis、对象存储、Secret Manager、TLS、migration、备份恢复、不可变镜像和回滚。
- 修复现有生产 Compose/K3s 草案与当前应用在环境变量、健康检查、端口、路由和持久化方面的不一致。

已于 2026-07-19 完成 Windows 干净环境启动和 Web → Worker → Admin 验收；`deploy-local/integration-test/30-browser-agent-windows.ps1` 会创建确定性 Browser Agent 任务并校验 trace、截图。

### P1 / 中期

- 将本机 Worker 产品化为可签名安装、系统凭据存储、开机启动、版本兼容、受控升级和日志导出的 Windows 客户端。
- 定义版权检索的作品、检索条件、候选线索和取证 artifact 最小模型，跑通首个真实业务闭环。
- 为社媒自动发布能力补安全确认：哪些平台允许自动发布、哪些场景必须人工确认、如何记录审计。
- 为 TikTok / Instagram / YouTube / Douyin 各自补稳定的 fixture 单测和手工验证清单。
- 完善 social upload 的错误码、登录状态识别、重试边界和人工接管体验。
- 将 `manual_publish_required`、平台 visibility、账号 profile、allowed domains 做成更明确的 policy template。
- 完善 Generic Browser Agent 的 trace UI、prompt/token 成本审计和 provider 计量。
- 微信同步进入第二阶段：通过桌面 UI 自动化打开群文件、触发下载，再复用本地扫描链路。

### P2 / 封闭内测与商用准备

- 为 artifact 增加留存、删除、脱敏、敏感文件扫描和权限控制。
- 为 Worker 增加离线告警、run 超时、任务堆积、失败率和平台登录异常监控。
- 梳理真实平台自动化合规边界，尤其是社媒发帖、平台条款、版权取证和微信资料同步。
- 拆分产品线 package 或 extension 发布机制，减少所有 adapter 都进入同一个 Worker 安装包。
- 邀请少量明确授权的客户进行封闭内测，验证跨租户隔离、安装升级、任务成功率、人工接管率和单位任务成本后再评估开放注册。

完整生产化阶段、技术边界和验收门禁见 `docs/tech/0719-browser-agent-productionization-plan.md`。在 P0 完成前，不得把当前 Admin/API 直接暴露到公网。

---

## 八、接手注意事项

1. 先确认分支：`main` 是唯一长期开发主线；短期功能分支验收后应合回并删除。
2. 先读 `README.md`、本文和 `docs/brd/0627-browser-agent-automation-brd.md`，再进入具体模块。
3. 新业务通过 extension、adapter 和 policy 接入，不要把平台特例写进通用浏览器运行时。
4. 不要绕过 go-api 写 MySQL；Worker 只通过 API 上报状态、artifact 和 manual action。
5. 不要上传或记录第三方平台账号密码、Cookie、Session 明文。
6. 真实社媒/微信任务涉及用户账号和外部平台，改动后需要说明验证账号、平台状态、是否执行发布。
7. 不要在前端目录新增 `.env*`；环境变量归口 `deploy-local` / 部署目录。
8. 提交前用 `git status --short`、`git diff --stat`、`git diff --cached --stat` 复核；只暂存本次相关文件。

---

## 九、建议接手检查清单

- [ ] 当前工作基于 `main` 或准备合回 `main` 的短期功能分支。
- [ ] 工作区没有用户未提交改动，或已明确哪些改动不能触碰。
- [ ] 使用 `deploy-local/.env.browser-agent` 启动本地服务。
- [ ] Web 和 Admin 均可访问，真实端口已从 `deploy-local/run/*.port` 确认。
- [ ] Worker 完成 `init` / `pair` / `doctor`，能力上报包含目标 adapter。
- [ ] 跑通 `10-automation-mock.sh` 和 `20-automation-mysql-smoke.sh`。
- [ ] 从 Web 创建至少一个 Browser Agent 或 mock 类任务，Admin 能看到 run、trace、artifact。
- [ ] 对社媒/微信任务，确认账号、文件、发布动作和授权边界后再做真实验证。
