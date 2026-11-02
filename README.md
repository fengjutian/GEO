# GeoPilot

GeoPilot 是一个轻量级 GEO（生成式引擎优化）网站诊断工具，帮助开发者和企业检查网站是否便于搜索引擎与 AI 系统抓取、理解，并跟踪整改状态。

## 项目文档

- [产品方案与路线图](docs/product-overview.md)
- [产品需求规格说明书 V1.0](docs/product-requirements-v1.0.md)
- [功能实现与技术设计文档 V1.0](docs/technical-design-v1.0.md)
- [数据库设计与 REST API 接口规范 V1.0](docs/database-and-rest-api-spec-v1.0.md)
- [数据库迁移 SQL 与 OpenAPI 3.1 完整接口定义 V1.0](docs/database-migration-and-openapi-v1.0.md)
- [开发进度与验证记录](docs/development-status.md)
- [API 快速说明](API.md)

## 当前能力

- 网站项目创建、列表、编辑、详情与删除
- 工作台统计：网站数、累计扫描、待处理问题、最近扫描与站点状态
- 同步首页扫描，并持久化成功/失败状态、错误代码与规则版本
- 诊断规则：HTTP 状态、Content-Type、Title、Meta Description、Canonical、robots.txt、sitemap.xml、JSON-LD Schema、H1 结构、可读正文、内容大小
- 问题生命周期：首次/最近发现时间、证据、整改建议、扣分值与状态（待处理/处理中/已修复/已忽略）
- 重新扫描时继承问题状态；已修复问题再次出现会自动重新打开并记录事件
- 扫描历史、与上一次成功扫描的差异对比（新增/持续/未再出现）
- 可独立访问的 HTML 报告与 JSON 数据导出
- SSRF 防护：仅 HTTP/HTTPS、80/443 端口、连接级 DNS/IP 校验、重定向复检、超时与响应体上限
- 后端规则/报告单元测试与可选的 MySQL API 集成测试

## 技术栈

- 后端：Go + Gin + GORM
- 前端：React + TypeScript + Vite
- 数据库：MySQL 8
- UI：Tailwind CSS + Lucide React
- 架构：单体应用；不引入 Redis、消息队列或微服务

## 启动

### 1. 启动数据库

```bash
docker compose up -d mysql
```

### 2. 启动后端

需要 Go 1.22+。

```bash
cd backend
cp .env.example .env
go mod download
go run .
```

后端会自动读取 `backend/.env`，默认 API 地址为 `http://localhost:8080`。健康检查：

```bash
curl http://localhost:8080/api/health
```

### 3. 启动前端

需要 Node.js 20+。

```bash
cd frontend
npm install
npm run dev
```

默认前端地址：`http://localhost:5173`。如后端地址不同，可在 `frontend/.env` 设置：

```bash
VITE_API_BASE=http://localhost:8080/api
```

生产构建：

```bash
cd frontend
npm run build
```

## 测试

后端单元测试不依赖数据库：

```bash
cd backend
go test ./...
```

MySQL API 集成测试需要先准备一个**专用空测试库**，再通过 `TEST_DB_DSN` 开启（测试会清空该库的业务表）：

```bash
# PowerShell
$env:TEST_DB_DSN="geopilot:change_me_geopilot@tcp(127.0.0.1:3306)/geopilot_test?charset=utf8mb4&parseTime=True&loc=Local"
go test -run TestAPI -v ./...
```

```bash
# bash
TEST_DB_DSN='geopilot:change_me_geopilot@tcp(127.0.0.1:3306)/geopilot_test?charset=utf8mb4&parseTime=True&loc=Local' go test -run TestAPI -v ./...
```

前端类型检查与构建：

```bash
cd frontend
npm run build
```

## 目录结构

```text
backend/
  main.go        配置、数据库连接、迁移与优雅退出
  api.go         路由、请求校验、扫描持久化、问题/报告/工作台接口
  models.go      Site / Scan / Finding / IssueEvent
  scanner.go     URL 安全校验、连接级 SSRF 防护、HTML 规则引擎
  report.go      HTML 报告与 JSON 报告结构
  *_test.go      单元测试与可选 MySQL 集成测试
frontend/
  src/App.tsx        应用外壳与工作台/详情切换
  src/Dashboard.tsx  工作台
  src/SitePage.tsx   诊断概览、问题中心、扫描历史
  src/api.ts         API 客户端与错误处理
  src/components.tsx 共享 UI 组件
docs/
```

## 安全与部署边界

- 扫描器会访问用户填写的 URL。即使已包含应用层 SSRF 防护，生产环境仍应通过网络出口策略阻断内网与云元数据地址。
- 当前版本没有登录认证、租户隔离、请求限流与审计日志，**不应直接暴露到公网**。
- 扫描为同步执行，单次只抓取首页，不执行 JavaScript；大页面会被截断到 2 MiB 后分析。
- 规则评分仅用于同一产品内比较，不代表任何 AI 搜索平台的官方评分。
- GEO 诊断不保证网站一定被任何 AI 搜索平台收录或引用。
