# 开发进度与验证记录

更新时间：2026-11-02

## 架构重构（v0.3.0）

### 核心模块独立

- 新增独立 Go 模块 `github.com/fengjutian/geopilot-core`，为后续拆分私有包准备。
- `core/scan`：URL 安全校验、连接级 SSRF 防护、抓取、可插拔规则引擎、评分。
- `core/report`：与存储无关的 HTML / JSON 报告渲染。
- 核心模块不依赖 Gin、GORM、MySQL 或应用配置，可独立发布与复用。
- 新增 `Rule` / `RuleFunc` 扩展点、`scan.WithRules` 与 `Engine.Analyze` 快照复跑能力。
- 仓库新增 `go.work`，`backend` 通过 `replace` 引用本地 `core`，拆分后只需切换私有模块版本。

### 后端分层

```text
cmd/server          应用入口、信号处理、HTTP Server
internal/config     环境变量、.env、CORS
internal/domain     持久化模型、状态、指纹、版本
internal/store      MySQL 连接、迁移、回填、仓储
internal/service    扫描编排、问题生命周期、工作台、报告用例
internal/httpapi    路由、中间件、请求解析与错误映射
```

- HTTP 层不再直接依赖 GORM；业务错误通过服务层统一映射。
- `service.Scanner` 抽象为接口，可替换为队列 worker、浏览器渲染或远程扫描服务。
- 扫描结果持久化、问题状态传播、自动重开事件保持在同一事务。
- API 响应结构与原有前端保持兼容。

## 本轮完成（v0.2.0）

### 后端

- 拆分 `main.go` / `api.go`，补充数据库启动重试、`.env` 读取、优雅退出与请求 ID。
- 新增 `GET /api/dashboard` 工作台聚合接口。
- 网站支持编辑（`PUT /api/sites/:id`）；删除改为事务级联删除扫描、问题与状态事件。
- 扫描记录新增目标 URL、最终 URL、HTTP 状态、失败代码/原因、规则版本、评分算法版本；服务启动时清理遗留 `running` 任务。
- 扫描失败按 DNS、TLS、超时、安全策略分别记录错误代码。
- 问题表新增指纹、扣分值、首次/最近发现时间与状态；新增 `issue_events` 记录状态变化。
- 重新扫描时按指纹继承状态；已修复问题再次出现会自动重新打开并留痕。
- 新增扫描详情接口与跨扫描差异（新增/持续/未再出现）。
- 新增 JSON 报告导出；HTML 报告补齐扫描元数据、严重级别统计、状态与评分说明。
- SSRF 防护升级为连接级校验：解析后的公网 IP 才用于拨号，阻断 DNS rebinding；限制 80/443 端口并忽略环境代理。
- 新增请求体 1 MiB 上限、统一错误响应与 CORS 配置。
- 新增规则/报告单元测试，以及可选的 MySQL API 集成测试。

### 前端

- 重构为 `App` / `Dashboard` / `SitePage` / `components` / `api` 模块。
- 工作台接入真实统计、站点最近扫描、待处理问题与最近扫描活动。
- 新增网站编辑、删除确认。
- 新增诊断概览：评分、严重级别分布、整改进度、优先整改、评分轨迹。
- 问题中心支持级别/状态/关键词筛选，并可直接更新问题状态。
- 扫描历史支持详情弹层、与上次成功扫描的差异、HTML/JSON 报告入口。
- 支持 `#/sites/:id?tab=overview|issues|history` 深链接与移动端布局。
- 删除二进制/增量构建产物跟踪，补充 `.gitignore`。

## 验证记录

| 项目 | 命令 | 结果 |
| --- | --- | --- |
| 核心模块测试 | `cd core && go test ./...` | 通过 |
| 后端格式与构建 | `cd backend && go build ./... && go vet ./...` | 通过 |
| 后端测试 | `cd backend && go test ./...` | 通过（数据库测试自动跳过） |
| MySQL API 集成测试 | `TEST_DB_DSN=... go test -run TestAPI -v ./internal/httpapi` | 通过（MySQL 8.0.46） |
| 前端类型检查与构建 | `cd frontend && npm run build` | 通过 |
| 真实站点扫描 | `POST /api/sites/:id/scans`，目标 `https://example.org` | 成功，7 项问题，评分 42 |
| TLS 失败场景 | 目标 `https://expired.badssl.com` | 状态 `failed`，`TLS_ERROR` 持久化正确 |
| 状态生命周期 | 标记 `resolved` 后再次扫描 | 自动重开并生成事件 |
| 报告转义 | 问题标题含 `<script>` | HTML 报告正确转义 |

说明：真实站点扫描与界面验证使用本机临时 MySQL 实例；Docker 服务在当前环境未启动，未验证 `docker compose` 路径。

## 已知限制

- 无登录认证、资源权限、限流与审计日志，不能直接暴露公网。
- 扫描仍是同步请求，长耗时目标会占用 HTTP 连接；下一步应改为数据库任务队列 + 后台 worker + 轮询。
- 只抓取首页，不执行 JavaScript，不支持多页面/内链发现。
- 规则为启发式检查，没有规则配置、行业权重或基准对比。
- 站点 URL 修改后，问题指纹会随 URL 变化，旧问题不会自动关联。

## 建议下一阶段

1. **安全基线**：单用户登录、会话、接口认证、限流与审计日志。
2. **异步扫描**：`queued → running → completed/failed` 状态机、任务领取、重启恢复与前端轮询。
3. **多页面抓取**：内链发现、页面去重、每页快照与页面级报告。
4. **项目/租户维度**：项目分组、站点归属、行业与品牌信息。
5. **规则可配置化**：规则开关、权重、规则版本迁移说明与回归测试集。
6. **AI 可见度记录**：人工录入平台回答、品牌提及与官网引用，并与整改记录关联。
