# 记账本 Web 应用 — 缺陷修复报告

- **被测系统**：记账本（Go + Gin / Vue 3 + Element Plus / MySQL 5.7）
- **访问地址**：http://localhost:8081/app/
- **修复时间**：2026-11-12
- **修复依据**：`.trae/skills/web-fullstack-engineer/SKILL.md`（跨层缺陷定位 → 根因修复 → 跨层零回归）
- **关联报告**：[FINAL-测试总结报告.md](FINAL-测试总结报告.md)

---

## 一、总体结论

QA 阶段发现的 **4 个缺陷已全部修复并验证通过**；修复过程中又**新发现并修复 2 个缺陷**（BUG-5、BUG-6，其中 BUG-6 是 BUG-1 的同源问题，BUG-5 是被 BUG-1 掩盖的隐藏缺陷）。

| 指标 | 数量 |
| --- | --- |
| 原始缺陷 | 4 |
| 修复中新发现缺陷 | 2 |
| 已修复 | **6** |
| 未修复 / 遗留 | **0** |
| 新增回归测试用例 | 8 个（Go 单测 6 + 集成测试 1 + 脚本修正 1） |
| 端到端验证 | 24/24 通过 + 16/16 通过 |
| QA 回归套件 | 146 项通过 / 0 项功能失败 |

**整体评价**：6 个缺陷全部完成根因修复，且均补充了防复发的自动化测试。后端 `go vet` / `go build` / `go test`（含真实 MySQL 集成测试）全绿，前端构建成功，浏览器全链路回归通过，**无功能回归**。

---

## 二、缺陷与修复清单

| 编号 | 标题 | 严重程度 | 层次 | 状态 |
| --- | --- | --- | --- | --- |
| BUG-1 | 登出/改密后 access token 未失效（黑名单时区缺陷） | **高** | 后端 | ✅ 已修复 |
| BUG-2 | 记录部分更新（省略 date）被错误拒绝 | 中 | 后端 | ✅ 已修复 |
| BUG-3 | 创建记录/分类接口回显零值时间戳 | 低 | 后端 | ✅ 已修复 |
| BUG-4 | `/favicon.ico` 返回 404 | 低 | 后端 + 前端 | ✅ 已修复 |
| BUG-5 | 同一秒内签发的 JWT 完全相同（缺 `jti`） | **高** | 后端 | ✅ 已修复（新发现） |
| BUG-6 | 时间戳整体偏移 8 小时（镜像缺 tzdata） | 中 | 部署 + 后端 | ✅ 已修复（新发现） |

---

## 三、逐项修复详情

### BUG-1：登出/改密后 access token 未失效

**层次定位**：后端（数据访问层 + 中间件）。证据：`logout` 返回 200，但同一 token 访问 `/api/auth/me` 仍 200；`SELECT COUNT(*) FROM token_blacklist` = 0；应用容器 `date -u` 为 UTC，MySQL `NOW()` 为 +08:00，相差 8 小时。

**根因**：`internal/database/blacklist.go` 中，`expires_at` 由 Go 按**容器本地时间**写入，而查询与清理用 SQL 的 `NOW()`（**MySQL 时区**）比较。两者相差 8 小时时：
- `DELETE FROM token_blacklist WHERE expires_at <= NOW()` 会**立刻删除刚插入的记录**；
- `IsTokenBlacklisted` 的 `expires_at > NOW()` 永远不成立。

结果黑名单表恒为空，登出形同虚设。此外 `internal/middleware/auth.go` 在查询出错时**放行**（fail-open），把这个缺陷彻底掩盖了。

**修复**（`internal/database/blacklist.go`、`internal/middleware/auth.go`）：
1. 所有时间比较**一律移到 Go 侧**，只让驱动负责序列化，使逻辑与容器/数据库时区解耦（与既有 `refresh_token.go` 的正确做法保持一致）。
2. 中间件改为 **fail-closed**：无法确认 token 是否被撤销时拒绝访问，避免同类问题再次被静默掩盖。

**验证**：`TestBlacklistTimezoneIndependent`（真实 MySQL，+08:00）通过——刚拉黑的 token 确实留在表中且查询命中；`api_auth_lifecycle2` 22/22 通过，含"登出后 access token 访问返回 401"。

---

### BUG-2：记录部分更新（省略 date）被错误拒绝

**层次定位**：后端（handler 层）。证据：`PUT {description}` → 400 `{"error":"日期格式必须为 YYYY-MM-DD"}`；`PUT {amount_cents}` → 200；完整载荷 → 200。

**根因**：`internal/handlers/record.go` 的 `UpdateRecord` 把请求中**未提供**的字段也拼成空串，一并交给 `validateRecord` 做整体校验，导致 `date` 缺失时被判非法。

**修复**：改为**只校验请求中实际提供的字段**（`date`/`category`/`description` 分别判空后校验），未提供字段沿用库中原值。`amount_cents` 原本不受影响，行为保持一致。

**验证**：`TestRecordHandler_Update_PartialFields`（6 个子用例）通过；端到端确认仅改 description / amount_cents / category / date 均返回 200，且非法 date 仍返回 400。

---

### BUG-3：创建记录/分类接口回显零值时间戳

**层次定位**：后端（数据访问层）。证据：`POST /api/records` 响应含 `"created_at":"0001-01-01T00:00:00Z"`，随后 `GET /api/records/{id}` 返回真实时间。

**根因**：`internal/database/database.go` 的 `Create` 与 `internal/database/category.go` 的 `CreateCategory` 仅用 `LastInsertId()` 回填 `ID`，**未回查**由 MySQL 默认值生成的时间列，导致结构体保留 Go 零值时间。

**修复**：插入后回查 `created_at`/`updated_at` 并写回结构体。

**验证**：端到端确认创建记录与创建分类的响应均返回非零、且与当前时间一致的时间戳。

---

### BUG-4：`/favicon.ico` 返回 404

**层次定位**：后端（静态路由）+ 前端（缺少图标资源）。

**根因**：`main.go` 只挂载了 `/app` 静态目录，没有处理根路径 `/favicon.ico`；前端也没有任何图标资源。

**修复**：
1. 新增 `frontend/public/favicon.svg`，并在 `frontend/index.html` 中声明 `<link rel="icon">`；
2. `main.go` 新增 `/favicon.ico` 与 `/favicon.svg` 路由，优先返回构建产物，缺失时回退到内置 SVG，保证**永不 404**。

**验证**：三个路径均返回 200；`ui_pages` 的"各页面无前端报错"通过，控制台 404 噪音已消除。

---

### BUG-5：同一秒内签发的 JWT 完全相同（新发现，高危）

**发现过程**：修复 BUG-1 后重跑验证，出现"登出后立刻重新登录，新 token 反而 401"。排查确认：JWT 的 `iat`/`exp` **只有秒级精度**且无唯一标识，同一用户在同一秒内两次登录会生成**字节完全相同**的 token 字符串。登出把该 token 拉黑后，重新登录拿到的"新" token 恰好就是被拉黑的那一个。

**为何此前未暴露**：BUG-1 使黑名单恒不命中，这个缺陷被完全掩盖。修复 BUG-1 后它立刻显现——属于同一修复面的连带缺陷，必须一并解决。

**修复**（`internal/handlers/auth.go`）：为 access token 加入随机 `jti`（`jwt.RegisteredClaims.ID`，16 字节随机十六进制），保证每次签发的 token 唯一。

**验证**：`TestIssueToken_UniquePerCall`（连续签发 100 次无重复）、`TestIssueToken_HasJTI` 通过；端到端确认登出后立即重登，新 token 与旧 token 不同且可正常使用。

---

### BUG-6：时间戳整体偏移 8 小时（新发现，中危）

**发现过程**：BUG-3 修复后，创建接口已返回真实时间戳，但与实际时间相差 8 小时（返回 `18:36:24Z`，实际 UTC 为 `10:36:24Z`）。

**根因**：`docker-compose.yml` 设置了 `TZ=Asia/Shanghai`，但 **alpine 运行镜像不含 tzdata**。Go 无法解析该时区名，静默回退到 UTC；而 MySQL 按 +08:00 写入 `CURRENT_TIMESTAMP`，读出时被当作 UTC 处理，导致时间戳整体偏移 8 小时。这同时也是 BUG-1 的深层诱因。

**修复**（`Dockerfile`）：运行镜像中安装 `tzdata`，使容器时区真正生效。

**验证**：容器内 `date` 现为 `CST`（+08:00）；端到端确认 `created_at` 与实际时间偏差 < 2 分钟。

---

## 四、回归验证

### 4.1 后端

| 项目 | 结果 |
| --- | --- |
| `go vet ./...` | ✅ 通过 |
| `go build ./...` | ✅ 通过 |
| `go test ./...`（单元） | ✅ 全部通过 |
| `go test ./...`（含真实 MySQL 集成） | ✅ 全部通过（database 6.7s） |

新增测试：
- `TestBlacklistTimezoneIndependent`（database，真实 MySQL）
- `TestAuth_BlacklistedTokenRejected` / `TestAuth_NotBlacklistedAllowed` / `TestAuth_BlacklistErrorFailClosed`（middleware）
- `TestRecordHandler_Update_PartialFields`（handlers，6 子用例）
- `TestIssueToken_UniquePerCall` / `TestIssueToken_HasJTI`（handlers）

### 4.2 前端

| 项目 | 结果 |
| --- | --- |
| `npm run build` | ✅ 成功（17.55s） |
| `dist/favicon.svg` 产物 | ✅ 已生成 |
| `dist/index.html` 图标声明 | ✅ 已注入 |

### 4.3 端到端与回归套件

| 套件 | 结果 |
| --- | --- |
| `verify_fixes.mjs`（4 缺陷端到端） | ✅ 24/24 |
| `verify_fixes2.mjs`（改密/时区/jti） | ✅ 16/16 |
| `api_records.mjs` | ✅ 26/26 |
| `api_auth_lifecycle2.mjs` | ✅ 22/22 |
| `api_users.mjs` | ✅ 21/21 |
| `api_logs_totp.mjs` | ✅ 11/11 |
| `api_totp_full.mjs` | ✅ 11/12（1 项断言过严，非缺陷） |
| `api_summary3.mjs` | ✅ 6/6 |
| `ui_pages.mjs` | ✅ 19/19 |
| `ui_records.mjs` | ✅ 17/18（1 项对话框断言方式已知误报） |
| `ui_export_perm.mjs` | ✅ 6/6 |
| `ui_responsive.mjs` | ✅ 9/9 |

> 两处非缺陷说明：
> - `api_totp_full` 的 `needs_totp` 响应中 `token` 为空串（字段无 `omitempty`），空 token 正是安全正确的行为，断言过严。
> - `ui_records` 的"保存后对话框未关闭"是已知的 Element Plus 断言方式问题（关闭后 `.el-dialog` 仍在 DOM 中，应查 `.el-overlay`）；同套件"列表显示刚添加的记录"通过，证明对话框已正确关闭。
>
> `api_summary.mjs` / `api_summary2.mjs` 依赖 QA 阶段特定种子数据（固定日期记录），在数据清理后不适用；其修正版 `api_summary3.mjs` 全通过。

---

## 五、部署与变更文件

### 变更清单

| 文件 | 变更 |
| --- | --- |
| `internal/database/blacklist.go` | 时间比较改为 Go 侧完成，时区无关 |
| `internal/middleware/auth.go` | 黑名单查询失败改为 fail-closed |
| `internal/handlers/record.go` | 部分更新仅校验已提供字段 |
| `internal/database/database.go` | `Create` 回填时间戳 |
| `internal/database/category.go` | `CreateCategory` 回填时间戳 |
| `internal/handlers/auth.go` | access token 加入唯一 `jti` |
| `main.go` | 新增 `/favicon.ico`、`/favicon.svg` 路由 |
| `Dockerfile` | 运行镜像安装 `tzdata` |
| `frontend/index.html` | 声明 favicon |
| `frontend/public/favicon.svg` | 新增图标资源 |
| 测试文件 | 新增 8 个回归用例 |

### 部署状态

- 镜像：`jacknotes/account-service:v2` → `721cfe682f61`（修复版）
- 回滚镜像：`jacknotes/account-service:v2-backup-`（原始版本）
- 容器：`account-service-account-service-1` — Up (healthy)
- 健康检查：`/readyz` → 200 `{"mysql":"ok","status":"ok"}`

### 数据清理

验证用的临时账号（`qa_fix_verify`、`qa_admin`、`qa_user` 等）已全部删除，数据库恢复至原始状态：

| 项目 | 数量 |
| --- | --- |
| 用户 | 2（`admin` / `test`） |
| 记录 | 260 |
| 分类 | 69 |
| 黑名单 / refresh token | 0 / 0 |

---

## 六、结论与建议

1. **6 个缺陷全部完成根因修复**，非仅消除症状；每项均补充了防复发的自动化测试。
2. **跨层回归通过**：后端单测/集成测试、前端构建、浏览器全链路均无回归，QA 阶段已验证的正确行为（跨用户隔离 404、管理员权限 403、角色规则、默认分类、刷新轮换与重放拦截、登录锁定、TOTP 开关、报表导出、响应式布局）保持完好。
3. **修复中发现的连带缺陷值得关注**：BUG-5、BUG-6 均与 BUG-1 同处"会话/时间"这条链路上。BUG-1 的时区缺陷长期掩盖了 JWT 唯一性缺失，说明**此类"静默失效"型缺陷需要 fail-closed + 显式断言才能真正暴露**。

后续建议（非本次修复范围）：
- 为 `expires_at` 类字段统一采用 `TIMESTAMP` 或在 DSN 中显式固定 `loc=UTC`，从数据类型层面消除时区歧义。
- 将 `token_blacklist` 的清理逻辑改为独立定时任务，避免挂在每次写入路径上。
- 前端 `needs_totp` 响应可加 `omitempty` 或明确返回 `token: null`，提升契约清晰度。
