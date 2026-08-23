# 中间件选型建议（文档）

本文回答“除 MySQL 外还应使用什么中间件”，并给出针对本项目的落地建议。
按你的选择，本次只出文档、不实际集成。所有建议均以「从轻到重」排序，可逐步引入。

## 当前已使用

| 中间件 | 用途 | 状态 |
|--------|------|------|
| MySQL 5.7 | 唯一数据存储（含 access token 黑名单、refresh token、审计日志） | 已接入（本仓库默认） |
| Gin | HTTP 框架（路由、中间件、优雅关闭） | 已接入 |

> 原 Redis 已按项目要求移除：token 黑名单整合进 MySQL（`token_blacklist` 表），
> 限流/登录锁定/TOTP 计数为单实例内存实现。单实例部署无需任何额外依赖。

## 建议引入（按优先级）

### 1. Nginx / Caddy / Traefik —— 反向代理 + TLS（强烈建议，成本最低）
- 作用：HTTPS 终止、静态资源缓存、隐藏后端、统一入口。
- 落地：Nginx 配置示例见 README「TLS / 反向代理」；配套 `TRUSTED_PROXIES` 环境变量。
- 无需改代码，纯部署层。

### 2. Prometheus + Grafana —— 指标监控（推荐）
- 作用：QPS、错误率、延迟（RED 指标）、DB 连接池、GC、内存。
- 落地：在 Gin 上挂一个 `/metrics` 端点（Prometheus 官方 go client，或 `gin-prometheus` 中间件），Grafana 配面板；配合 docker-compose 里 prometheus + grafana 两个服务。
- 预估工作量：小（一个中间件 + 一个端点），收益直接。

### 3. Loki + Promtail（或直接 JSON 日志 → 采集器）—— 日志聚合
- 作用：集中查看访问日志与业务日志，便于排障与审计。
- 落地：本项目访问日志已用 slog 输出结构化字段（method/path/status/latency/request_id 等），只需把日志输出改为 JSON 格式（`slog.NewJSONHandler` 写 stdout），再用 Promtail/Fluent Bit 采集进 Loki。
- 预估工作量：很小（改一行日志 handler + 采集配置）。

### 4. 对象存储 / 本地文件 —— 报表等文件类产物落盘
- 作用：PDF/图片导出目前是纯前端生成（html2canvas + jspdf），浏览器本地完成。若需要“服务端生成报表/备份”，可加对象存储（MinIO/S3 自托管，或云厂商 OSS）。
- 适合场景：需要把报表发邮件、定时生成、多端共享时。个人记账场景通常不需要。
- 预估工作量：中（新增一个导出接口 + 存储适配）。

### 5. 消息队列（RabbitMQ / Kafka / NATS）—— **本场景不推荐**
- 理由：记账应用是典型的低并发、强一致、简单 CRUD + 聚合报表，没有明显的异步解耦需求。
- 若未来出现「记账后触发多端同步通知」「大量导入异步处理」等场景，再引入 NATS/RabbitMQ 也不迟，不要提前上 Kafka。

### 6. OpenTelemetry / Jaeger —— 链路追踪
- 作用：跨服务请求追踪。当前是单体应用，用 request-id（已内置）即可串日志；只有拆分成多服务后才需要。
- 建议：暂缓。

### 7. CI/CD 与代码质量工具（非运行时中间件，但收益高）
- golangci-lint（静态检查）、govulncheck（依赖漏洞）、GitHub Actions 跑 `go test ./...`。
- 前端引入 ESLint + Prettier。
- 这一步几乎零运维成本，建议最先做。

## 决策速查

| 你的场景 | 建议 |
|----------|------|
| 只想线上 HTTPS + 稳 | Nginx/Traefik（+ TRUSTED_PROXIES） |
| 想看到指标和告警 | Prometheus + Grafana（/metrics） |
| 想集中查日志 | JSON 结构化日志 + Loki |
| 多实例横向扩容 | 限流为各实例内存独立，可接受则无需处理；如需共享限流/黑名单状态，再引入 Redis 或外部限流网关 |
| 服务器端报表/备份 | MinIO/S3 对象存储 |
| 异步任务解耦 | 暂不需要；出现明确需求再加消息队列 |
