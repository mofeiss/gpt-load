# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

GPT-Load 是一个高性能、企业级的 AI API 透明代理服务，专为需要集成多个 AI 服务的企业和开发者设计。该项目采用 Go 后端 + Vue 3 前端的架构，支持 OpenAI、Google Gemini、Anthropic Claude 等多种 AI 服务。

## 开发命令

### 后端开发 (Go)
```bash
# 构建前端并运行服务器
make run

# 开发模式运行（带竞态检测）
make dev

# 直接运行后端（需要先构建前端）
go run ./main.go
```

### 前端开发 (Vue 3)
```bash
cd web

# 安装依赖
npm install

# 开发模式
npm run dev

# 构建生产版本
npm run build

# 代码检查
npm run lint
npm run lint:check

# 类型检查
npm run type-check

# 格式化
npm run format

# 全面检查
npm run check-all
```

### Docker 开发
```bash
# 使用 Docker Compose 启动
docker compose up -d

# 查看日志
docker compose logs -f

# 重启服务
docker compose down && docker compose up -d
```

## 架构概览

### 后端架构 (Go)
- **依赖注入**: 使用 `go.uber.org/dig` 进行依赖注入管理
- **分层架构**:
  - `internal/app/`: 应用程序生命周期管理
  - `internal/handler/`: HTTP 请求处理器
  - `internal/services/`: 业务逻辑服务层
  - `internal/channel/`: AI 服务渠道适配器
  - `internal/keypool/`: API 密钥池管理
  - `internal/config/`: 配置管理
  - `internal/store/`: 存储层（Redis/内存）
  - `internal/proxy/`: 代理服务器
  - `internal/middleware/`: 中间件

### 核心组件
- **代理服务器** (`internal/proxy/server.go`): 处理 AI API 请求转发
- **密钥池** (`internal/keypool/`): 智能密钥管理和负载均衡
- **渠道适配器** (`internal/channel/`): 支持 OpenAI、Gemini、Anthropic 格式
- **配置管理** (`internal/config/`): 支持环境变量和动态配置热重载

### 前端架构 (Vue 3)
- **框架**: Vue 3 + TypeScript + Vite
- **UI 组件**: Naive UI
- **状态管理**: Pinia (通过 `@vueuse/core`)
- **路由**: Vue Router 4
- **构建工具**: Vite

### 路由结构
- `/api/*`: 管理 API (需要认证)
- `/proxy/{group_name}/*`: AI API 代理端点
- `/health`: 健康检查
- 其他: 前端静态资源

## 配置系统

### 环境变量配置
项目使用 `.env` 文件配置基础设置：
- `AUTH_KEY`: 管理界面认证密钥（必需）
- `DATABASE_DSN`: 数据库连接字符串（默认 SQLite）
- `REDIS_DSN`: Redis 连接字符串（可选，默认内存存储）
- `PORT`: 服务端口（默认 3001）

### 动态配置
系统支持运行时配置热重载：
- 系统设置：全局行为配置
- 组配置：特定组的覆盖配置
- 配置优先级：组配置 > 系统设置 > 环境配置

### 密钥配置
- **最大重试次数**: 单个请求使用不同 Key 的最大重试次数，0为不重试
- **重试间隔**: 请求错误后重试的间隔时间（毫秒），默认 100ms
- **黑名单阈值**: Key 连续失败次数达到该值后进入黑名单
- **密钥验证间隔**: 后台验证密钥的间隔时间（分钟）

## 数据库模式

### 核心模型
- `SystemSetting`: 系统设置
- `Group`: API 密钥组
- `APIKey`: 具体的 API 密钥
- `RequestLog`: 请求日志
- `GroupHourlyStat`: 组小时统计

### 数据库支持
- SQLite (默认)
- MySQL
- PostgreSQL

## 开发注意事项

### 后端开发
- 使用依赖注入模式，通过 `dig` 容器管理服务
- 遵循现有的错误处理模式，使用 `internal/errors/` 包
- 新增功能需要考虑集群部署的兼容性（Master/Slave 架构）
- 配置项需要支持热重载

### 前端开发
- 使用 TypeScript 严格模式
- 遵循现有的组件命名规范
- API 调用使用 `web/src/api/` 中的封装函数
- 状态管理使用 `@vueuse/core` 的响应式工具

### 代码质量
- 后端：Go 1.23+，遵循 Go 标准代码风格
- 前端：ESLint + Prettier + TypeScript 严格检查
- 提交前运行 `make run` 确保完整构建

### 测试
- 前端检查命令：`cd web && npm run check-all`
- 后端暂时没有单元测试，但需要确保功能完整性

## 部署相关

### 生产部署
- 使用 Docker 镜像：`ghcr.io/tbphp/gpt-load:latest`
- 支持 Docker Compose 一键部署
- 支持集群部署（需要共享数据库和 Redis）

### 环境变量安全
- 生产环境必须修改 `AUTH_KEY` 为强密码
- 不要使用默认的 `sk-123456` 密钥
- 建议使用 `sk-proj-[32位随机字符串]` 格式