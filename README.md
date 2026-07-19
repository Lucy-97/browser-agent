# Browser Agent 平台

本项目是用于浏览器自动化执行与代理控制的基础底座平台。

## Changelog

- 2026-07-19：完成生产化 Phase 1 客户身份主链路：邮箱密码注册/登录、租户 owner 创建、JWT + HttpOnly Cookie、active membership 校验、登录限流、登录后 Worker 配对 UI 和本地 Web → Gateway → API 验收；成员管理仍留待封闭内测运营能力补齐。
- 2026-07-19：新增线上客户交付与生产化技术方案，明确首期采用“云端控制面 + 客户本机 Worker”，并将账号租户隔离、生产部署和 Worker 交付列为上线门禁。
- 2026-07-19：仓库收敛为单一 Browser Agent 项目，统一以 `main` 为开发主线，移除旧业务分支和多环境切换说明。

## 一、 项目入口与 Agent 协作

- `README.md` 是项目事实入口：维护架构、目录归属、数据库 ownership、文档治理规则和文档 Roadmap。
- `AGENTS.md` 是通用 agent 行为入口：维护 Codex、Claude、Gemini 等编码 agent 都应遵守的协作、编码、安全和验证习惯。

### 单一主线与模块边界

本仓库只维护 Browser Agent 项目，`main` 是唯一长期开发主线。通用能力、版权检索取证、社媒运营和微信资料同步通过 Worker extension、adapter、policy 与 API 边界隔离，不再通过长期业务分支拆分产品。

功能开发应优先保持底座与业务 adapter 解耦；实验性工作可使用短期功能分支，验证后合回 `main` 并删除，不再维护平行产品分支。

## 二、 本地开发启动

本地开发统一使用 Docker Compose；完整说明见 [deploy-local/guide.md](deploy-local/guide.md)，包括 Agent 进程沙箱避坑、共享基础设施、Go 本机编译、日志、健康检查和 Admin 账号说明。

### 快速启动：

```bash
bash deploy-local/tools/run-infra-local.sh start
bash deploy-local/tools/db-apply.sh all
bash deploy-local/tools/run-api-host-local.sh start
bash deploy-local/tools/run-gateway-host-local.sh start
bash deploy-local/tools/run-admin-host-local.sh start
bash deploy-local/tools/run-web-host-local.sh start
bash deploy-local/tools/run-worker-host-local.sh init
bash deploy-local/tools/run-worker-host-local.sh pair
bash deploy-local/tools/run-worker-host-local.sh start
bash deploy-local/integration-test/10-automation-mock.sh
bash deploy-local/integration-test/20-automation-mysql-smoke.sh
```

`run-infra-local.sh` 默认启动 MySQL 和 Redis。Elasticsearch 仍未接入当前 `deploy-local` infra compose。

Linux/macOS 上，前端和本机 API 默认在宿主机通过 tmux 启动。前端脚本支持 `start`、`restart`、`stop`、`status`；无参数等价于 `start`。首次使用前需确保宿主机已安装 tmux，例如 `brew install tmux`。Windows 使用 `deploy-local/tools/run-platform-windows.ps1` 和 `run-worker-windows.ps1`，详见本地开发指南。

### Browser Agent 本地默认端口

| 服务 | 宿主机地址 |
| --- | --- |
| Web | `http://localhost:24001` |
| Admin | `http://localhost:26174` |
| Gateway | `http://localhost:29000` |
| Go API | `http://localhost:29001` |
| MySQL | `127.0.0.1:24307` |
| Redis | `127.0.0.1:27380` |

### 线上客户交付目标

当前仓库已完成本地 Web → API → Worker → Admin 基础链路验证，但现有 Compose/K3s 文件和可选共享 token 仍是开发或部署骨架，不能直接作为多客户生产环境。首期线上形态统一采用“云端 Web/Admin/Gateway/API/数据服务 + 客户本机 Worker”：第三方平台登录态和浏览器 profile 默认留在客户设备，Worker 仅通过出站 HTTPS 访问云端。

生产化的实施阶段、信任边界、账号租户模型、对象存储、Worker 安装升级、监控告警和上线门禁见 [Browser Agent 线上客户交付与生产化技术方案](docs/tech/0719-browser-agent-productionization-plan.md)。客户身份与租户隔离已在本地链路落地；在 Phase 2 的生产部署、Secret、对象存储、备份和 staging 门禁完成前，仍不得将当前 Admin/API 直接暴露到公网。

## 三、 核心微服务架构

后端采用“职责解耦”的异构设计，最大化发挥不同语言的生态优势：

*   **go-gateway — 流量网关**
    *   统一请求入口（基于 Gin）。已接通 JWT/HttpOnly Cookie 验证、可信租户身份注入、Redis 全局与登录路径限流、CORS、指标和 `/web/*`、`/worker/*` 路由；生产域名、TLS/WAF 与 staging 部署属于 Phase 2。
*   **go-api — 核心业务中枢**
    *   严密的三层架构（Handler → Engine → Repo）。
    *   **唯一写权限节点**：全系统只有 go-api 有权对 MySQL 数据库执行 `INSERT/UPDATE/DELETE`。当前已承载账号登录、租户 membership、Automation、Worker、artifact 和 Admin/Web 接口；支付与成员邀请等客户化能力属于后续边界。
    *   对内部（Python）暴露 `X-Internal-Secret` 鉴权的 `/internal/*` 端点处理强一致性写操作。

### 代码目录归属(TODO)

*   `backend-gateway`：Go API 网关，负责 JWT、限流、CORS、SSE 透传。
*   `backend-api`：Go 业务主服务，采用 Handler → Engine → Repository 分层，是用户、支付、业务数据的唯一写入节点。
*   `worker/local-cli`：本地 Browser Worker CLI，使用 Python 3.12，负责设备绑定、领取任务、控制本机 Chromium、上传任务 artifact；不得保存或上传第三方平台账号密码、Cookie 或 Session 明文。
*   `frontend-web`：Next.js 15 App Router，Cloudflare Pages 部署目标。
*   `frontend-admin`：Next.js App Router 管理后台。

### 数据库 ownership 与迁移

*   Go API 管理 `user*`、`verify_code` 等业务写入。
*   跨服务强一致写操作使用 `X-Internal-Secret` 和 `/internal/*`。
*   `database/init.sql` 只作为新库基线；任何新增或调整字段、索引、表的变更，必须同步新增或更新 `database/migrations/` 下的幂等 migration，确保已有环境可增量升级。

## 四、 前端

*   **Web 端 (`frontend-web`)**
    *   Next.js 15 (App Router) + TailwindCSS 4。
    *   优先 SSR 提升 SEO 与首屏性能，打包后通过 Cloudflare Pages 部署于边缘网络。
*   **Admin 控制台 (`frontend-admin`)**
    *   Next.js App Router。内部运营管理与可视化测试面板。所有请求走 `/admin/*` 路径，由后端提供 BFF 代理及独立 RBAC 鉴权。


## 五、 本仓库 Agent 约定

本节维护 XDD 仓库特定的编码边界、数据边界和本地运行命令；通用协作规范见 [AGENTS.md](AGENTS.md)。

### 数据与配置边界

*   新增或修改数据库 migration 时，必须同步更新 `database/init.sql`，确保新环境初始化后的 schema 与按历史 migrations 升级后的 schema 一致。
*   `database/init.sql` 和 `database/migrations/` 只负责 schema 变更和必要的数据结构迁移，禁止写入业务表、配置表、提示词表、商品表、策略表等配置/种子数据。
*   本地默认配置和 mock 种子数据统一维护在 `deploy-local/mock/` 下，由本地 mock 导入流程或 Admin/API 写入。
*   环境变量统一归口到部署目录管理：本地开发使用 `deploy-local/.env` 与 `deploy-local/.env.example`；现有 K3s 骨架位于 `deploy/k3s/`，在生产化方案验收前不得视为可直接上线的生产配置。
*   前端应用目录（例如 `frontend-web/`、`frontend-admin/`）禁止新增或维护 `.env*` 文件，Next/Vite 等前端运行变量也必须从 `deploy-local/` 或后续选定的生产部署目录注入。

### 服务修改后验证

*   修改 `backend-api` 或 `backend-gateway` 下的 Go 代码后，必须运行：

```bash
bash deploy-local/tools/reload-go-local.sh
```

*   该脚本会重新编译 go-api 和 go-gateway，并重启本地后端 Docker 容器，确保容器内进程加载新二进制。
*   `reload-go-local.sh` 会写入用户级 Go build cache；如编码 Agent 运行在沙箱内且无法写入仓库外缓存目录，应对该命令请求最小范围提权后执行。
*   本地前端服务默认由用户在宿主机通过 tmux 运行；启动脚本自动优先加载 `deploy-local/.env.browser-agent`，也可通过 `BROWSER_AGENT_ENV_FILE` 指定其他配置文件。Web 使用：

```bash
bash deploy-local/tools/run-web-host-local.sh start
```

*   Admin 使用：

```bash
bash deploy-local/tools/run-admin-host-local.sh start
```

*   前端脚本支持 `start`、`restart`、`stop`、`status`，分别管理 `web` / `admin` tmux session。
*   Agent 不负责直接持有宿主机 `next dev` / `vite dev` 前台进程；如用户已通过脚本启动宿主机前端，Agent 可复用对应端口做页面验证。Next.js 禁止在同一工作区同时运行 `next dev` 与 `next build`，避免共同写入 `.next` 导致缓存产物损坏。
*   本地 Docker Compose 只承载后端和基础设施，不定义 `web` / `admin` 前端服务。

### 路径与命令注意事项

*   Shell 中访问含 glob 特殊字符的仓库路径时必须加引号，例如 `frontend-web/app/[lang]/...`、`docs/prd&ui/...`。
*   本地后端 Compose 文件为 `deploy-local/docker-compose-backend.yaml`；共享基础设施 Compose 文件为 `deploy-local/docker-compose-infra.yaml`。
*   本机不要假设容器内的 `api:8001` 一定暴露给宿主机。当前 Browser Agent 本地开发直接访问 Go API `http://localhost:29001`；调用 `/internal/...` 内部接口应在容器网络内执行，例如：

```bash
docker compose -f deploy-local/docker-compose-backend.yaml exec -T ai-engine ...
```

*   调用 `/internal/...` 接口必须携带 `X-Internal-Secret`，secret 从当前容器环境或 `deploy-local/.env` 获取。
*   需要访问 Docker socket 的命令，例如 `docker compose ... exec/restart/up`，在沙箱环境中应请求最小范围提权执行。
*   修改 `deploy-local/.env` 或 Compose `env_file` 后，普通 reload/restart 不会刷新容器环境变量；需要重建对应服务容器，例如：

```bash
docker compose -f deploy-local/docker-compose-backend.yaml up -d --force-recreate <service>
```

*   本地若未配置 `R2_CUSTOM_DOMAIN`，R2 presign 只保证上传可用；不要把 `*.r2.cloudflarestorage.com/<bucket>/...` 当作浏览器公开 URL 使用，前端应使用临时预览 URL 或本地 blob 缓存展示。

## 六、 文档治理与 Roadmap

本项目文档索引按目录结构维护，不按业务标签作为一级分组。文件名中的 `@tag` 只作为检索辅助，README 中的文档链接必须使用仓库相对路径，禁止使用本机绝对路径或 `file://` 链接。

### 文档治理规则

*   新文档必须放入 `docs/` 对应子目录，禁止在根目录新增普通 `.md` 文档；根目录仅保留项目入口类文件，例如 `README.md`、`AGENTS.md`、`CLAUDE.md`、`GEMINI.md`。
*   文档目录优先表达文档类型与用途：`docs/brd/` 放商业需求，`docs/prd&ui/` 放产品与 UI，`docs/tech/` 放技术方案与审计，`docs/deploy/` 放部署运维，`docs/tech/mvp/` 放 MVP 横向技术方案，根目录 `compliance/` 放合规、法务、主体、税务和跨境运营相关文档。
*   文档文件名使用字母、数字、短横线和可选 `@tag` 标记；禁止使用 `[tag]` 方括号标签，避免命令行 glob 转义问题。
*   有明确主题归属的文档可在文件名中保留 `@tag`，格式为 `YYMMDD-@tag-description.md` 或 `@tag-description.md`；无明确主题归属的基础设施、测试、纯技术文档使用 `YYMMDD-description.md` 或 `description.md`。
*   文档正文标题不强制注入业务标签；需要强调归属时，可在标题或摘要中自然说明。
*   新增或调整重要文档后，必须同步维护本节 Roadmap，并按目录结构放入对应分组。
*   写文档的时候，需要在文档开头写清楚changelog。

> 已过时的历史文档统一归档至 `docs/archive/`、各子目录的 `archive/` 或 `repo-archive/`，不再在下方索引中列出。

### 文档目录索引

#### `docs/brd/`

- [0627-browser-agent-automation-brd.md](docs/brd/0627-browser-agent-automation-brd.md)：Browser Agent 商业场景扩展与架构解耦 BRD，规划版权侵权检索、海外社交媒体自动化运营及单仓库内的模块隔离策略。

#### `docs/prd&ui/`

#### `docs/tech/`

- [0617-local-automation-worker-architecture.md](docs/tech/0617-local-automation-worker-architecture.md)：基于 Browser Use、Crawl4AI、Skyvern 调研形成的通用本地 Browser Worker 技术架构与历史演进记录。
- [0617-local-automation-worker-api-schema.md](docs/tech/0617-local-automation-worker-api-schema.md)：通用 Local Automation Worker 的 `automation_*` 数据模型、Worker API、任务 payload、人工动作、artifact 和错误码设计。
- [0617-local-automation-worker-implementation-plan.md](docs/tech/0617-local-automation-worker-implementation-plan.md)：通用 Automation Worker 的历史实施路线，覆盖 runtime 分层、go-api、artifact、人机协同、社媒 PoC 和 Browser Agent 增强。
- [0617-local-automation-platform-iteration-plan.md](docs/tech/0617-local-automation-platform-iteration-plan.md)：通用 Automation 平台整体迭代计划，统一平台 go-api、数据库、Admin、Web、Worker、部署验证和 YouTube/TikTok PoC 的阶段路线。
- [0619-llm-browser-agent-integration-plan.md](docs/tech/0619-llm-browser-agent-integration-plan.md)：LLM Browser Agent 接入计划，明确 deterministic adapter 与 LLM Agent 边界、工具集、视觉/HTML 理解、阶段路线和未完成清单。
- [0702-weixin-group-file-sync-agent-plan.md](docs/tech/0702-weixin-group-file-sync-agent-plan.md)：微信群资料同步 Agent 技术方案，规划基于 `weixin-agent-sdk` 的微信文件接收、本地归档、manifest 记录和后续资料库接入路径。
- [0709-browser-agent-branch-handoff.md](docs/tech/0709-browser-agent-branch-handoff.md)：Browser Agent 项目交接文档，覆盖当前实现状态、本地启动、核心代码地图、任务链路、adapter 边界、风险事项和后续待办。
- [0719-browser-agent-productionization-plan.md](docs/tech/0719-browser-agent-productionization-plan.md)：线上客户交付与生产化技术方案，定义云端控制面 + 客户本机 Worker 架构，以及账号租户、部署、数据、Worker、安全、监控和 CI/CD 上线门禁。

#### `docs/tech/mvp/`

#### `docs/deploy/`

#### `docs/`


#### `deploy/local/`

#### `compliance/`

- [0619-browser-automation-crawler-compliance-note.md](compliance/0619-browser-automation-crawler-compliance-note.md)：浏览器自动化、网页采集和 PDF 下载的中国法合规备忘录，覆盖刑法计算机犯罪边界、个人信息、数据安全、版权、反不正当竞争和产品红线。
