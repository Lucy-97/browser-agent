# 微信群资料同步 Agent 技术方案

## Changelog

- 2026-07-02：新增第一版方案，明确以 `weixin-agent-sdk` 作为微信入口，先实现本地文件归档与 manifest，再逐步接入后端资料库。
- 2026-07-03：切换到本机微信客户端同步方向，新增 `weixin.desktop_sync` Worker 任务，先扫描本机微信下载/附件目录，再逐步接入桌面端 UI 自动化和群选择。

## 目标

基于本机微信客户端做一个本地同步 agent，用于把微信群聊中的文件资料同步到启元资料库。第一阶段先解决本机已下载资料的扫描、落盘、artifact 和 manifest；第二阶段再接入桌面端 UI 自动化，实现选择群、打开群文件、触发下载和增量同步。

## 第一阶段边界

- Web 下发 `weixin.desktop_sync` 任务，本地 Worker 扫描用户指定的微信下载/附件目录。
- 支持用群关键词过滤文件路径；留空时扫描目录下所有新增文件。
- 每个同步文件会作为 `weixin_file` artifact 上传，并生成 `weixin_manifest` artifact，记录原始路径、文件名、MIME、大小和修改时间。
- 暂不直接写数据库，不绕过 `go-api` 的数据 ownership。
- 暂不读取微信聊天数据库，不采集未下载的历史消息正文。

## 当前落地

新增能力：

```bash
worker/local-cli/qiyuan_worker/adapters/weixin/desktop_sync.py
```

Web 端下发任务：

```bash
POST /web/automation/weixin-desktop-sync-jobs
{
  "source_dirs": ["$HOME/Library/Containers/com.tencent.xinWeChat/Data"],
  "group_keywords": ["科研群"],
  "max_files": 200
}
```

Worker artifact：

```bash
weixin_file
weixin_manifest
```

当前 MVP 是“扫描本机已落盘资料”，不是直接破解/读取微信聊天库。下一阶段用桌面端 UI 自动化补齐“选择群、打开群文件列表、触发下载、再扫描”的链路。

## 后续接入建议

第二阶段建议在 `backend-api` 增加一个受控入口，而不是让微信 agent 直接写库：

- `POST /worker/weixin-sync/files` 或复用 automation run artifact 上传接口。
- 请求使用设备 token 或 internal secret 做鉴权。
- 后端负责创建资料记录、去重、索引、权限绑定和审计日志。
- 微信 agent 只负责本地缓存、重试队列和上传状态 manifest。

第三阶段再根据业务选择是否自动触发：

- 文档解析、OCR、向量化入库。
- 群聊上下文摘要。
- 按群或发送者建立资料集合。
- 失败文件的补偿重试和人工确认。

## 风险与注意

- `weixin-agent-sdk` 是非官方项目，应限定为内部个人账号辅助工具，避免承诺官方稳定性。
- 群文件可能包含隐私或第三方版权资料，需要明确群内授权、访问范围和删除机制。
- 微信登录态、`context_token` 和长轮询稳定性依赖 SDK 实现，生产化前需要补运行监控和错误恢复。
- 资料入库前需要做文件大小、类型、病毒扫描、重复文件 hash 和权限校验。
