# geopilot-core

GeoPilot 的可复用核心模块，规划为后续独立私有 Go 模块：

```text
github.com/fengjutian/geopilot-core
```

模块不依赖 Gin、GORM、MySQL 或应用配置，可被 API、CLI、扫描 worker 以及后续独立报告服务复用。

## 包结构

```text
scan/     URL 安全校验、连接级 SSRF 防护、抓取、规则引擎、评分
report/   与存储无关的 HTML / JSON 报告渲染
```

## 作为本地依赖使用

仓库根目录的 `go.work` 同时包含 `backend` 与 `core`。`backend/go.mod` 通过本地 replace 引用：

```go
require github.com/fengjutian/geopilot-core v0.0.0
replace github.com/fengjutian/geopilot-core => ../core
```

拆分为私有仓库后，删除 `replace`，改为私有模块版本：

```bash
go get github.com/fengjutian/geopilot-core@v0.1.0
```

并通过 `GOPRIVATE=github.com/fengjutian/*` 配置私有模块访问。

## 扫描引擎

```go
engine := scan.New()
result := engine.Scan(ctx, "https://example.com")

fmt.Println(result.Status, result.Score)
for _, finding := range result.Findings {
    fmt.Println(finding.Severity, finding.RuleCode, finding.Title)
}
```

### 扩展规则

`Rule` 是唯一扩展点，默认规则由 `scan.DefaultRules()` 提供：

```go
custom := scan.RuleFunc{
    CodeName: "CUSTOM",
    Fn: func(ctx context.Context, page *scan.Page) []scan.Finding {
        return []scan.Finding{scan.NewFinding(
            scan.SeverityLow,
            "CUSTOM_CHECK",
            "自定义检查",
            "证据",
            "建议",
        )}
    },
}

engine := scan.New(scan.WithRules(append(scan.DefaultRules(), custom)...))
```

也可以使用 `engine.Analyze(ctx, page)` 对已存储的页面快照执行规则，避免重复抓取。

### 安全边界

- 仅允许 HTTP/HTTPS 与 80/443 端口
- 拒绝 localhost、私网、链路本地、保留地址与云元数据地址
- DNS 解析结果与实际拨号绑定，防止 DNS rebinding
- 每次重定向重新校验，限制重定向次数
- 超时、响应体上限，忽略环境代理
- 不执行 JavaScript

## 报告

`report` 接收中立的 DTO，不依赖数据库模型：

```go
html := report.RenderHTML(site, scan, findings)
payload, err := report.MarshalJSON(site, scan, findings)
```

动态内容都会进行 HTML 转义。

## 测试

```bash
cd core
go test ./...
```
