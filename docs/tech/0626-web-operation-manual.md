# QIYUAN Web 操作手册

> **⚠️ 架构解耦声明 (2026-06-27)**
> 
> 根据最新的 [0627-browser-agent-automation-brd.md](../brd/0627-browser-agent-automation-brd.md) 业务解耦策略：
> - **AI 4 Science 相关业务**（文献抓取 `qiyuan.google_scholar`、PDF 解析、知识图谱等）已拆分至独立的 `feature/qiyuan` 分支。
> - 当前的 **`feature/browser-agent` 分支** 仅专注于通用底层基础能力（Playwright、调度机制）以及泛自动化运营场景（如版权监测、社交媒体维护）。
> 
> *注：本文档部分内容可能包含早期混杂的文献检索或知识图谱示例，请结合上述分支隔离原则进行阅读，具体实现以各自分支的代码为准。*



> 版本：2026-06-26
> 前端地址：`http://localhost:23001`
> 技术栈：Next.js 15 App Router
> 面向角色：科研人员 / 最终用户

---

## 一、系统概览

QIYUAN 是面向材料科学领域的知识图谱自动化构建平台，覆盖从文献获取到结构化知识入库的完整链路：

```text
文献爬取 → PDF 解析 → 结构化文本预处理 → 模板驱动 LLM 抽取 → 三元组建模 → Neo4j 入库 → MCP 协议开放 → 平台 Agent 查询
```

frontend-web 面向科研人员，提供任务创建、解析结果查看和知识图谱查询功能。管理控制台请使用 frontend-admin（参见 `admin-operation-manual.md`）。

---

## 二、页面说明

### 2.1 Home（首页）— `/`

**作用**：任务创建入口，提供三种任务创建方式和本地 Worker 初始化指引。

**页面布局**：

| 区域 | 组件 | 功能 |
|------|------|------|
| **创建任务**（三栏） | `LiteratureSearchForm` | Google Scholar 文献检索：输入关键词（如 `DFT U parameter LiFePO4`），设置 Max Results 和 Max PDFs，勾选 Download PDFs / Open Chromium |
| | `BrowserAgentForm` | AI 浏览器智能体：输入目标 URL、任务描述和允许的域名，让 LLM Agent 自主访问网页执行信息提取 |
| | `PdfUploadForm` | 手动 PDF 上传：拖拽或点击上传本地 PDF（支持多文件，单文件上限 50MB），自动进入解析流水线 |
| **本地 Worker** | `WorkerSetup` | 展示本地 Worker 初始化三步命令（init → pair → start），任务由用户自有的本地 Worker 拉取执行 |

**BRD 对应关系**：

| BRD 章节 | 对应能力 |
|---------|---------|
| 二.1 文献爬取 | LiteratureSearchForm（Google Scholar 自动获取） |
| 二.1 文献爬取（人工方式） | PdfUploadForm（手动上传） |
| 二.2 PDF 解析 | PdfUploadForm 上传后自动触发 AI Engine 解析 |

---

### 2.2 Jobs（任务列表）— `/jobs`

**作用**：查看所有已创建的自动化任务及其执行状态。

**表格字段**：

| 字段 | 说明 |
|------|------|
| Job ID | 任务唯一标识（长 ID，悬浮显示完整） |
| Type | 任务类型（`qiyuan.literature.search` / `qiyuan.browser_agent` / `qiyuan.manual_upload`） |
| Status | 状态（`pending` → `running` → `completed` / `failed`） |
| Created | 创建时间 |
| Updated | 最后更新时间 |

**BRD 对应关系**：

| BRD 章节 | 对应能力 |
|---------|---------|
| 二.1 文献爬取 | 查看文献检索任务的执行进度 |
| 三 建议技术链路 | 追踪所有任务类型的生命周期 |

---

### 2.3 Results（解析结果）— `/results`

**作用**：查看 PDF 解析结果，展开查看 LLM 抽取的三元组和 Neo4j 入库状态。

**表格字段**：

| 字段 | 说明 |
|------|------|
| （展开箭头） | 点击展开详情面板 |
| Title | 论文标题 |
| Status | 解析状态（`pending` / `parsing` / `parsed` / `failed`） |
| Neo4j | 图谱入库状态（`pending` / `synced` / `skipped` / `failed`） |
| Year | 发表年份 |
| DOI | 数字对象标识符 |
| Updated | 更新时间 |

**展开详情（点击任意行）**：

- **Neo4j 状态标签**：绿色 = 已入库，黄色 = 待处理，红色 = 失败
- **LLM 抽取三元组表格**：展示模板驱动抽取结果（material / element / u_value / unit / method / context / confidence 等字段）
- **原始文本预览**：可折叠查看 PyMuPDF/MinerU 提取的结构化文本（前 2000 字符）

**BRD 对应关系**：

| BRD 章节 | 对应能力 |
|---------|---------|
| 二.2 PDF 解析与结构化文本预处理 | 原始文本预览区域 |
| 二.3 模板驱动的三元组抽取 | LLM 抽取三元组表格 |
| 四.1 文献获取与 PDF 预处理管道 | 批量解析结果列表 |
| 四.2 可配置抽取模块 | 展开详情中的抽取结果和模板信息 |

---

## 四、典型操作流程

### 流程 1：Google Scholar 文献检索 → 自动解析

```text
1. 打开 http://localhost:23001
2. 在 "New Search" 面板输入关键词（如 "DFT U parameter LiFePO4"）
3. 设置 Max Results = 10, Max PDFs = 3
4. 勾选 "Download PDFs" 和 "Open local Chromium window"
5. 点击 "Create Job"
6. 切换到 /jobs 查看任务状态
7. Worker 拉取任务 → 打开 Chromium → 搜索 Google Scholar → 下载 PDF
8. AI Engine poll_loop 自动领取解析任务 → PyMuPDF 解析 → MinerU 结构化 → LLM 抽取 → Neo4j 入库
9. 切换到 /results 查看解析结果，点击展开查看三元组和入库状态
10. 切换到 /graph 查看知识图谱，使用 Cypher 查询引擎验证数据
```

### 流程 2：手动上传 PDF → 自动解析

```text
1. 打开 http://localhost:23001
2. 在 "Upload PDF" 面板拖拽或点击上传本地 PDF 文件
3. 点击 "Upload" 按钮
4. 上传成功后显示 result_id，点击可跳转到 /results
5. AI Engine poll_loop 自动解析（同流程 1 步骤 8-10）
```

### 流程 3：知识图谱查询

```text
1. 打开 http://localhost:23001/graph
2. 页面自动加载图谱全量数据（节点 + 关系），中央显示力导向图
3. 在左上「节点检索」输入关键词（如 Fe2O3）→ 回车，匹配的节点在图谱中高亮
4. 在左上「Cypher 查询引擎」输入 Cypher（如 MATCH (m:Material)-[:HAS_ELEMENT]->(e:Element {name:"Fe"}) RETURN m, e LIMIT 20）→ 执行查询
5. 查询结果在面板表格中展示，匹配的图谱节点同步高亮
6. 点击图谱中的任意节点 → 右侧详情面板显示属性 + 关联关系
7. 底部类型过滤 chips 可隐藏/显示特定节点类型，缩放控制可调整视图
```

---

## 五、API 代理规则

frontend-web 通过 Next.js rewrites 代理后端请求：

| 前端路径前缀 | 代理目标 | 说明 |
|-------------|---------|------|
| `/web/*` | Go API（默认 `127.0.0.1:28001`） | 平台业务接口 |

---

## 六、待确认事项当前状态

| BRD 第六节待确认项 | 当前状态 |
|-------------------|---------|
| 文献来源优先人工还是爬虫 | 两种方式均已实现（Google Scholar 自动 + PDF 上传手动 + Browser Agent） |
| PDF 解析是否正式采用 MinerU | 已集成 MinerU，同时保留 PyMuPDF 作为降级方案 |
| DFT+U 三元组最小字段集 | 8 字段全部实现（material/element/u_value/unit/method/context/doi/confidence） |
| Neo4j 查询接口支持的典型问题 | Cypher 通用查询 + 4 个领域专用 REST 接口（参数推荐/范围校验/出处追溯/证据查询） |
| MCP 调用方、鉴权、返回格式 | Streamable HTTP（主推）+ SSE（降级兼容）双传输已实现，鉴权方式待定 |
| 交付节奏和截止时间 | 待定 |
