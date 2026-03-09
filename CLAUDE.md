# 客跟宝 · CLAUDE.md

> 本文件是给 Claude Code 的项目说明书。每次开始新任务前请先读完本文件。

---

## 产品背景

**客跟宝** 是一款面向传统行业个人销售（建材、装修、保险、医疗器械等）的客户跟进提醒工具。

核心价值主张：
- 销售每天早上收到 AI 生成的「今日必联系 3 人 + 开场白建议」
- 用最低的录入成本（30 秒语音/文字）管理客户跟进状态
- 不是 CRM（太重），不是备忘录（太笨），是「私人客户秘书」

目标用户：传统行业个人销售，微信客户 50 个以上，无企业 CRM，单兵作战为主。

当前阶段：**MVP 需求验证阶段**，核心目标是验证付费意愿，而非做完整产品。

---

## 技术栈

### 后端
- 语言：**Go 1.22+**
- Web 框架：**Gin**
- 数据库：**MySQL**
- ORM：**GORM**
- AI 集成：Anthropic Claude API（`claude-sonnet-4-20250514`）
- 认证：JWT（`golang-jwt/jwt/v5`）
- 配置：环境变量 + `.env` 文件（`godotenv`）

### 前端
- 框架：**React 18 + TypeScript**
- 构建工具：**Vite**
- 样式：**Tailwind CSS**
- HTTP 客户端：**axios**
- 状态管理：**Zustand**（轻量，MVP 阶段够用）
- 路由：**React Router v6**
- 适配：移动端优先，微信内置浏览器 H5

### 部署（MVP 阶段）
- 后端：单个 Go 二进制文件，可部署到任何 Linux VPS
- 前端：静态文件，build 后由 Go 后端直接 serve
- 数据库：MySQL 数据库，部署在独立服务器上

---

## 项目结构

```
kegenbao/
├── CLAUDE.md                  # 本文件
├── .env.example               # 环境变量模板
├── .gitignore
├── go.mod
├── go.sum
│
├── cmd/
│   └── server/
│       └── main.go            # 入口，启动 HTTP 服务
│
├── internal/
│   ├── config/
│   │   └── config.go          # 读取环境变量，统一配置结构体
│   ├── database/
│   │   └── db.go              # GORM 初始化，AutoMigrate
│   ├── middleware/
│   │   ├── auth.go            # JWT 鉴权中间件
│   │   └── cors.go            # CORS 配置
│   ├── models/
│   │   ├── user.go            # User 模型
│   │   ├── customer.go        # Customer 模型
│   │   └── record.go          # FollowUpRecord 模型
│   ├── handlers/
│   │   ├── auth.go            # 注册、登录
│   │   ├── customer.go        # 客户 CRUD
│   │   ├── record.go          # 跟进记录 CRUD
│   │   └── ai.go              # AI 分析、今日简报
│   ├── services/
│   │   ├── auth.go            # 用户认证业务逻辑
│   │   ├── customer.go        # 客户业务逻辑
│   │   └── ai.go              # 调用 Anthropic API 的逻辑
│   └── router/
│       └── router.go          # 路由注册，API 分组
│
├── frontend/
│   ├── index.html
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   ├── tsconfig.json
│   ├── package.json
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── api/               # axios 请求封装
│       │   ├── client.ts      # axios 实例，token 拦截器
│       │   ├── auth.ts
│       │   ├── customers.ts
│       │   └── ai.ts
│       ├── store/             # Zustand store
│       │   ├── authStore.ts
│       │   └── customerStore.ts
│       ├── pages/
│       │   ├── Login.tsx
│       │   ├── Register.tsx
│       │   ├── CustomerList.tsx
│       │   ├── CustomerDetail.tsx
│       │   ├── AddCustomer.tsx
│       │   └── Briefing.tsx
│       ├── components/
│       │   ├── CustomerCard.tsx
│       │   ├── TempBadge.tsx
│       │   ├── RecordItem.tsx
│       │   ├── AiBox.tsx
│       │   └── Toast.tsx
│       └── types/
│           └── index.ts       # TypeScript 类型定义
│
└── data/                      # 运行时数据目录（.gitignore）
    └── data.db                # SQLite 数据库文件
```

---

## 数据模型

### User
```go
type User struct {
    gorm.Model
    Phone        string `gorm:"uniqueIndex;not null"` // 手机号，唯一，用于登录
    PasswordHash string `gorm:"not null"`
    Nickname     string
}
```

### Customer
```go
type Customer struct {
    gorm.Model
    UserID       uint   `gorm:"index;not null"`
    Name         string `gorm:"not null"`
    Industry     string
    Phone        string
    Temp         string `gorm:"default:'温'"` // 热 | 温 | 冷
    LastContact  *time.Time
    Notes        string // 简短备注，冗余字段，等于最新 Record 的 note，避免 N+1 查询
}
```

### FollowUpRecord
```go
type FollowUpRecord struct {
    gorm.Model
    CustomerID uint      `gorm:"index;not null"`
    UserID     uint      `gorm:"index;not null"`
    Note       string    `gorm:"not null"`
    ContactedAt time.Time
}
```

---

## API 设计

所有接口前缀：`/api/v1`

认证方式：`Authorization: Bearer <jwt_token>`，除登录/注册外全部需要。

### 认证
```
POST /api/v1/auth/register    # 注册（phone, password, nickname）
POST /api/v1/auth/login       # 登录（phone, password）→ 返回 token
GET  /api/v1/auth/me          # 获取当前用户信息
```

### 客户管理
```
GET    /api/v1/customers            # 获取客户列表（支持 ?temp=热&search=xx&sort=urgent）
POST   /api/v1/customers            # 创建客户
GET    /api/v1/customers/:id        # 获取单个客户（含跟进记录）
PUT    /api/v1/customers/:id        # 更新客户（名称、行业、温度等）
DELETE /api/v1/customers/:id        # 删除客户（软删除）
```

### 跟进记录
```
POST   /api/v1/customers/:id/records   # 添加跟进记录
GET    /api/v1/customers/:id/records   # 获取跟进记录列表
DELETE /api/v1/records/:id             # 删除单条记录
```

### AI 功能
```
POST /api/v1/ai/briefing        # 今日简报（分析全部客户，返回 top3）
POST /api/v1/ai/suggest/:id     # 单客户 AI 分析（意向 + 开场白）
```

### 响应格式（统一）
```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```
错误时 `code` 非 0，`message` 为可读错误信息，`data` 为 null。

---

## 核心业务逻辑

### 今日简报生成（`/api/v1/ai/briefing`）
1. 查询当前用户所有客户，包含最新一条跟进记录
2. 计算每个客户距今天数（`days_since_contact`）
3. 构造 prompt，调用 Claude API
4. 返回 top3 客户 + 每人的选择理由 + 开场白建议
5. 结果**不缓存**（MVP 阶段，每次实时生成）

### AI Prompt 规范
- 系统提示：明确角色为「专业销售顾问，帮助传统行业个人销售提升跟进效率」
- 输出格式：要求返回 JSON，结构固定，便于前端解析
- 温度参数：`max_tokens: 1000`，无需 streaming（MVP 阶段）
- 模型：多种模型可选，qwen/kimi/minimax/claude等，默认使用minimax

### 意向温度规则
- **热**：最近 3 天内有联系，客户主动表达意向
- **温**：3-14 天内有联系，态度中性
- **冷**：超过 14 天未联系，或客户明确表示暂不考虑
- 温度由用户手动设置，AI 分析时会参考但不自动修改

---

## 环境变量

```bash
# .env.example

# 服务配置
PORT=8080
ENV=development              # development | production

# 数据库
DB_PATH=./data/data.db

# JWT
JWT_SECRET=your-secret-key-here
JWT_EXPIRE_HOURS=720         # 30 天

# Anthropic
ANTHROPIC_API_KEY=sk-ant-xxx
```

---

## 开发规范

### Go 后端
- 错误处理：所有 handler 错误统一用 `c.JSON` 返回，不直接 panic
- 日志：使用标准库 `log/slog`，结构化输出
- 数据库操作：全部在 service 层，handler 只负责解析请求和返回响应
- 用户隔离：所有数据库查询必须带 `user_id = ?` 条件，严禁跨用户访问数据
- 密码：使用 `bcrypt` hash 存储，成本因子 12

### React 前端
- 组件：函数式组件 + hooks，不用 class component
- 类型：所有 props 和 API response 必须有 TypeScript 类型定义
- API 调用：统一放在 `src/api/` 目录，组件内不直接使用 axios
- 错误处理：API 错误统一在 axios 拦截器处理，401 自动跳转登录页
- 移动端适配：所有页面必须在 375px 宽度下正常显示

### 代码风格
- Go：`gofmt` 格式化，`golangci-lint` 检查
- TypeScript：ESLint + Prettier
- 提交前运行：`go vet ./...` 和 `npm run lint`

---

## 当前 MVP 功能范围

**✅ 必须做（核心验证功能）**
- 用户注册/登录（手机号+密码）
- 客户 CRUD（增删改查）
- 跟进记录添加
- 意向温度设置（热/温/冷）
- 今日简报（AI 生成今日 top3 + 开场白）
- 单客户 AI 分析

**❌ 本阶段不做（避免过度开发）**
- 微信登录/授权（先用手机号登录）
- 推送通知（先靠用户主动打开）
- 数据导入导出
- 团队/多人协作
- 付费/订阅系统
- 短信验证码

---

## 常用命令

```bash
# 后端
go mod tidy                    # 整理依赖
go run cmd/server/main.go      # 启动开发服务器
go build -o bin/server cmd/server/main.go  # 构建

# 前端
cd frontend
npm install
npm run dev                    # 开发模式（代理到 :8080）
npm run build                  # 构建到 frontend/dist/
npm run lint                   # ESLint 检查

# 一键启动（开发）
# 后端自动 serve frontend/dist/ 下的静态文件
# 前端 dev 模式时通过 vite proxy 转发 /api 到后端
```

---

## 重要提示

1. **数据安全第一**：用户数据严格隔离，任何查询都要带 `user_id`，这是 MVP 阶段最重要的安全要求
2. **AI Key 安全**：`ANTHROPIC_API_KEY` 只放在服务端，绝不暴露到前端
3. **SQLite 并发**：开启 WAL 模式（`PRAGMA journal_mode=WAL`），避免写冲突
4. **MVP 心态**：功能够用就好，不要过度设计。有疑问时选简单方案
5. **前端构建产物**：`frontend/dist/` 由 Go 后端在 `production` 模式下 serve，开发时用 Vite 代理