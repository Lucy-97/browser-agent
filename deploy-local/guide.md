# QIYUAN 本地开发指南

## Changelog

- 2026-06-27：引入本地多环境隔离机制（qiyuan 与 browser-agent）。支持通过 `QIYUAN_ENV` 变量无缝切换不同的底层容器（MySQL/Redis/Neo4j）和宿主机端口，实现单机双分支安全并存。
- 2026-06-19：补充完整本地验收流程，覆盖环境检查、API/Worker 常驻启动、日志观察、mock/MySQL/Browser Agent/Google Scholar PDF demo、artifact 校验、manual action 和 Worker token 失效处理。
- 2026-06-19：本机默认端口整体上移 20000，避让同机多项目开发端口冲突。
- 2026-06-18：调整 demo 为常驻 Worker 形态：`run-worker-host-local.sh` 新增 `deploy-local/logs/worker-local.log` 常驻日志。
- 2026-06-18：Browser Agent 本地 demo 支持可视化运行：`HEADED=1 PAUSE_AFTER_SECONDS=10 bash deploy-local/integration-test/30-browser-agent-demo.sh`。
- 2026-06-18：新增 Browser Agent 本地 demo 验收脚本 `deploy-local/integration-test/30-browser-agent-demo.sh`，用于验证 `generic.browser_agent` adapter 可操作 Chromium 打开本地 fixture、完成搜索、上传 trace 和截图 artifact。
- 2026-06-18：新增 `frontend-admin` Vite/React 调试控制台和 `run-admin-host-local.sh`，并将 `run-api-host-local.sh` 调整为 tmux 常驻模式，避免宿主进程被 shell 回收。
- 2026-06-18：补齐本地 MySQL/Redis infra compose、schema apply 脚本和 MySQL 持久化 smoke test。`backend-api` 现在可在 `MYSQL_DSN` + `REDIS_ADDR` 下跑通 Automation 持久化闭环。
- 2026-06-18：新增当前可用的 `backend-api` 宿主机启动方式和 Automation mock smoke test。Docker Compose、Gateway、Admin、Web 和 MySQL 持久化链路仍按后续迭代补齐。

## 一、环境隔离与启动方式

自 2026-06-27 起，QIYUAN 项目在本地支持物理隔离的多环境并存开发。通过设置 `QIYUAN_ENV` 环境变量，可以在不同的分支和场景中使用不同的配置（默认读取 `deploy-local/.env.${QIYUAN_ENV}`，未指定时默认回退读取 `deploy-local/.env` 或 `.env.qiyuan`）。

目前支持的环境包括：
- `qiyuan`：用于文献检索与知识库业务（如 `feature/qiyuan` 分支）。端口基准：23000。
- `browser-agent`：用于自动化运营业务（如 `feature/browser-agent` 分支）。端口基准：24000。

**启动指定环境的基础设施：**

```bash
QIYUAN_ENV=qiyuan bash deploy-local/tools/run-infra-local.sh start
QIYUAN_ENV=qiyuan bash deploy-local/tools/db-apply.sh all
```

**启动 API 及其他组件：**

```bash
QIYUAN_ENV=qiyuan bash deploy-local/tools/run-api-host-local.sh start
QIYUAN_ENV=qiyuan bash deploy-local/tools/run-web-host-local.sh start
```

可通过对应环境的 `.env.${QIYUAN_ENV}` 覆盖变量：

```bash
cp deploy-local/.env.example deploy-local/.env
```

```text
API_ADDR=:28001
ARTIFACT_DIR=deploy-local/artifacts
MYSQL_DSN=qiyuan:qiyuan@tcp(127.0.0.1:23307)/qiyuan?parseTime=true&charset=utf8mb4&loc=Local
MYSQL_MAX_OPEN_CONNS=10
MYSQL_MAX_IDLE_CONNS=5
REDIS_ADDR=127.0.0.1:26380
REDIS_PASSWORD=
REDIS_DB=0
ADMIN_API_BASE_URL=http://127.0.0.1:28001
```

未配置 `MYSQL_DSN` 时，`backend-api` 使用内存 repository，进程重启后状态会丢失。如本机 MySQL 已准备好 schema，可设置：

```text
MYSQL_DSN=qiyuan:qiyuan@tcp(127.0.0.1:23307)/qiyuan?parseTime=true&charset=utf8mb4&loc=Local
```

如本机已启动 Redis，可设置：

```text
REDIS_ADDR=127.0.0.1:26380
```

当前 Redis 用于 `automation:jobs:claim` 短租约锁，保护多 API 实例并发领取任务。未配置 `REDIS_ADDR` 时使用 no-op locker，便于本地 mock smoke test 在没有 Redis 的环境继续运行。

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
http://localhost:25174
```

如果 `25174` 已被占用，脚本会选择下一个空闲端口，并在 `deploy-local/run/frontend-admin.port` 记录真实端口。Admin dev server 使用 Vite proxy，将浏览器里的 `/api/*` 转发到 `ADMIN_API_BASE_URL`，默认是 `http://127.0.0.1:28001`。

Web：

```bash
bash deploy-local/tools/run-web-host-local.sh start
bash deploy-local/tools/run-web-host-local.sh status
bash deploy-local/tools/run-web-host-local.sh stop
```

Web 默认访问：

```text
http://localhost:23001
```

如果 `23001` 已被占用，脚本会选择下一个空闲端口，并在 `deploy-local/run/frontend-web.port` 记录真实端口。Web 当前是无依赖静态 SPA，浏览器直接访问 `http://127.0.0.1:28001` 的 `/web/*` API；`backend-api` 已对 localhost origin 开启本地 CORS。

本地鉴权边界：

```text
INTERNAL_SECRET=local-dev-internal-secret
ADMIN_API_TOKEN=
WEB_API_TOKEN=
```

`ADMIN_API_TOKEN` 或 `WEB_API_TOKEN` 为空时，对应接口保持本地开发免 token；配置后，`/admin/*` 需要 `X-Admin-Token` 或 `Authorization: Bearer ...`，`/web/*` 需要 `X-Web-Token` 或 `Authorization: Bearer ...`。Admin dev server 会从 `deploy-local/.env` 注入 `VITE_ADMIN_API_TOKEN`；Web 静态页可通过浏览器 localStorage 设置 `qiyuan.webToken`。

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

Worker 是用户本机执行节点，不放进 Docker。它会使用本机 Python、Playwright 和 Chromium，并把配置、设备信息和浏览器 profile 放在用户系统应用数据目录。默认平台地址来自 `WORKER_SERVER_URL`，未配置时使用 `http://127.0.0.1:28001`。常驻 Worker 的 stdout/stderr 会写入：

```text
deploy-local/logs/worker-local.log
```

日志和运行状态：

```text
deploy-local/logs/backend-api.log
deploy-local/logs/worker-local.log
tmux session: qiyuan-backend-api-local
tmux session: qiyuan-worker-local
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

脚本会启动 MySQL/Redis、应用 schema、在 `:38001` 临时启动 MySQL 模式 API，并验证 pairing、heartbeat、job claim、checkpoint、artifact、本地文件 artifact 上传下载、PDF artifact 入解析队列、internal parser claim/parsed 写回、manual action resolve、列表查询、设备撤销和 revoked token 拒绝。

Browser Agent 本地 demo：

```bash
bash deploy-local/integration-test/30-browser-agent-demo.sh
```

该脚本要求本地 API 已启动，Worker 已完成 `init` 和 `pair`，且本机 Python 环境可使用 Playwright/Chromium。脚本会创建 `generic.browser.agent` job，让 Worker 控制 Chromium 打开 `deploy-local/mock/browser-agent-demo.html`，执行搜索、抽取结果，并校验 run completed、结果数量、`agent_trace` artifact 和最终截图 artifact。

默认脚本使用 headless Chromium，所以终端只会输出通过结果，不会弹浏览器。需要肉眼看浏览器动作时使用：

```bash
HEADED=1 PAUSE_AFTER_SECONDS=10 bash deploy-local/integration-test/30-browser-agent-demo.sh
```

## 四、完整本地验收流程

### 1. 环境和配置检查

```bash
cd deploy-local
./tools/run-infra-local.sh status
./tools/run-api-host-local.sh status
./tools/run-worker-host-local.sh status
./tools/run-worker-host-local.sh doctor
curl -fsS http://127.0.0.1:28001/healthz
```

检查 `deploy-local/.env` 至少包含：

```text
API_ADDR=:28001
MYSQL_DSN=qiyuan:qiyuan@tcp(127.0.0.1:23307)/qiyuan?parseTime=true&charset=utf8mb4&loc=Local
REDIS_ADDR=127.0.0.1:26380
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

`10` 验证内存/当前 API 的 job/run/artifact 生命周期。`20` 验证 MySQL/Redis 持久化、artifact 文件上传下载、PDF artifact 入解析队列、internal parser 写回、manual action resolve、设备撤销和 revoked token 拒绝。

### 5. 跑 Browser Agent 可视化 demo

```bash
HEADED=1 PAUSE_AFTER_SECONDS=10 ./integration-test/30-browser-agent-demo.sh
```

期望输出：

```text
browser agent demo passed job=... run=... artifacts=['agent_trace', 'screenshot']
```

### 6. 验证 artifact 和本地执行记录

用脚本输出的 `run_id` 查看 artifact：

```bash
curl -s "http://127.0.0.1:28001/admin/automation/runs/<run_id>/artifacts" | python3 -m json.tool
```

下载 PDF：

```bash
curl -L "http://127.0.0.1:28001/admin/automation/artifacts/<artifact_id>/download" -o /tmp/qiyuan-demo.pdf
file /tmp/qiyuan-demo.pdf
```

Worker 本地任务记录在：

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
curl -s "http://127.0.0.1:28001/admin/automation/manual-actions?status=pending&limit=5" | python3 -m json.tool
```

resolve：

```bash
curl -X POST "http://127.0.0.1:28001/admin/automation/manual-actions/<manual_action_id>/resolve" \
  -H "Content-Type: application/json" \
  -d '{"status":"resolved","payload":{"resolved_by":"local-demo"}}'
```

## 五、当前边界

1. `backend-api` 未配置 `MYSQL_DSN` 时使用内存 repository；配置后会使用 MySQL repository。
2. MySQL schema 已在 `database/init.sql` 和 `database/migrations/` 中定义，`deploy-local/tools/db-apply.sh` 可应用基线和迁移。
3. Redis 已接入 job claim 短锁；未配置时不会影响本地 mock 流程。
4. Worker 已支持 automation runtime、mock adapter 和 `generic.browser_agent` PoC。
5. Admin 已有 Automation 调试控制台首版；Web 已有本地文献检索任务入口；Gateway 和 AI Engine 的本地启动脚本后续补齐。
