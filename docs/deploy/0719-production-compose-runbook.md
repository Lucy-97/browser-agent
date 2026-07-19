# Browser Agent 生产 Compose 部署运行手册

## Changelog

- 2026-07-19：建立首个受支持的生产部署入口，覆盖不可变镜像、环境校验、文件 Secret、托管 MySQL/Redis TLS、数据库迁移、健康检查、资源限制、部署与按 SHA 回滚。

## 1. 适用范围与当前状态

本手册适用于单区域 Linux `amd64` 主机上的 staging 和首期封闭内测控制面。受支持入口为 `deploy/production/compose.yaml`；`deploy/k3s/` 当前仅是实验草案，不得用于客户生产环境。

该基线会部署 Web、Gateway 和 API，数据库与 Redis 必须使用外部托管服务。Admin 未纳入公网生产服务，Worker 继续运行在客户电脑，并仅通过 HTTPS 出站连接控制面。

```text
客户浏览器
  -> HTTPS / WAF / TLS 终止层
  -> 127.0.0.1:3000 Web
  -> Compose 内部 Gateway:8080
  -> Compose 内部 API:8001
  -> 托管 MySQL（验证 TLS）/ 托管 Redis（验证 TLS）
```

## 2. 上线前准备

必须准备以下资源：

1. 一台安装 Docker Engine 和 Docker Compose v2 的 Linux `amd64` 主机。
2. 一个已解析到入口层的正式域名，以及 Caddy、Traefik、云负载均衡或 CDN 提供的有效 HTTPS/WAF。
3. 可通过 TLS 访问的托管 MySQL 8 数据库和托管 Redis，二者均不得直接暴露公网。
4. 可拉取 `ghcr.io/lucy-97/browser-agent-{api,gateway,web}:<commit-sha>` 的 GHCR 凭据。
5. 独立的 staging 与 production 配置、数据库、Redis、Secret 和域名；禁止复用本地开发密码。

生产镜像由 `.github/workflows/release-images.yaml` 在 `main` 更新后发布，tag 固定为完整 40 位 commit SHA，不发布或使用 `latest`。

## 3. 配置与 Secret

复制非敏感模板并创建本地 Secret 目录：

```bash
cd deploy/production
cp .env.example .env
mkdir -p secrets
chmod 700 secrets
openssl rand -base64 48 > secrets/jwt_secret.txt
openssl rand -base64 48 > secrets/internal_api_secret.txt
openssl rand -base64 36 > secrets/db_password.txt
openssl rand -base64 36 > secrets/redis_password.txt
chmod 600 secrets/*
```

按托管数据库实际信息创建 `secrets/mysql_dsn.txt`。DSN 必须开启 Go MySQL Driver 的证书和主机名校验：

```text
browser_agent:<password>@tcp(mysql.example.internal:3306)/browser_agent?parseTime=true&charset=utf8mb4&tls=true
```

然后填写 `.env`：

- `IMAGE_TAG`：已经由镜像工作流成功发布的完整 commit SHA。
- `PUBLIC_WEB_BASE_URL`：唯一正式 HTTPS 地址。
- `DB_SSL_MODE`：只允许 `VERIFY_CA` 或 `VERIFY_IDENTITY`。
- `REDIS_REQUIRED=true`、`REDIS_TLS_ENABLED=true`。
- `REDIS_TLS_SERVER_NAME`：Redis 证书覆盖的 DNS 名称。

`secrets/` 与 `.env` 已被 Git 忽略。当前文件挂载是单机 Compose 的过渡方案；正式托管环境应由云 Secret Manager 在部署时生成这些只读文件，不能把 Secret 写入仓库、镜像或 CI 日志。

## 4. 首次部署

先登录 GHCR，再校验、迁移和启动：

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
bash deploy.sh validate
bash deploy.sh migrate
bash deploy.sh deploy
bash deploy.sh status
```

`deploy` 会再次执行幂等 migration，然后只启动 `api`、`gateway`、`web`。MySQL schema 来源为 `database/init.sql` 和按文件名排序的 `database/migrations/*.sql`。

Compose 只把 Web 绑定到 `127.0.0.1:3000`。TLS 终止层必须代理到该地址，例如 Caddy：

```caddyfile
app.example.com {
  reverse_proxy 127.0.0.1:3000
}
```

不得把 API、Gateway、MySQL、Redis 或 Admin 端口直接映射到公网。

## 5. 验证与回滚

部署后至少检查：

```bash
bash deploy.sh status
bash deploy.sh logs
curl --fail --show-error https://app.example.com/
```

staging 还必须完成登录、Worker 配对、创建任务、客户本机 Worker 领取/回写以及 artifact 下载的端到端验收。

回滚时把 `.env` 的 `IMAGE_TAG` 改为上一个已发布的完整 commit SHA，再运行：

```bash
bash deploy.sh deploy
```

数据库 migration 必须保持向后兼容。存在破坏性 schema 变更时，不能仅回滚镜像，必须按该次变更单独制定数据回退方案。

## 6. 当前仍然阻断客户上线的事项

本次交付是生产部署基线，不等于已完成公开上线。以下事项未完成前，只能用于 staging：

- artifact 仍写入单机 Docker volume，尚未接入 S3/R2 对象存储和短时授权下载。
- 托管 MySQL 自动备份、恢复演练和 artifact 备份尚未验收。
- 固定域名、TLS/WAF、云 Secret Manager 和实际托管 MySQL/Redis 尚未在目标云环境落地。
- staging 的 Web → Gateway → API → 客户 Worker 全链路尚未执行线上形态 E2E。
- Admin 独立身份/RBAC、Windows Worker 签名安装包与升级机制仍在后续阶段。

完成上述门禁后，才进入受邀客户封闭内测；公共注册继续保持关闭。
