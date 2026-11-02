# GeoPilot

GeoPilot 是一个轻量级 GEO（生成式引擎优化）网站诊断工具，帮助开发者和企业检查网站是否便于搜索引擎与 AI 系统抓取、理解，并跟踪整改状态。

> **当前阶段：优先验证功能，暂不开发登录认证。** 版本只允许在本地或受控内网运行，不得直接暴露公网。开发要求见 [开发规范](docs/development-guidelines.md)。

## 项目文档

- [产品方案与路线图](docs/product-overview.md)
- [产品需求规格说明书 V1.0](docs/product-requirements-v1.0.md)
- [功能实现与技术设计文档 V1.0](docs/technical-design-v1.0.md)
- [数据库设计与 REST API 接口规范 V1.0](docs/database-and-rest-api-spec-v1.0.md)
- [数据库迁移 SQL 与 OpenAPI 3.1 完整接口定义 V1.0](docs/database-migration-and-openapi-v1.0.md)
- [开发进度与验证记录](docs/development-status.md)
- [功能验证报告](docs/functional-validation-report.md)
- [开发规范](docs/development-guidelines.md)
- [架构说明](docs/architecture.md)
- [核心模块说明](core/README.md)
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
- 架构：分层单体应用 + 独立 `geopilot-core` 模块；不引入 Redis、消息队列或微服务

核心扫描与报告逻辑位于独立 Go 模块 [`core`](core/README.md)，不依赖 Gin/GORM，后续可拆分为私有模块 `github.com/fengjutian/geopilot-core`。

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
go run ./cmd/server
```

仓库根目录的 `go.work` 同时组织 `backend` 与 `core` 两个模块；`backend/go.mod` 当前通过 `replace ... => ../core` 使用本地核心模块。拆分为私有仓库后，移除 `replace` 并改用私有模块版本即可。

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

仓库提供一键验证脚本（包含格式、vet、测试和前端构建）：

```bash
# Windows
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/verify.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/verify.ps1 -Integration

# macOS / Linux / Git Bash
./scripts/verify.sh
```

在仓库根目录也可一次运行两个模块：

```bash
go test ./core/... ./backend/...
```

核心模块测试不依赖数据库：

```bash
cd core
go test ./...
```

后端测试：

```bash
cd backend
go test ./...
```

MySQL API 集成测试需要先准备一个**专用空测试库**，再通过 `TEST_DB_DSN` 开启（测试会清空该库的业务表）：

```bash
# PowerShell
$env:TEST_DB_DSN="geopilot:change_me_geopilot@tcp(127.0.0.1:3306)/geopilot_test?charset=utf8mb4&parseTime=True&loc=Local"
go test -run TestAPI -v ./internal/httpapi
```

```bash
# bash
TEST_DB_DSN='geopilot:change_me_geopilot@tcp(127.0.0.1:3306)/geopilot_test?charset=utf8mb4&parseTime=True&loc=Local' go test -run TestAPI -v ./internal/httpapi
```

真实 API 冒烟验证（需要已启动 API，且数据库为可清理的测试库）：

```bash
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke-api.ps1 `
  -ApiBase http://127.0.0.1:8080/api `
  -Target https://example.org `
  -FailureTarget https://expired.badssl.com
```

前端类型检查与构建：

```bash
cd frontend
npm run build
```

## 目录结构

```text
core/                        独立核心模块（未来的私有包）
  scan/
    types.go                 页面/规则/结果/评分类型与扩展点
    target.go                URL 规范化和连接级 SSRF 防护
    engine.go                抓取、资源加载、规则执行与评分
    rules.go                 内置诊断规则
  report/                    HTML / JSON 报告渲染
backend/
  cmd/server/                应用入口与优雅退出
  internal/config/           环境变量与 .env
  internal/domain/           Site / Scan / Finding / IssueEvent
  internal/store/            MySQL 持久化、迁移、回填
  internal/service/          扫描编排、问题生命周期、工作台、报告
  internal/httpapi/          路由、中间件、请求/响应
  internal/httpapi/api_test.go  MySQL 端到端测试
frontend/
  src/App.tsx                应用外壳与工作台/详情切换
  src/Dashboard.tsx          工作台
  src/SitePage.tsx           诊断概览、问题中心、扫描历史
  src/api.ts                 API 客户端与错误处理
  src/components.tsx         共享 UI 组件
docs/
go.work                      后端与 core 模块工作区
```

## 扩展方式

- **新增诊断规则**：在 `core/scan` 实现 `Rule` 或使用 `RuleFunc`，通过 `scan.WithRules` 注册；也可复用 `Engine.Analyze` 对快照重跑规则。
- **替换扫描执行方式**：后端 `service.Scanner` 是接口，可替换为队列消费者、浏览器渲染器或远程扫描服务。
- **新增 API**：在 `internal/service` 增加用例，在 `internal/httpapi` 增加薄处理器，不直接操作数据库。
- **替换存储**：`internal/store` 封装 GORM；后续可抽象接口或增加 PostgreSQL 实现。
- **拆分私有包**：将 `core/` 移至私有仓库，修改模块路径与 `backend` 依赖即可，业务层无需改动。

## 安全与部署边界

- 扫描器会访问用户填写的 URL。即使已包含应用层 SSRF 防护，生产环境仍应通过网络出口策略阻断内网与云元数据地址。
- 当前版本没有登录认证、租户隔离、请求限流与审计日志，**不应直接暴露到公网**。
- 扫描为同步执行，单次只抓取首页，不执行 JavaScript；大页面会被截断到 2 MiB 后分析。
- 规则评分仅用于同一产品内比较，不代表任何 AI 搜索平台的官方评分。
- GEO 诊断不保证网站一定被任何 AI 搜索平台收录或引用。
