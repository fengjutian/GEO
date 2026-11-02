# GeoPilot

GeoPilot 是一个轻量级 GEO（生成式引擎优化）网站诊断工具的 MVP。

## 技术栈
- 后端：Go + Gin + GORM
- 前端：React + TypeScript + Vite
- 数据库：MySQL 8
- UI：Tailwind CSS + shadcn/ui 风格基础组件 + Lucide React
- 架构：单体应用；不引入 Redis、消息队列或微服务

## 当前包含
- 网站项目 CRUD（创建、列表、详情、删除）
- 启动网站扫描并保存扫描记录
- 诊断规则：HTTP 状态、Title、Meta Description、Canonical、robots.txt、sitemap.xml、JSON-LD Schema、标题结构、可读正文
- 扫描问题持久化，按严重级别呈现
- 历史扫描记录与 HTML 报告页面
- SSRF 基础防护：仅 HTTP/HTTPS、阻止本机/私网/链路本地/保留地址、重定向后重新校验、请求超时与响应体限制
- Docker Compose 启动 MySQL

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
go mod tidy
go run .
```
默认 API 地址：`http://localhost:8080`。

### 3. 启动前端
需要 Node.js 20+。
```bash
cd frontend
npm install
npm run dev
```
默认前端地址：`http://localhost:5173`。

## 注意
- 这是可继续迭代的 MVP 工程骨架，不代表已在你的服务器或目标网站环境中完成实机验证。
- 对外部署前必须补充登录认证、项目级访问控制、请求限流、审计日志与更严格的网络出口控制。
- 网站扫描会访问用户填写的 URL。即使已包含基础 SSRF 防护，也建议在生产环境通过网络层策略阻断内网与云元数据地址。
- GEO 诊断是技术与内容可读性检查，不保证网站一定被任何 AI 搜索平台收录或引用。
