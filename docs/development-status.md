# 开发进度与验证记录

更新时间：2026-11-02

## 异步扫描（v0.4.0）

- 新增 `queued`、`cancelled` 扫描状态；`POST /api/sites/:id/scans` 改为返回 `202` 和排队任务。
- 新增 `internal/worker`：数据库队列轮询、并发 2、异常恢复；`queued → running` 使用条件更新领取。
- 入队使用站点行锁 + 活跃任务检查，避免并发请求重复创建扫描。
- 扫描完成后的记录、问题与自动重开事件仍在同一事务写入。
- 服务启动时遗留 `running` 标记失败，`queued` 任务继续由 worker 处理。
- 前端新增 `waitForScan` 轮询、排队/执行状态、自动刷新工作台与扫描历史。
- 新增异步集成测试、真实 API 冒烟验证和排队/完成界面验证。

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
internal/store      MySQL 连接、迁移、回填、仓储、任务领取
internal/service    扫描编排、问题生命周期、工作台、报告用例
internal/worker     队列轮询、并发执行、异常恢复
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
| 一键验证 | `scripts/verify.ps1 -Integration` | 通过 |
| 前端类型检查与构建 | `cd frontend && npm run build` | 通过 |
| 功能验证 | F-01 至 F-11 | 通过，详见 [功能验证报告](functional-validation-report.md) |
| 异步任务冒烟 | `scripts/smoke-api.ps1` | `SMOKE PASSED`：202/queued/轮询/完成/失败 |
| 真实站点扫描 | `POST /api/sites/:id/scans`，目标 `https://example.org` | 成功，7 项问题，评分 42 |
| 失败场景 | 目标 `https://expired.badssl.com` | 状态 `failed`，`FETCH_TIMEOUT` 持久化正确 |
| 状态生命周期 | 标记 `resolved` 后再次扫描 | 自动重开并生成事件 |
| 报告转义 | 问题标题含 `<script>` | HTML 报告正确转义 |

说明：真实站点扫描与界面验证使用本机临时 MySQL 实例；Docker 服务在当前环境未启动，未验证 `docker compose` 路径。

## 当前阶段决策

**暂不开发登录，优先验证功能。** 详见 [开发规范](development-guidelines.md)。

F-01 至 F-11 已于 2026-11-02 完成一轮验证，结果见 [功能验证报告](functional-validation-report.md)，未发现阻塞性缺陷。

当前重点：

1. 验证真实站点的诊断准确性。
2. 验证问题状态、自动重开、扫描对比与报告的数据一致性。
3. 验证前端关键路径与错误状态。
4. 保持 SSRF、输入校验、HTML 转义等安全边界不回退。

## 已知限制

- 无登录认证、资源权限、限流与审计日志，不能直接暴露公网；该能力当前明确暂缓，不作为功能验证的阻塞项。
- 扫描队列为单进程 worker，启动时清理遗留 `running`；多进程/多实例部署前需要任务租约与心跳。
- 只抓取首页，不执行 JavaScript，不支持多页面/内链发现。
- 规则为启发式检查，没有规则配置、行业权重或基准对比。
- 站点 URL 修改后，问题指纹会随 URL 变化，旧问题不会自动关联。

## 建议下一阶段

1. **功能回归**：按开发规范中的 F-01 至 F-11 清单持续回归，记录真实站点、失败站点和状态流转证据。
2. **多页面抓取**：内链发现、页面去重、每页快照与页面级报告。
3. **规则可配置化**：规则开关、权重、规则版本迁移说明与回归测试集。
4. **项目/租户维度**：项目分组、站点归属、行业与品牌信息。
5. **AI 可见度记录**：人工录入平台回答、品牌提及与官网引用，并与整改记录关联。
6. **登录认证**：仅在功能验证通过并进入部署阶段后启动。
