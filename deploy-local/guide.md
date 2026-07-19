# Browser Agent 本地开发指南

## Changelog

- 2026-07-19：仓库收敛为单一 Browser Agent 项目；启动脚本固定优先加载 `.env.browser-agent`，统一默认端口和服务命名。
- 2026-07-19：新增 Windows Browser Agent 一键端到端验收，通过 Web 代理创建确定性任务、真实 Chromium 执行，并从 Admin 代理校验 run、trace 和截图；移除已失效的旧 demo 命令。
- 2026-07-17：新增 Windows 原生平台与 Worker 启停脚本，覆盖 Go API、Web、Admin、Worker 的隐藏窗口常驻运行、状态检查、日志和 PID 管理。
- 2026-06-19：补充完整本地验收流程，覆盖环境检查、API/Worker 常驻启动、日志观察、mock/MySQL/Browser Agent、artifact 校验、manual action 和 Worker token 失效处理。
- 2026-06-19：本机默认端口整体上移 20000，避让同机多项目开发端口冲突。
- 2026-06-18：调整 demo 为常驻 Worker 形态：`run-worker-host-local.sh` 新增 `deploy-local/logs/worker-local.log` 常驻日志。
- 2026-06-18：新增 `frontend-admin` Vite/React 调试控制台和 `run-admin-host-local.sh`，并将 `run-api-host-local.sh` 调整为 tmux 常驻模式，避免宿主进程被 shell 回收。
- 2026-06-18：补齐本地 MySQL/Redis infra compose、schema apply 脚本和 MySQL 持久化 smoke test。`backend-api` 现在可在 `MYSQL_DSN` + `REDIS_ADDR` 下跑通 Automation 持久化闭环。
- 2026-06-18：新增当前可用的 `backend-api` 宿主机启动方式和 Automation mock smoke test。Docker Compose、Gateway、Admin、Web 和 MySQL 持久化链路仍按后续迭代补齐。

## 一、配置与启动方式

本仓库只维护 Browser Agent 本地环境。启动脚本依次尝试读取：

1. `BROWSER_AGENT_ENV_FILE` 指定的文件；
2. `deploy-local/.env.browser-agent`；
3. `deploy-local/.env`。

首次使用可从模板创建本地配置；配置文件可能包含本机密钥，不要提交：

```bash
cp deploy-local/.env.example deploy-local/.env.browser-agent
```

**启动基础设施并应用数据库：**

```bash
bash deploy-local/tools/run-infra-local.sh start
bash deploy-local/tools/db-apply.sh all
```

**启动 API 及其他组件：**

```bash
bash deploy-local/tools/run-api-host-local.sh start
bash deploy-local/tools/run-admin-host-local.sh start
bash deploy-local/tools/run-web-host-local.sh start
bash deploy-local/tools/run-worker-host-local.sh start
```

模板默认使用 API `29001`、Web `24001`、Admin `26174`、MySQL `24307` 和 Redis `27380`。未配置 `MYSQL_DSN` 时，`backend-api` 使用内存 repository，进程重启后状态会丢失；未配置 `REDIS_ADDR` 时使用 no-op locker，便于无 Redis 的本地 mock 验收。

当前 Redis 用于 `automation:jobs:claim` 短租约锁，保护多 API 实例并发领取任务。

### Windows 原生启动（browser-agent）

Windows 需要 Docker Desktop（WSL2 后端）、Node.js/npm、Go 1.25 和 Python 3.12。平台脚本优先使用 `PATH` 中的 Go；未找到时会回退到 `%LOCALAPPDATA%\CodexTools\go1.25.0\go\bin\go.exe`。Worker 使用 `worker/local-cli/.venv`，浏览器由 Playwright 管理。

首次准备基础设施和数据库：

```powershell
docker compose --env-file deploy-local/.env.browser-agent -f deploy-local/docker-compose-infra.yaml up -d
bash deploy-local/tools/db-apply.sh all
```

若 Windows 没有 Bash，可进入 MySQL 容器依次应用 `database/init.sql`、`database/migrations/000000-baseline.sql`、`database/migrations/20260618_automation_platform_schema.sql` 和 `database/migrations/20260618_worker_device_token_hash.sql`。已初始化的数据卷不需要重复执行。

启动和检查 API、Web、Admin：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/tools/run-platform-windows.ps1 start -Environment browser-agent
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/tools/run-platform-windows.ps1 status -Environment browser-agent
```

首次初始化 Worker 并安装 Chromium：

```powershell
worker/local-cli/.venv/Scripts/python.exe -m playwright install chromium
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/tools/run-worker-windows.ps1 init -Environment browser-agent
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/tools/run-worker-windows.ps1 pair -Environment browser-agent
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/tools/run-worker-windows.ps1 doctor -Environment browser-agent
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/tools/run-worker-windows.ps1 start -Environment browser-agent
```

Windows 未配置系统凭据存储时，本地开发环境可在忽略提交的 `.env.browser-agent` 中设置 `BROWSER_AGENT_WORKER_ALLOW_INSECURE_FILE_SECRETS=1`。这会把 Worker token 保存在用户应用数据目录，只适合受控的本地开发机，生产环境禁止使用。

`browser-agent` 默认访问地址：Web `http://localhost:24001`、Admin `http://localhost:26174`、API `http://localhost:29001`、MySQL `127.0.0.1:24307`、Redis `127.0.0.1:27380`。停止服务时把 `start` 改为 `stop`；重启时改为 `restart`。日志位于 `deploy-local/logs/`，PID 文件位于 `deploy-local/run/`。

## 二、服务管理

基础设施：

```bash
bash deploy-local/tools/run-infra-local.sh start
bash deploy-local/tools/run-infra-local.sh status
bash deploy-local/tools/run-infra-local.sh logs
bash deploy-local/tools/run-infra-local.sh stop
```

数据库 schema：

```bash
bash deploy-local/tools/db-apply.sh init
bash deploy-local/tools/db-apply.sh migrations
bash deploy-local/tools/db-apply.sh all
```

API：

```bash
bash deploy-local/tools/run-api-host-local.sh status
bash deploy-local/tools/run-api-host-local.sh logs
bash deploy-local/tools/run-api-host-local.sh restart
bash deploy-local/tools/run-api-host-local.sh stop
```

Admin：

```bash
bash deploy-local/tools/run-admin-host-local.sh start
bash deploy-local/tools/run-admin-host-local.sh status
bash deploy-local/tools/run-admin-host-local.sh stop
```

Admin 默认访问：

```text
http://localhost:26174
```

如果 `26174` 已被占用，脚本会选择下一个空闲端口，并在 `deploy-local/run/browser_agent-frontend-admin.port` 记录真实端口。Admin dev server 将浏览器里的 `/api/*` 转发到 `ADMIN_API_BASE_URL`，默认是 `http://127.0.0.1:29001`。

Web：

```bash
bash deploy-local/tools/run-web-host-local.sh start
bash deploy-local/tools/run-web-host-local.sh status
bash deploy-local/tools/run-web-host-local.sh stop
```

Web 默认访问：

```text
http://localhost:24001
```

如果 `24001` 已被占用，脚本会选择下一个空闲端口，并在 `deploy-local/run/browser_agent-frontend-web.port` 记录真实端口。Web 通过 Next.js rewrite 访问 `http://127.0.0.1:29001` 的 `/web/*` API；`backend-api` 已对 localhost origin 开启本地 CORS。

本地鉴权边界：

```text
INTERNAL_SECRET=local-dev-internal-secret
ADMIN_API_TOKEN=
WEB_API_TOKEN=
```

`ADMIN_API_TOKEN` 或 `WEB_API_TOKEN` 为空时，对应接口保持本地开发免 token；配置后，`/admin/*` 需要 `X-Admin-Token` 或 `Authorization: Bearer ...`，`/web/*` 需要 `X-Web-Token` 或 `Authorization: Bearer ...`。Admin dev server 会从本地环境文件注入 `VITE_ADMIN_API_TOKEN`；浏览器端可通过 localStorage 设置 `browser-agent.adminToken` 或 `browser-agent.webToken`。

Worker：

```bash
bash deploy-local/tools/run-worker-host-local.sh init
bash deploy-local/tools/run-worker-host-local.sh pair
bash deploy-local/tools/run-worker-host-local.sh doctor
bash deploy-local/tools/run-worker-host-local.sh once
bash deploy-local/tools/run-worker-host-local.sh start
bash deploy-local/tools/run-worker-host-local.sh status
bash deploy-local/tools/run-worker-host-local.sh stop
```

Worker 是用户本机执行节点，不放进 Docker。它会使用本机 Python、Playwright 和 Chromium，并把配置、设备信息和浏览器 profile 放在用户系统应用数据目录。默认平台地址来自 `WORKER_SERVER_URL`，未配置时使用 `http://127.0.0.1:29001`。常驻 Worker 的 stdout/stderr 会写入：

```text
deploy-local/logs/worker-local.log
```

日志和运行状态：

```text
deploy-local/logs/backend-api.log
deploy-local/logs/worker-local.log
tmux session: browser_agent-backend-api-local
tmux session: browser_agent-worker-local
```

`deploy-local/logs/` 和 `deploy-local/run/` 是本地运行产物，不应提交。

## 三、Automation Mock Smoke Test

内存/当前 API smoke test：

```bash
bash deploy-local/integration-test/10-automation-mock.sh
```

脚本会验证：

1. `POST /worker/devices/pairing`
2. `GET /worker/devices/pairing/{pairing_id}`
3. `POST /worker/devices/{device_id}/heartbeat`
4. `POST /admin/automation/jobs`
5. `GET /worker/automation/jobs/next`
6. `POST /worker/automation/runs/{run_id}/checkpoint`
7. `POST /worker/automation/runs/{run_id}/artifacts`
8. `POST /worker/automation/runs/{run_id}/complete`
9. `GET /admin/automation/jobs/{job_id}`
10. `GET /admin/automation/runs/{run_id}`
11. `GET /admin/automation/runs/{run_id}/artifacts`

如果本地 API 没启动，脚本会临时启动 `backend-api`，测试结束后自动停止。

MySQL/Redis 持久化 smoke test：

```bash
bash deploy-local/integration-test/20-automation-mysql-smoke.sh
```

脚本会启动 MySQL/Redis、应用 schema、在 `:38001` 临时启动 MySQL 模式 API，并验证 pairing、heartbeat、job claim、checkpoint、artifact、本地文件 artifact 上传下载、manual action resolve、列表查询、设备撤销和 revoked token 拒绝。

Browser Agent Windows 端到端验收：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/integration-test/30-browser-agent-windows.ps1 -Environment browser-agent
```

该脚本要求 API、Web、Admin 和已配对 Worker 均已启动。它通过 Web 的 Next.js 代理创建 `deterministic_search` 任务，让 Worker 控制真实 headless Chromium 打开内嵌 fixture、填写并提交搜索；随后通过 Web 查询 run，并通过 Admin 代理校验任务完成以及 `agent_trace`、`screenshot` 两类 artifact。此模式不调用 LLM，适合稳定的环境验收。

## 四、完整本地验收流程

### 1. 环境和配置检查

```bash
cd deploy-local
./tools/run-infra-local.sh status
./tools/run-api-host-local.sh status
./tools/run-worker-host-local.sh status
./tools/run-worker-host-local.sh doctor
curl -fsS http://127.0.0.1:29001/healthz
```

检查 `deploy-local/.env` 至少包含：

```text
API_ADDR=:29001
MYSQL_DSN=<使用 deploy-local/.env.browser-agent 中的本地 MySQL DSN>
REDIS_ADDR=127.0.0.1:27380
ARTIFACT_DIR=deploy-local/artifacts
```

### 2. 启动常驻平台和 Worker

```bash
./tools/run-infra-local.sh start
./tools/db-apply.sh all
./tools/run-api-host-local.sh start
./tools/run-worker-host-local.sh restart
```

如果 Worker 未绑定或 token 失效，重新绑定：

```bash
./tools/run-worker-host-local.sh stop
./tools/run-worker-host-local.sh pair
./tools/run-worker-host-local.sh restart
```

### 3. 观察日志

开两个终端分别执行：

```bash
tail -f logs/backend-api.log
```

```bash
tail -f logs/worker-local.log
```

Worker 日志中应能看到：

```text
worker.poll
worker.no_job
worker.job_claimed job_id=... run_id=...
worker.job_finished job_id=... status=...
```

如果看到 `UNAUTHORIZED: missing or invalid device token`，说明本地 device token 和平台记录不一致，按第 2 步重新 `pair`。

### 4. 跑自动化基础验收

```bash
./integration-test/10-automation-mock.sh
./integration-test/20-automation-mysql-smoke.sh
```

`10` 验证内存/当前 API 的 job/run/artifact 生命周期。`20` 验证 MySQL/Redis 持久化、artifact 文件上传下载、manual action resolve、设备撤销和 revoked token 拒绝。

### 5. 跑 Browser Agent Windows 端到端验收

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File deploy-local/integration-test/30-browser-agent-windows.ps1 -Environment browser-agent
```

期望输出：

```text
browser agent Windows E2E passed
job=...
run=...
artifacts=agent_trace,screenshot
```

### 6. 验证 artifact 和本地执行记录

用脚本输出的 `run_id` 查看 artifact：

```bash
curl -s "http://127.0.0.1:29001/admin/automation/runs/<run_id>/artifacts" | python3 -m json.tool
```

下载 PDF：

```bash
curl -L "http://127.0.0.1:29001/admin/automation/artifacts/<artifact_id>/download" -o /tmp/browser-agent-demo.pdf
file /tmp/browser-agent-demo.pdf
```

Worker 本地任务记录在系统应用数据目录。为保留已配对设备和浏览器 profile，当前 CLI 仍沿用历史兼容目录名：

```text
~/Library/Application Support/QIYUAN Worker/jobs/<job_id>/
```

重点查看：

```bash
cat "$HOME/Library/Application Support/QIYUAN Worker/jobs/<job_id>/job.json"
cat "$HOME/Library/Application Support/QIYUAN Worker/jobs/<job_id>/checkpoints.ndjson"
cat "$HOME/Library/Application Support/QIYUAN Worker/jobs/<job_id>/upload-manifest.json"
```

### 7. 处理验证码/manual action

如果自动化流程输出 `needs_manual_action`，先在本机 Chromium 里完成 Google 验证，再查 pending action：

```bash
curl -s "http://127.0.0.1:29001/admin/automation/manual-actions?status=pending&limit=5" | python3 -m json.tool
```

resolve：

```bash
curl -X POST "http://127.0.0.1:29001/admin/automation/manual-actions/<manual_action_id>/resolve" \
  -H "Content-Type: application/json" \
  -d '{"status":"resolved","payload":{"resolved_by":"local-demo"}}'
```

## 五、当前边界

1. `backend-api` 未配置 `MYSQL_DSN` 时使用内存 repository；配置后会使用 MySQL repository。
2. MySQL schema 已在 `database/init.sql` 和 `database/migrations/` 中定义，`deploy-local/tools/db-apply.sh` 可应用基线和迁移。
3. Redis 已接入 job claim 短锁；未配置时不会影响本地 mock 流程。
4. Worker 已支持 automation runtime、mock adapter 和 `generic.browser_agent` PoC。
5. Admin 已有 Automation 调试控制台首版；Web 已有 Browser Agent、版权检索、社媒运营和微信资料同步入口；Gateway 和 AI Engine 的本地启动脚本后续补齐。
