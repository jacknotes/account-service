# 记账微服务

基于 Go + Gin 的前后端分离记账应用，支持按日期和关键字查询，具备完整的增删改查功能。

## 技术栈

- **后端**: Go 1.21, Gin, SQLite（纯 Go 驱动，无需 CGO）
- **前端**: 原生 HTML/CSS/JavaScript

## 功能特性

- ✅ **用户认证**：用户名密码登录，可选 TOTP 双因素认证
- ✅ 添加记账记录（日期、金额、分类、描述）
- ✅ 编辑记录
- ✅ 删除记录
- ✅ 按日期范围查询
- ✅ 按关键字搜索（描述、分类）
- ✅ 分页展示
- ✅ **每日汇总**：按日查看收入、支出、结余及明细
- ✅ **每月汇总**：按月查看并支持按日分项
- ✅ **每年汇总**：按年查看并支持按月分项
- ✅ **报表**：自定义日期范围，按日统计、按分类统计

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 启动服务

```bash
go run main.go
```

默认监听 `http://localhost:8080`，打开浏览器访问即可。

### 3. 首次使用

访问 http://localhost:8080/app/ 会跳转到登录页。**首次使用且无用户时**，可点击「注册」创建账号；首个注册用户自动成为**管理员**，之后注册将关闭。管理员可在「用户管理」中增删改查用户、修改用户密码。

### 4. Docker 部署

```bash
# 构建镜像
docker build -t account-service .

# 运行（数据持久化到宿主机）
docker run -d -p 8081:8081 \
  -v $(pwd)/data:/app/data \
  -e JWT_SECRET=your-secret-key \
  --name account-service \
  account-service

# 或使用 docker-compose
docker-compose up -d
```

访问 http://localhost:8081/app/

### 5. 环境变量（可选）

| 变量 | 说明 | 默认值 |
|------|------|--------|
| PORT | 服务端口 | 8080 |
| DATABASE_PATH | 数据库文件路径 | ./data/accounting.db |
| FRONTEND_DIR | 前端静态文件目录 | ./frontend |
| JWT_SECRET | JWT 签名密钥 | 默认值（生产环境务必修改） |

## API 接口

**认证**
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/auth/register/status | 是否允许注册 |
| POST | /api/auth/register | 注册（仅当无用户时可用） |
| POST | /api/auth/login | 登录（返回 token，启用 TOTP 时需再提交验证码） |
| GET | /api/auth/me | 当前用户（需认证） |
| POST | /api/auth/change-password | 修改密码（需认证） |
| GET | /api/auth/users | 用户列表（管理员） |
| POST | /api/auth/users | 添加用户（管理员） |
| GET | /api/auth/users/:id | 获取用户（管理员） |
| PUT | /api/auth/users/:id | 更新用户（管理员） |
| DELETE | /api/auth/users/:id | 删除用户（管理员） |
| POST | /api/auth/users/:id/change-password | 管理员修改用户密码 |
| GET | /api/auth/operation-logs | 操作日志（管理员，支持 user_id、action 筛选） |
| GET | /api/auth/totp/setup | 获取 TOTP 密钥/二维码（需认证） |
| POST | /api/auth/totp/enable | 启用 TOTP（需认证） |
| POST | /api/auth/totp/disable | 关闭 TOTP（需认证） |

**记账**
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/records | 查询列表（支持 start_date, end_date, keyword, page, page_size） |
| GET | /api/records/:id | 获取单条记录 |
| POST | /api/records | 创建记录 |
| PUT | /api/records/:id | 更新记录 |
| DELETE | /api/records/:id | 删除记录 |

**汇总与报表**
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/summary/daily?date= | 每日汇总 |
| GET | /api/summary/monthly?year=&month= | 每月汇总 |
| GET | /api/summary/yearly?year= | 每年汇总 |
| GET | /api/report?start_date=&end_date= | 报表（按日、按分类） |

### 请求示例

**创建记录**
```json
POST /api/records
{
  "date": "2024-02-06",
  "amount": -25.5,
  "category": "餐饮",
  "description": "午餐"
}
```

**查询（日期 + 关键字）**
```
GET /api/records?start_date=2024-01-01&end_date=2024-12-31&keyword=餐饮&page=1&page_size=20
```

## 项目结构

```
├── main.go              # 入口
├── config/              # 配置
├── internal/
│   ├── database/        # 数据库操作
│   ├── handlers/        # API 处理器
│   └── models/          # 数据模型
├── frontend/            # 前端静态资源
│   ├── index.html
│   ├── style.css
│   └── app.js
└── data/                # SQLite 数据库（自动创建）
```
