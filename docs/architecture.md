# 架构说明

## 模块边界

```text
┌─────────────────────────────┐
│ frontend (React + Vite)     │
└──────────────┬──────────────┘
               │ HTTP / JSON
┌──────────────▼──────────────┐
│ backend/cmd/server          │  配置、依赖装配、生命周期
│  ├─ internal/config         │
│  ├─ internal/httpapi        │  路由、中间件、请求/响应、错误映射
│  ├─ internal/service        │  用例编排、问题生命周期、事务边界
│  ├─ internal/store          │  GORM/MySQL 仓储、迁移、回填
│  └─ internal/domain         │  持久化模型、状态、指纹、版本
└───────┬─────────────────────┘
        │ Go module dependency
┌───────▼─────────────────────┐
│ geopilot-core (独立模块)     │
│  ├─ scan                    │  抓取、SSRF、规则、评分
│  └─ report                  │  HTML / JSON 报告
└─────────────────────────────┘
```

## 依赖规则

| 层 | 允许依赖 | 禁止依赖 |
| --- | --- | --- |
| `core` | Go 标准库、`x/net/html` | Gin、GORM、MySQL、应用配置 |
| `domain` | Go 标准库 | 其他业务层 |
| `store` | 标准库、GORM、`domain` | `core`、HTTP、Gin |
| `service` | 标准库、`core`、`domain`、`store` | Gin、HTTP |
| `httpapi` | 标准库、Gin、`service` | GORM、直接 SQL |
| `cmd/server` | 所有内部包、配置 | 业务实现 |

## 扩展点

### 扫描器

`service.Scanner` 是接口：

```go
type Scanner interface {
    Scan(ctx context.Context, target string) corescan.Result
}
```

默认实现是 `core/scan.Engine`。后续可以替换为：

- 数据库任务队列的 worker
- Playwright/Chromium 渲染器
- 远程扫描服务客户端
- 多地域探测聚合器

### 规则

`core/scan.Rule` / `RuleFunc` 是规则扩展点。默认规则可由 `DefaultRules()` 获取，也可通过 `WithRules` 完全替换。`Engine.Analyze(ctx, page)` 可直接对页面快照重跑规则。

### 存储

目前 `store.Store` 是 MySQL/GORM 实现。后续若支持 PostgreSQL 或对象存储，可先抽象仓储接口，再在 `service` 层注入。

### 私有模块拆分

当前：

```go
require github.com/fengjutian/geopilot-core v0.0.0
replace github.com/fengjutian/geopilot-core => ../core
```

拆分后：

1. 将 `core/` 移到私有仓库 `github.com/fengjutian/geopilot-core`。
2. 在私有仓库打 tag，例如 `v0.1.0`。
3. 删除 `backend/go.mod` 中的 `replace`，改为 `go get github.com/fengjutian/geopilot-core@v0.1.0`。
4. 配置 `GOPRIVATE=github.com/fengjutian/*` 和对应的 Git 凭据。

业务层只依赖 `Scanner` 接口和 `report` 包，不需要修改。

## 事务与一致性

- 扫描记录先以 `running` 创建，完成或失败后更新。
- 扫描结果、问题行、自动重开的状态更新和 `issue_events` 在同一事务中写入。
- 服务重启时将遗留 `running` 标记为失败。
- 手工问题状态更新会同步同一指纹的历史问题，并写入事件。
- 旧扫描的证据不会被新扫描覆盖；状态属于当前问题元数据。

## API 兼容

`internal/httpapi` 保持现有 `/api` 路径和响应结构。重构只改变代码边界，不改变前端契约。新增能力时优先：

1. 在 `domain` 增加模型/字段；
2. 在 `store` 增加仓储方法；
3. 在 `service` 增加用例；
4. 在 `httpapi` 增加薄处理器；
5. 更新 `API.md` 与前端类型。
