# Browser Agent 底座

本项目是用于浏览器自动化执行与代理控制的基础底座平台。

## 一、 项目入口与 Agent 协作

- `README.md` 是项目事实入口：维护架构、目录归属、数据库 ownership、文档治理规则和文档 Roadmap。
- `AGENTS.md` 是通用 agent 行为入口：维护 Codex、Claude、Gemini 等编码 agent 都应遵守的协作、编码、安全和验证习惯。

### 业务分支解耦策略
本项目为底层基础 Browser Agent 框架，具体业务需求通过 Git 分支层面进行严格的业务隔离，以保持底座纯粹性和组件技术栈一致性：
- **`feature/browser-agent` 分支**：作为通用能力拓展底座，并承载泛互联网自动化运营（如社媒维护、短剧版权侵权检测）等泛内容商业场景。
- **`feature/qiyuan` 分支**：专注 AI 4 Science 方向，聚焦学术文献下载和科研知识库抽取业务。
两个分支共用底层架构（如 Playwright 模拟、分布式 Worker 调度等机制）。底层通用能力的升级可通过合并拣选（Cherry-pick）跨分支共享。

## 二、 本地开发启动

本地开发统一使用 Docker Compose；完整说明见 [deploy-local/guide.md](deploy-local/guide.md)，包括 Agent 进程沙箱避坑、共享基础设施、Go 本机编译、日志、健康检查和 Admin 账号说明。

### 快速启动：

```bash
bash deploy-local/tools/run-infra-local.sh start
bash deploy-local/tools/db-apply.sh all
bash deploy-local/tools/run-api-host-local.sh start
bash deploy-local/tools/run-admin-host-local.sh start
bash deploy-local/tools/run-web-host-local.sh start
bash deploy-local/tools/run-worker-host-local.sh init
bash deploy-local/tools/run-worker-host-local.sh pair
bash deploy-local/integration-test/10-automation-mock.sh
bash deploy-local/integration-test/20-automation-mysql-smoke.sh
```

`run-infra-local.sh` 默认启动 MySQL 和 Redis。Elasticsearch 仍未接入当前 `deploy-local` infra compose。

前端和本机 API 默认在宿主机通过 tmux 启动。前端脚本支持 `start`、`restart`、`stop`、`status`；无参数等价于 `start`。首次使用前需确保宿主机已安装 tmux，例如 `brew install tmux`。如果 Web 的 `23001` 或 Admin 的 `25174` 被占用，脚本会自动使用下一个空闲端口。

### 本地端口(TODO)：

| 服务 | 宿主机地址 |
| --- | --- |
| Web | `http://localhost:23001` |
| Admin | `http://localhost:25174` |
| Gateway | `http://localhost:28080` |
| Go API | `http://localhost:28001` |
| AI Engine | `http://localhost:28002` |
| MySQL | `127.0.0.1:23307` |
| Redis | `127.0.0.1:26380` |
| Elasticsearch | `http://localhost:29200` |

## 三、 核心微服务架构

后端采用“职责解耦”的异构设计，最大化发挥不同语言的生态优势：

*   **go-gateway — 流量网关**
    *   统一请求入口（基于 Gin）。负责 C 端 JWT 鉴权、基于 Redis 的高并发限流、CORS、人机验证（Turnstile）以及长对话 SSE（Server-Sent Events）的无缓冲流式透传。
*   **go-api — 核心业务中枢**
    *   严密的三层架构（Handler → Engine → Repo）。
    *   **唯一写权限节点**：全系统只有 go-api 有权对 MySQL 数据库执行 `INSERT/UPDATE/DELETE`。负责账号体系、支付订阅、创作者发布及 Admin 后台管理。
    *   对内部（Python）暴露 `X-Internal-Secret` 鉴权的 `/internal/*` 端点处理强一致性写操作。

### 代码目录归属(TODO)

*   `backend-gateway`：Go API 网关，负责 JWT、限流、CORS、SSE 透传。
*   `backend-api`：Go 业务主服务，采用 Handler → Engine → Repository 分层，是用户、支付、业务数据的唯一写入节点。
*   `worker/local-cli`：本地 Browser Worker CLI，P1 以 macOS Python 3.12 命令行形态运行，负责设备绑定、领取任务、控制本机 Chromium、上传爬取 artifact；不得保存或上传第三方文献源账号密码。
*   `frontend-web`：Next.js 15 App Router，Cloudflare Pages 部署目标。
*   `frontend-admin`：React + Vite 管理后台。

### 数据库 ownership 与迁移

*   Go API 管理 `user*`、`verify_code` 等业务写入。
*   跨服务强一致写操作使用 `X-Internal-Secret` 和 `/internal/*`。
*   `database/init.sql` 只作为新库基线；任何新增或调整字段、索引、表的变更，必须同步新增或更新 `database/migrations/` 下的幂等 migration，确保已有环境可增量升级。

## 四、 前端

*   **Web 端 (`frontend-web`)**
    *   Next.js 15 (App Router) + TailwindCSS 4。
    *   优先 SSR 提升 SEO 与首屏性能，打包后通过 Cloudflare Pages 部署于边缘网络。
*   **Admin 控制台 (`frontend-admin`)**
    *   React + Vite SPA。内部运营管理与可视化测试面板。所有请求走 `/admin/*` 路径，由后端提供 BFF 代理及独立 RBAC 鉴权。


## 五、 本仓库 Agent 约定

本节维护 XDD 仓库特定的编码边界、数据边界和本地运行命令；通用协作规范见 [AGENTS.md](AGENTS.md)。

### 数据与配置边界

*   新增或修改数据库 migration 时，必须同步更新 `database/init.sql`，确保新环境初始化后的 schema 与按历史 migrations 升级后的 schema 一致。
*   `database/init.sql` 和 `database/migrations/` 只负责 schema 变更和必要的数据结构迁移，禁止写入业务表、配置表、提示词表、商品表、策略表等配置/种子数据。
*   本地默认配置和 mock 种子数据统一维护在 `deploy-local/mock/` 下，由本地 mock 导入流程或 Admin/API 写入。
*   环境变量统一归口到 `deploy-{local/sta}` 目录管理：本地开发使用 `deploy-local/.env` 与 `deploy-local/.env.example`；K3s 使用 `deploy-k3s/{sta,prod}/env.yaml` 和对应 `.env.example`。
*   前端应用目录（例如 `frontend-web/`、`frontend-admin/`）禁止新增或维护 `.env*` 文件，Next/Vite 等前端运行变量也必须从 `deploy-{local/sta}` 注入。

### 服务修改后验证

*   修改 `backend-api` 或 `backend-gateway` 下的 Go 代码后，必须运行：

```bash
bash deploy-local/tools/reload-go-local.sh
```

*   该脚本会重新编译 go-api 和 go-gateway，并重启本地后端 Docker 容器，确保容器内进程加载新二进制。
*   `reload-go-local.sh` 会写入用户级 Go build cache；如编码 Agent 运行在沙箱内且无法写入仓库外缓存目录，应对该命令请求最小范围提权后执行。
*   本地前端服务默认由用户在宿主机通过 tmux 运行；如需指定环境隔离，请在所有启动脚本前带上 `QIYUAN_ENV` 变量（例如 `QIYUAN_ENV=browser-agent` 或 `QIYUAN_ENV=qiyuan`），不同环境将映射到独立的底层数据库和偏移端口。Web 使用：

```bash
QIYUAN_ENV=qiyuan bash deploy-local/tools/run-web-host-local.sh start
```

*   Admin 使用：

```bash
QIYUAN_ENV=qiyuan bash deploy-local/tools/run-admin-host-local.sh start
```

*   前端脚本支持 `start`、`restart`、`stop`、`status`，分别管理 `web` / `admin` tmux session。
*   Agent 不负责直接持有宿主机 `next dev` / `vite dev` 前台进程；如用户已通过脚本启动宿主机前端，Agent 可复用对应端口做页面验证。**注意：Next.js 禁止在同目录运行多个 dev server，若要在本地同时启动 qiyuan 和 browser-agent 两个环境的 Web，请克隆两份源码分目录执行。**
*   本地 Docker Compose 只承载后端和基础设施，不定义 `web` / `admin` 前端服务。

### 路径与命令注意事项

*   Shell 中访问含 glob 特殊字符的仓库路径时必须加引号，例如 `frontend-web/app/[lang]/...`、`docs/prd&ui/...`。
*   本地后端 Compose 文件为 `deploy-local/docker-compose-backend.yaml`；共享基础设施 Compose 文件为 `deploy-local/docker-compose-infra.yaml`。
*   本机不要假设容器内的 `api:8001` 一定暴露给宿主机。宿主机调用公开 API 走 Gateway，例如 `http://localhost:28080`；调用 `/internal/...` 内部接口应在容器网络内执行，例如：

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

- [0627-browser-agent-automation-brd.md](docs/brd/0627-browser-agent-automation-brd.md)：Browser Agent 商业场景扩展与架构解耦 BRD，规划海外社交媒体自动化运营、短剧版权侵权检索以及不同业务分支间的隔离共享策略。

#### `docs/prd&ui/`

#### `docs/tech/`

- [0617-local-automation-worker-architecture.md](docs/tech/0617-local-automation-worker-architecture.md)：基于 Browser Use、Crawl4AI、Skyvern 调研，将本地 Worker 升级为可支持文献爬取、浏览器 Agent、TikTok/YouTube 自动上传和通用网页自动化任务的技术架构。
- [0617-local-automation-worker-api-schema.md](docs/tech/0617-local-automation-worker-api-schema.md)：通用 Local Automation Worker 的 `automation_*` 数据模型、Worker API、任务 payload、人工动作、artifact 和错误码设计。
- [0617-local-automation-worker-implementation-plan.md](docs/tech/0617-local-automation-worker-implementation-plan.md)：通用 Automation Worker 的实施路线，覆盖 runtime 分层、go-api、Google Scholar adapter、artifact、人机协同、YouTube/TikTok PoC 和 Browser Agent 增强。
- [0617-local-automation-platform-iteration-plan.md](docs/tech/0617-local-automation-platform-iteration-plan.md)：通用 Automation 平台整体迭代计划，统一平台 go-api、数据库、Admin、Web、Worker、部署验证和 YouTube/TikTok PoC 的阶段路线。
- [0619-llm-browser-agent-integration-plan.md](docs/tech/0619-llm-browser-agent-integration-plan.md)：LLM Browser Agent 接入计划，明确 deterministic adapter 与 LLM Agent 边界、工具集、视觉/HTML 理解、阶段路线和未完成清单。
- [0702-weixin-group-file-sync-agent-plan.md](docs/tech/0702-weixin-group-file-sync-agent-plan.md)：微信群资料同步 Agent 技术方案，规划基于 `weixin-agent-sdk` 的微信文件接收、本地归档、manifest 记录和后续资料库接入路径。
- [0709-browser-agent-branch-handoff.md](docs/tech/0709-browser-agent-branch-handoff.md)：`feature/browser-agent` 分支工作交接文档，覆盖当前实现状态、本地启动、核心代码地图、任务链路、adapter 边界、风险事项和后续待办。

#### `docs/tech/mvp/`

#### `docs/deploy/`

#### `docs/`


#### `deploy/local/`

#### `compliance/`

- [0619-browser-automation-crawler-compliance-note.md](compliance/0619-browser-automation-crawler-compliance-note.md)：浏览器自动化、网页采集和 PDF 下载的中国法合规备忘录，覆盖刑法计算机犯罪边界、个人信息、数据安全、版权、反不正当竞争和产品红线。
