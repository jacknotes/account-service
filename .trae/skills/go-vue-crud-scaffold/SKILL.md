---
name: "go-vue-crud-scaffold"
description: "按本项目最佳实践一键生成可运行的 Go+Gin+MySQL 后端 + Vue3+Element Plus 前端的全栈 CRUD 应用骨架（目录、Dockerfile、compose、配置、示例代码）。当用户要开始一个新的 Go+Vue 全栈业务应用（记账/后台/CRUD/工单等）或要求'按这个项目搭一个类似的'时使用。"
---

# Go + Vue 全栈业务应用脚手架

基于 `account-service` 项目沉淀的可运行骨架与工程最佳实践。目标是让「开始一个新业务应用」时能一次性拉出正确分层、部署配置与踩坑要点，避免重犯已解决的问题。

## 何时使用

- 用户要新建一个 Go + Gin + MySQL + Vue3 全栈业务应用（记账、后台管理、CRUD、工单等任何前后端分离项目）。
- 用户说「按这个项目搭一个类似的」「像 account-service 那样」「复用这套架构」。
- 需要生成项目目录骨架、Dockerfile、docker-compose、环境变量模板、分层示例代码。

> 这不是生成完整业务功能，而是产出「可靠、可部署、符合既有约定」的项目骨架与关键示例代码，再按具体业务填充。

## 技术栈基准（锁定版本，勿随意升级）

- 后端：**Go 1.24 + Gin v1.9**、`database/sql` + `go-sql-driver/mysql`（不要引入 ORM，保持轻量）
- 数据库：**MySQL 5.7**（唯一持久存储；字符集 `utf8mb4`，排序 `utf8mb4_unicode_ci`）
- 认证：`golang-jwt/jwt/v5`、`golang.org/x/crypto`(bcrypt)；可选 `pquerna/otp`(TOTP)
- 前端：**Vue 3 + Vite 5 + Element Plus** + `unplugin-auto-import`/`unplugin-vue-components` 按需引入；图表用 ECharts 6（若需要）
- 部署：多阶段 Dockerfile（`node:20-alpine` → `golang:1.24-alpine` → `alpine:3.20`）

## 目录结构（照此搭建）

```
├── main.go                  # 入口：路由注册、中间件、静态托管、优雅关闭
├── config/config.go         # 环境变量加载与校验（集中管理，含默认值与必填校验）
├── internal/
│   ├── database/            # MySQL 数据访问 + 版本化迁移（schema_migrations）
│   ├── handlers/            # API 处理器（含输入校验、错误统一映射）
│   ├── middleware/          # 认证、限流、request-id 日志
│   ├── models/              # 数据模型
│   └── service/             # 服务接口（可选；小项目可直接 handlers 调 database）
├── frontend/                # Vue3 + Vite 前端；构建产物 frontend/dist 由后端静态托管
├── scripts/init-mysql.sql   # 首次建库（仅 CREATE DATABASE；表结构交给应用迁移）
├── Dockerfile               # 多阶段构建（详见下文）
├── docker-compose.yml       # 生产/部署栈（MySQL 不暴露宿主机端口）
├── docker-compose.infra.yml # 仅本地开发基础依赖（可暴露端口+弱口令，勿用于生产）
└── .env.example / .env      # 环境变量模板与真实配置
```

## 后端分层与关键约定

- **配置集中**在 `config.Load()`：所有环境变量一个入口，必填项校验失败直接报错退出；提供默认值与超时解析。见 `config/config.go`。
- **版本化迁移**：`database.go` 内置迁移（`schema_migrations` 记录版本），应用启动自动建表/升级字段，无需手动建表。删除用户时用**级联删除**其关联数据（外键 FK）。
- **金额一律用「分」整数**（字段 `amount_cents`），杜绝浮点精度问题；正数收入、负数支出。
- **分层接口**：handler → database 直连（或经 service）。handler 只做输入校验、鉴权、错误映射（如分类重名映射 409、`ErrDuplicateCategory`），不写裸 SQL。
- **中间件顺序**（main.go 里依次注册）：`Recovery → RequestID → AccessLog → (全局限流) → (认证) → (管理员) → 路由处理器`。
- **健康探针**：`/healthz`（存活）+ `/readyz`（含 MySQL Ping，失败返回 503）。compose 用它做健康检查。
- **优雅关闭**：监听 SIGINT/SIGTERM，`srv.Shutdown(ctx)` 带 10s 超时。
- **静态托管**：`如果 frontend/dist 存在则挂载静态，否则返回提示`，保证未构建也能启动后端。

## 认证与安全要点（务必沿用）

- **密码 bcrypt**：拒绝 >72 字节，避免静默截断；强度规则：≥8 位 + 大小写 + 数字 + 特殊字符。
- **JWT 双 token**：`access_token` 15 分钟 + `refresh_token` 7 天；`/refresh` 采用**轮换制（旧 token 即刻失效）**。
- **撤销机制**：登出/改密后 access token 入黑名单（表 `token_blacklist`，15 分钟内失效）；refresh token 存 **SHA-256 哈希**并支持服务端删除。
- **SQL 安全**：全部参数化；LIKE 关键字需正确转义与 `ESCAPE`（历史坑：反斜杠被折叠导致 500）；排序字段用**白名单**，杜绝注入。
- **数据隔离**：所有查询带 `user_id`，用户表与记录表建外键。
- **限流**：登录限流（1 req/3s，burst 3）+ 全局限流；账号锁定（5 次失败锁 5 分钟）存应用内存（单实例限制，多实例需外接限流）。

## 前端约定（Vue3 + Element Plus）

- `unplugin-auto-import` + `unplugin-vue-components` 实现 Element Plus 组件自动按需引入（无需手动 import）。
- 封装 `utils/chart.js` 统一 ECharts 主题（暗色+金色渐变）与 resize 处理。
- **ECharts 6 坑**：默认 legend 位置改为底部居中，会与 x 轴标签重叠 → **显式设置 `legend: { top: ... }`**。
- 401 自动续期：access_token 过期时自动调 `/refresh` 换新 token 后重放请求。
- 深色主题为主，用 Element Plus CSS 变量覆盖；主题切换需同时同步 `body.theme-light`。
- UI 风格偏好：暗色 + 金色渐变（Style C）。

## Dockerfile（多阶段，最小运行镜像）

```
frontend 阶段: node:20-alpine，COPY package*.json → npm install → COPY frontend → npm run build
builder 阶段 : golang:1.24-alpine，GOPROXY=goproxy.cn，CGO_ENABLED=0，COPY go.mod/sum → go mod download → COPY . → go build -ldflags="-s -w"
runtime 阶段: alpine:3.20，COPY binary + frontend/dist，adduser appuser 降权运行，EXPOSE 8081
```

要点：设置 `GOPROXY=https://goproxy.cn,direct`；`CGO_ENABLED=0` 纯 Go 编译；`-ldflags="-s -w"` 瘦身；非 root 用户运行。

## docker-compose 部署约定

- MySQL 服务**不暴露端口到宿主机**，仅容器内网访问（避免公网暴露）；应用通过服务名 `mysql:3306` 连接。
- 应用需环境变量（compose 里强制校验）：`MYSQL_DSN`、`JWT_SECRET`(≥32 位)、`MYSQL_ROOT_PASSWORD`、`MYSQL_USER`、`MYSQL_PASSWORD`，用 `${VAR:?错误提示}` 强制。
- 应用与 MySQL 都设 **`TZ=Asia/Shanghai`**，避免时间字段差 8 小时。
- 两服务都配 healthcheck；应用 `depends_on: mysql: condition: service_healthy`。
- **固定 Docker 网段**（防止 `docker compose down` 重建网络后网关 IP 变化）：
  ```yaml
  networks:
    account-net:
      driver: bridge
      ipam:
        config:
          - subnet: 172.28.0.0/24
            gateway: 172.28.0.1
  ```
- 数据持久化用命名卷 `mysql_data`；`restart: unless-stopped`。
- **镜像 tag 用真实的 v2 而非 latest**，避免拉到过期 Hub 镜像。

## 环境变量（config 读取的完整清单）

| 变量 | 说明 | 默认 |
|------|------|------|
| MYSQL_DSN | 连接串（必填）| - |
| JWT_SECRET | ≥32 位随机串（必填）| - |
| PORT | 端口 | 8081 |
| FRONTEND_DIR | 前端目录 | ./frontend/dist |
| ALLOWED_ORIGINS | CORS（`*` 全放）| `*` |
| TRUSTED_PROXIES | 可信反代 IP（逗号分隔）| 空 |
| HTTP_READ/WRITE/IDLE_TIMEOUT | 超时 | 10s/10s/60s |

`.env` 生成密钥：`openssl rand -base64 48`。

## 反代与真实 IP（TRUSTED_PROXIES）——高频坑

- 反代（Nginx/Traefik）在宿主机、应用在容器时，应用看到的直连是 **Docker 网桥网关 IP**（不是 127.0.0.1）。
- `TRUSTED_PROXIES=127.0.0.1` 无效，应填网关 IP；网段动态变化时固定网段（见上）。
- 配错症状：日志全记同一内网 IP、不同用户共享限流配额触发 429。
- Nginx 必须透传：`proxy_set_header X-Real-IP $remote_addr; proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`
- 常见组合：`TRUSTED_PROXIES=127.0.0.1,172.16.0.0/12,192.168.0.0/16`（按实际网段裁剪）。

## 数据运维

- **备份**：`mysqldump --single-transaction --skip-lock-tables --default-character-set=utf8mb4`。
- **导入**：目标库为空时 `< backup.sql`（dump 含建表与数据，含 schema_migrations 版本）。备份目录加入 `.gitignore`。
- **管理员忘记密码**：bcrypt 无法找回，只能重置——用 python `bcrypt.hashpw` 生成新哈希直更 DB，并 `DELETE FROM refresh_tokens WHERE user_id=...` 清会话。改库不走应用流程，已签发 access token 无法批量拉黑（自然 15 分钟过期）。
- **登录提示"过于频繁"** = 连续失败触发的账号锁定（5 次锁 5 分钟，应用内存）或登录限流，等 5 分钟或重启容器即可。

## 本机环境特殊注意（Windows + WSL2 用户）

- MySQL 本地 3306 常被 VMware NAT 占用，开发时 `docker-compose.infra.override.yml` 把端口映射到 **3307**。
- WSL 未启用 systemd，Docker daemon 需手动 `sudo service docker start`；重启后 IP 可能变化，需同步改 MYSQL_DSN。
- Docker daemon 不继承 shell 代理 → 拉公共镜像需显式注入 `HTTPS_PROXY/http_proxy` 重启 daemon，或用国内加速（`hub.rat.dev`、`docker.m.daocloud.io`）。
- 目标机器为 x86_64 才能跑 MySQL 5.7 镜像。
- `.dockerignore` 排除 `backups/ docs/ .omo/ scripts/*.sql`，减小构建上下文。

## 验证

骨架搭建完成后运行：
- `go build ./... && go test ./...`
- `cd frontend && npm install && npm run build`
- `docker compose up -d --build`，访问 `/app/`，检查 `/healthz`、`/readyz`
- 用「首个注册用户自动成为管理员」规则做一遍注册→登录→CRUD 冒烟