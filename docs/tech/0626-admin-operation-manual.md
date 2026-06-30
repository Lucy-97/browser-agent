# QIYUAN Admin 操作手册

> **⚠️ 架构解耦声明 (2026-06-27)**
> 
> 根据最新的 [0627-browser-agent-automation-brd.md](../brd/0627-browser-agent-automation-brd.md) 业务解耦策略：
> - **AI 4 Science 相关业务**（文献抓取 `qiyuan.google_scholar`、PDF 解析、知识图谱等）已拆分至独立的 `feature/qiyuan` 分支。
> - 当前的 **`feature/browser-agent` 分支** 仅专注于通用底层基础能力（Playwright、调度机制）以及泛自动化运营场景（如版权监测、社交媒体维护）。
> 
> *注：本文档部分内容可能包含早期混杂的文献检索或知识图谱示例，请结合上述分支隔离原则进行阅读，具体实现以各自分支的代码为准。*



> 版本：2026-06-26
> 管理地址：`http://localhost:25174`
> 技术栈：Next.js 15 App Router + TailwindCSS 4
> 面向角色：平台运维 / 管理员

---

## 一、系统概览

QIYUAN 是面向材料科学领域的知识图谱自动化构建平台。frontend-admin（管理控制台）面向平台运维人员，提供任务监控、执行记录追踪、设备管理、人工干预、文献管理和知识图谱查询等运维能力。

科研人员请使用 frontend-web（参见 `web-operation-manual.md`）。

**导航结构**：左侧 Sidebar 提供 6 个页面入口，顶部 Topbar 显示"QIYUAN 管理台 — 自动化控制台"标题、当前时间和刷新/模拟任务按钮。

| 导航项 | 路由 | 图标 | 作用 |
|--------|------|------|------|
| 任务 | `/jobs` | 📋 | 自动化任务管理 |
| 执行 | `/runs` | 📊 | Worker 执行记录 |
| 设备 | `/devices` | 💻 | Worker 设备管理 |
| 人工干预 | `/manual` | 🛡 | 待人工处理的操作 |
| 文献 | `/literature` | 📄 | 文献解析结果管理 |
| 知识图谱 | `/graph` | 🌐 | 图谱可视化与查询 |

---

## 二、页面说明

### 2.1 Jobs（任务管理）— `/jobs`

**作用**：查看所有自动化任务，支持创建模拟任务用于测试。

**顶部指标**：

| 指标 | 说明 |
|------|------|
| 排队任务 | 状态为 `queued` 的任务数量 |

**表格字段**：

| 字段 | 说明 |
|------|------|
| 任务 ID | 任务唯一标识（长 ID，悬浮显示完整） |
| 状态 | `queued` / `running` / `completed` / `failed` |
| 类型 | `qiyuan.literature.search` / `qiyuan.browser_agent` / `qiyuan.manual_upload` / `generic.browser.script` |
| 适配器 | Worker 适配器名称（如 `google_scholar`、`mock.echo`） |
| 优先级 | 任务优先级数值 |
| 创建时间 | 任务创建时间 |
| 输入 | 任务输入参数摘要（JSON 缩略） |

**操作**：

| 按钮 | 说明 |
|------|------|
| **刷新** | 重新加载任务列表 |
| **模拟任务** | 创建一个 `generic.browser.script` 类型的 mock 任务（用于测试 Worker 连通性） |

**API 调用**：`GET /admin/automation/jobs?limit=50`

---

### 2.2 Runs（执行记录）— `/runs`

**作用**：查看 Worker 的执行记录，选中某条记录后右侧展示详情面板。

**布局**：左右分栏（split-view）——左侧执行记录列表，右侧执行详情。

**顶部指标**：

| 指标 | 说明 |
|------|------|
| 运行中 | 状态为 `running` 的执行记录数量 |

**左侧表格字段**：

| 字段 | 说明 |
|------|------|
| 执行 ID | Run 唯一标识 |
| 状态 | `pending` / `running` / `completed` / `failed` / `cancelled` |
| 任务 ID | 关联的 Job ID |
| 设备 | 执行该 Run 的 Worker 设备 ID |
| 开始时间 | Run 开始时间 |
| 心跳 | Worker 最后心跳时间 |

**右侧详情面板（选中 Run 后加载）**：

| 面板 | 内容 | 数据来源 |
|------|------|---------|
| **Timeline** | Browser Agent 的 Trace Steps（操作轨迹）或 Checkpoints（检查点），显示步骤名、action、index 和时间 | `GET .../trace` + `GET .../checkpoints` |
| **Artifacts** | Run 产出的文件列表（类型、大小、创建时间、本地路径），支持点击下载 | `GET .../artifacts` |
| **Manual Actions** | 该 Run 产生的人工干预请求（类型、状态、消息、创建时间） | `GET .../manual-actions` |

**操作**：

| 按钮 | 说明 |
|------|------|
| **取消** | 取消正在运行的 Run（仅 `running` 状态可操作），发送 `POST .../cancel` |

**API 调用**：
- 列表：`GET /admin/automation/runs?limit=50`
- 详情：`GET /admin/automation/runs/{runID}/artifacts` / `checkpoints` / `manual-actions` / `trace`

---

### 2.3 Devices（设备管理）— `/devices`

**作用**：管理已配对的本地 Worker 设备，查看设备状态和吊销设备令牌。

**表格字段**：

| 字段 | 说明 |
|------|------|
| 设备 ID | 设备唯一标识 |
| 状态 | `active` / `revoked` |
| 名称 | 设备名称（如主机名） |
| 平台 | 操作系统（如 `darwin`、`linux`） |
| 版本 | Worker 版本号 |
| 能力 | 设备支持的能力列表（逗号分隔，如 `browser_agent, literature_search`） |
| 最后上线 | 最后心跳时间 |

**操作**：

| 按钮 | 说明 |
|------|------|
| **撤销** | 吊销设备令牌，禁止该 Worker 继续拉取任务（仅 `active` 状态可操作） |

**API 调用**：
- 列表：`GET /admin/worker/devices?limit=50`
- 吊销：`POST /admin/worker/devices/{deviceID}/revoke`

---

### 2.4 Manual（人工干预）— `/manual`

**作用**：查看需要人工介入的操作请求，运维人员可手动解决。

**顶部指标**：

| 指标 | 说明 |
|------|------|
| 待人工处理 | 状态为 `pending` 的人工干预数量 |

**表格字段**：

| 字段 | 说明 |
|------|------|
| 操作 ID | ManualAction 唯一标识 |
| 状态 | `pending` / `resolved` / `expired` |
| 类型 | 干预类型（如 `captcha`、`file_select`、`confirmation`） |
| 执行 ID | 关联的 Run ID |
| 消息 | 干预描述信息 |
| 创建时间 | 干预请求创建时间 |

**操作**：

| 按钮 | 说明 |
|------|------|
| **处理** | 标记为已解决（仅 `pending` 状态可操作），发送 `POST .../resolve` |

**典型场景**：Browser Agent 访问网页时遇到 CAPTCHA 验证码，Worker 提交人工干预请求，运维人员在此页面手动完成验证后标记为已处理。

**API 调用**：
- 列表：`GET /admin/automation/manual-actions?limit=50`
- 处理：`POST /admin/automation/manual-actions/{actionID}/resolve`

---

## 四、典型操作流程

### 流程 1：监控爬取任务执行

```text
1. 打开 http://localhost:25174（默认跳转到 /jobs）
2. 在 Jobs 页面查看任务列表，确认目标任务状态
3. 切换到 /runs 查看执行记录，点击某条 Run 查看右侧详情
4. 在 Timeline 中查看 Browser Agent 的操作轨迹
5. 在 Artifacts 中查看和下载产出的 PDF 文件
6. 如 Run 异常，可点击「取消」终止执行
```

### 流程 2：处理人工干预

```text
1. 打开 http://localhost:25174/manual
2. 查看「待人工处理」指标，确认有多少干预请求
3. 找到状态为 pending 的请求（通常是 CAPTCHA 验证）
4. 根据消息描述完成人工操作
5. 点击「处理」按钮标记为已解决
6. Worker 收到解决信号后继续执行
```

### 流程 5：管理 Worker 设备

```text
1. 打开 http://localhost:25174/devices
2. 查看已配对的 Worker 设备列表
3. 确认设备状态、能力和最后心跳时间
4. 如设备异常或不再使用，点击「撤销」吊销令牌
```

---

## 五、API 代理规则

frontend-admin 通过 Next.js rewrites 代理后端请求：

| 前端路径前缀 | 代理目标 | 说明 |
|-------------|---------|------|
| `/api/*` | Go API（默认 `127.0.0.1:28001`） | 平台自动化/Worker/文献管理接口 |
| `/web/*` | Go API（默认 `127.0.0.1:28001`） | 平台 Web 接口 |
