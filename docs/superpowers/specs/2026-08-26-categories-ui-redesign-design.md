# 记账应用增强设计（分类管理 · 默认当月 · 搜索修复 · UI 全面升级）

- 日期：2026-08-26
- 状态：已与用户逐节确认
- 范围：后端（分类 API、bug 修复、迁移）+ 前端（全站 Element Plus + ECharts 重构、响应式/手机端）

## 背景与目标

现有记账应用（Go + Gin + MySQL + Vue3 + Vite）存在四类需求：

1. 记一笔时分类需手动输入，缺少可维护的分类列表
2. 记账列表未默认限定当月，全量查询不实用
3. 关键字搜索触发 500（已定位 SQL 转义 bug）
4. 界面为原生手写样式，需要更现代的视觉与手机端可用性

用户决策记录：

| 决策点 | 结论 |
|--------|------|
| 视觉风格 | 方案 C「深色高级感」：深色底 + 金色渐变点缀 |
| 分类归属 | 每用户独立维护（与记录按用户隔离一致） |
| 分类类型 | 区分「支出 / 收入」两类 |
| 分类实现 | 方案一：categories 表 + 后端 CRUD API |
| UI 范围 | 全站重构（登录/布局/记账/汇总/报表/用户管理/日志） |
| 技术栈 | Vue3 + Element Plus + Vite + ECharts（按需引入） |
| 移动端 | 响应式支持手机使用（<768px），无需单独 App |
| 部署 | Linux 服务器（Go 托管 dist 静态文件，现有 compose 流程不变） |

## 1. 分类数据模型与 API

### 1.1 表结构（migration 004）

```sql
CREATE TABLE IF NOT EXISTS categories (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    name VARCHAR(64) NOT NULL,
    type VARCHAR(16) NOT NULL DEFAULT 'expense',  -- expense | income
    sort_order INT NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_cat (user_id, name, type),
    CONSTRAINT fk_categories_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

要点：

- 同一用户同一 type 下名称唯一（不同 type 可同名）
- 随用户删除级联删除
- `records.category` 保持纯文本，不做外键关联——删除分类不影响历史记录

### 1.2 API（挂在现有 auth 中间件组下）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/categories | 当前用户全部分类，按 type、sort_order、id 排序 |
| POST | /api/categories | `{name, type}`；校验 name 1~64 字符、type ∈ {expense, income}、同类型下唯一；唯一冲突返回 409 |
| DELETE | /api/categories/:id | 仅能删除自己的分类；成功返回 200 |

### 1.3 预置分类

注册成功时（同事务）插入默认集合：

- 支出：餐饮、交通、购物、居住、娱乐、医疗
- 收入：工资、理财、其他收入

存量用户（已注册但无分类）首次 GET /api/categories 时按需补插同一默认集合（幂等：INSERT IGNORE）。

### 1.4 分类管理页

- 侧边栏新增「分类」菜单（所有登录用户可见，位于「报表」之后）
- 支出/收入两个 tab（el-tabs）+ el-table 列表 + 新增（el-dialog）/ 删除（确认框）
- 删除有引用提示：仅提示「历史记录中的该分类文字不受影响」，不做引用计数（YAGNI）

## 2. 记账页改造

### 2.1 布局（桌面端）

- 顶部「本月结余」横幅：金色渐变数字，展示结余/收入/支出（数据来自当月汇总接口），右侧「＋ 记一笔」金色按钮
- 筛选栏：快捷切换胶囊（本月[默认]/上月/全部）+ el-date-picker 日期范围 + 关键字搜索框
- 记录表格：el-table，分类列用 el-tag（支出金色系、收入绿色系）

### 2.2 默认当月

- 进入页面时 `start_date=当月1日, end_date=当月末`，快捷切换「本月」呈选中态
- 快捷切换点击行为：本月/上月设置对应日期范围；「全部」清空日期范围
- 手动改日期范围后，快捷切换取消选中（自定义范围态）

### 2.3 记一笔弹窗

- 支出/收入切换（决定金额正负与分类下拉过滤）
- 分类：el-select 可搜索，选项来自 GET /api/categories（按当前收支类型过滤）
- 金额输入为正数，保存时支出取负（与现有 amount_cents 语义一致）
- 日期默认当天

## 3. 关键字搜索 500 修复

### 3.1 根因（已在真实 MySQL 5.7 复现）

`internal/database/database.go` List() 中 LIKE 子句写作：

```go
where += " AND (description LIKE ? ESCAPE '\\' OR category LIKE ? ESCAPE '\\')"
```

Go 双引号字符串 `'\\'` 发送至 MySQL 的实际文本为 `ESCAPE '\'`——单个反斜杠转义了收尾引号，语句残缺，MySQL 返回 Error 1064（语法错误），handler 兜底为 500。

### 3.2 修复

将 LIKE 子句的双引号字符串改为反引号原始字符串，使 MySQL 收到 `ESCAPE '\\'`（MySQL 解析为转义符 `\`）：

```go
// 修复前（双引号：\\ 转义为单个 \，发送文本为 ESCAPE '\'，引发 1064）：
// where += " AND (description LIKE ? ESCAPE '\\' OR category LIKE ? ESCAPE '\\')"
// 修复后（原始字符串：原样发送 ESCAPE '\\'）：
where += ` AND (description LIKE ? ESCAPE '\\' OR category LIKE ? ESCAPE '\\')`
```

实施时以集成测试（MYSQL_TEST_DSN + TestListRecords_FilterAndSort keyword 用例）通过为准，并补充 `%`、`_`、`\` 边界字符用例（escapeLike 已有单测，新增 List 层集成断言）。

## 4. 全站 UI 重构（Element Plus + ECharts + 风格 C）

### 4.1 依赖与引入

- 新增：element-plus、echarts、@element-plus/icons-vue、unplugin-auto-import、unplugin-vue-components（按需自动引入，控制体积）
- vite.config.js 配置 AutoImport/Components 插件

### 4.2 主题（风格 C 深色高级感）

覆盖 Element Plus CSS 变量（暗色为默认基底）：

| 令牌 | 值 |
|------|----|
| 主色（金） | #f5c451 |
| 主色渐变 | linear-gradient(135deg,#f5c451,#e8930c) |
| 页面背景 | #0c0c10 |
| 卡片背景 | #121218 |
| 边框 | #2e2e3c |
| 主文本 | #e8e8ee |
| 次文本 | #8b8b98 |
| 收入 | #3fd98a |
| 支出 | #ff7b72 |

保留现有浅/深主题切换能力：深色为默认，浅色作为辅助主题（同套变量做 light 覆盖）。

### 4.3 组件替换映射

Modal→el-dialog、Pagination→el-pagination、原生表格→el-table、原生表单→el-form/el-select/el-date-picker/el-input、tabs→el-tabs、确认框→ElMessageBox。

### 4.4 各页面

| 页面 | 改造 |
|------|------|
| 登录 | el-card 居中 + 品牌渐变标题 |
| 主布局 | 桌面侧边栏（金色高亮当前项）；<768px 汉堡抽屉（el-drawer） |
| 记账 | 见第 2 节 |
| 分类 | 见 1.4 |
| 汇总 | 汇总卡片保留；每日模式附迷你趋势图（ECharts line） |
| 报表 | ECharts 主战场：按日收支折线图 + 分类占比环形图（深色主题配置）；表格保留；PDF/图片导出沿用现有逻辑 |
| 用户管理 | el-table + el-pagination + el-tag 角色徽标 |
| 操作日志 | el-table + el-pagination |

### 4.5 响应式（手机支持）

- 断点：≥1024px 完整侧边栏；768~1023px 窄侧边栏；<768px 抽屉导航
- <768px：记录表格降级为卡片列表（分类图标+描述+金额）；筛选变横向滚动胶囊；记一笔弹窗全屏（底部弹出）；触控目标 ≥44px
- ECharts 监听容器 resize
- index.html 确保 viewport meta

### 4.6 部署

- Go 单二进制托管 dist，Linux 服务器现有 docker compose / 直接运行流程不变
- 手机浏览器访问同一地址；若经反代，沿用 TRUSTED_PROXIES 说明

## 5. 错误处理

- 分类 API 校验失败返回 400 + 中文消息；重复名称返回 409「分类已存在」；删除他人分类返回 404
- 前端所有请求错误统一 ElMessage 提示（替换 alert）
- 搜索修复后 List 层错误不再出现 1064；其余数据库错误仍走现有 500 兜底

## 6. 测试

- 后端单元测试：categories handler（内存 fake，参照现有 handler_test.go 模式）
- 后端集成测试：keyword 搜索（含 %/_/\ 边界）、分类 CRUD、注册预置分类
- 前端：vite build 通过 + 浏览器手测（桌面 + 手机视口模拟）

## 7. 非目标（本次不做）

- 分类图标/颜色自定义
- 记录表与分类表外键关联
- 移动端原生 App / PWA 离线
- 报表导出逻辑改动
