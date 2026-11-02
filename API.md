# API 快速说明

Base URL：`http://localhost:8080/api`

## 通用约定

- 请求与响应均为 JSON（报告下载除外）。
- 失败响应统一包含 `code`、`message`，并保留 `error` 字段兼容旧前端：

```json
{
  "code": "VALIDATION_ERROR",
  "message": "网站名称和地址不能为空",
  "error": "网站名称和地址不能为空"
}
```

- 每次响应带 `X-Request-ID`；请求可传入同名请求头用于排查。
- 写接口请求体上限 1 MiB。

## 健康检查

### `GET /api/health`

```json
{ "status": "ok", "database": "ok" }
```

数据库不可用时返回 `503`。

## 工作台

### `GET /api/dashboard`

```json
{
  "stats": {
    "siteCount": 3,
    "scanCount": 12,
    "openIssues": 8,
    "highPriority": 2,
    "averageScore": 72
  },
  "sites": [
    {
      "id": 1,
      "name": "官网",
      "url": "https://example.com",
      "latestScan": { "id": 12, "status": "completed", "score": 78 },
      "openIssues": 3,
      "highPriority": 1
    }
  ],
  "recentScans": [
    { "id": 12, "siteId": 1, "siteName": "官网", "status": "completed", "score": 78 }
  ]
}
```

`openIssues` 与 `highPriority` 基于每个站点最近一次成功扫描，只统计 `open` 和 `in_progress`。

## 网站

### `GET /api/sites`

返回网站数组，按创建时间倒序。

### `POST /api/sites`

```json
{ "name": "官网", "url": "https://example.com", "description": "" }
```

服务端会规范化 URL，并校验协议、端口与目标 IP。成功返回 `201` 和网站对象。

### `GET /api/sites/:id`

返回网站、最近 20 次扫描以及 `latestScan`。每次扫描包含 `issueCount` 与 `highIssueCount`。

### `PUT /api/sites/:id`

请求体同创建接口，用于修改名称、URL 和备注。URL 修改不会删除历史扫描。

### `DELETE /api/sites/:id`

事务删除网站及其扫描、问题和状态事件，成功返回 `204`。

## 扫描

### `POST /api/sites/:id/scans`

创建一个异步扫描任务并立即返回 `202 Accepted`。响应头包含 `Location: /api/scans/{id}`：

```json
{
  "scan": {
    "id": 13,
    "siteId": 1,
    "status": "queued",
    "targetUrl": "https://example.com",
    "finalUrl": "",
    "httpStatus": 0,
    "startedAt": null,
    "finishedAt": null,
    "score": 0,
    "summary": "",
    "ruleVersion": "2026.11.1",
    "scoreVersion": "v1"
  }
}
```

扫描由后台 worker 从队列领取并执行：

```text
queued -> running -> completed
                  -> failed
                  -> cancelled
```

调用方应轮询 `GET /api/scans/:id`，直到 `status` 为 `completed`、`failed` 或 `cancelled`。

- 每个网站同一时间只允许一个 `queued` 或 `running` 任务；冲突返回 `409 SCAN_IN_PROGRESS`。
- 抓取失败仍会生成记录，`status=failed`，`errorCode`/`errorMessage` 说明原因。
- 服务启动时会把遗留的 `running` 扫描标记为失败；遗留 `queued` 任务会由 worker 继续领取。
- 创建接口不再返回问题明细，问题通过扫描详情或 `findings` 接口获取。

### `GET /api/scans/:id`

返回扫描、网站、该次扫描的问题、严重级别分布、状态分布，以及与此前最近一次成功扫描的对比。该接口同时用于轮询扫描进度：

```json
{
  "scan": {},
  "site": {},
  "findings": [],
  "severityCounts": { "critical": 0, "high": 1, "medium": 2, "low": 3 },
  "statusCounts": { "open": 4, "in_progress": 1, "resolved": 1, "ignored": 0 },
  "comparison": {
    "baseScanId": 12,
    "newCount": 1,
    "persistingCount": 4,
    "resolvedCount": 2,
    "new": [],
    "resolved": []
  }
}
```

问题匹配使用 `站点 ID + 规范化页面 URL + 规则编号` 的指纹。

## 问题中心

### `GET /api/sites/:id/findings`

默认返回最近一次成功扫描的问题；没有成功扫描时返回空数组。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `scanId` | 查看指定扫描的问题 |
| `severity` | `critical` / `high` / `medium` / `low` |
| `status` | `open` / `in_progress` / `resolved` / `ignored` |
| `q` | 搜索标题、规则编号或证据 |

问题字段包含 `firstSeenAt`、`lastSeenAt`、`scorePenalty`、`fingerprint` 和当前状态。

### `PATCH /api/issues/:id`

```json
{ "status": "resolved", "note": "已补充 Canonical 并复核" }
```

- 状态：`open`、`in_progress`、`resolved`、`ignored`。
- 同一指纹的历史问题状态会同步更新。
- 状态变化写入 `issue_events`；相同状态重复提交保持幂等。
- 已修复问题在后续扫描中再次出现时会自动重新打开，并记录事件。

## 报告

### `GET /api/scans/:id/report`

返回可直接在浏览器打开的 HTML 报告，包含扫描时间、规则版本、评分、严重级别统计、问题证据与建议。所有外部内容已进行 HTML 转义。

### `GET /api/scans/:id/report.json`

以附件形式下载 JSON 报告：

```json
{
  "formatVersion": "1.0",
  "generatedAt": "2026-11-02T09:39:00+08:00",
  "site": {},
  "scan": {},
  "severityCounts": { "critical": 0, "high": 1, "medium": 2, "low": 3 },
  "findings": [],
  "boundary": "本报告由 GeoPilot 的确定性规则引擎生成……"
}
```

## 规则代码

| 规则 | 级别 | 说明 |
| --- | --- | --- |
| `TARGET_BLOCKED` | 严重 | URL 或目标 IP 不符合安全策略 |
| `URL_INVALID` | 严重 | URL 格式或协议非法 |
| `DNS_RESOLUTION_FAILED` | 严重 | 域名无法解析 |
| `TLS_ERROR` | 严重 | TLS 握手或证书校验失败 |
| `FETCH_TIMEOUT` | 严重 | 请求超时 |
| `HTTP_FETCH_FAILED` | 严重 | 其他首页请求失败 |
| `HTTP_STATUS` | 高 | 首页返回非 2xx |
| `CONTENT_TYPE_UNSUPPORTED` | 高 | 首页不是 HTML/XHTML |
| `BODY_LIMIT` | 高 | 响应体超过 2 MiB，仅分析截断内容 |
| `TITLE_MISSING` | 高 | 缺少非空 title |
| `TITLE_LONG` | 低 | title 超过 65 个字符 |
| `META_DESCRIPTION_MISSING` | 中 | 缺少 description |
| `CANONICAL_MISSING` | 中 | 缺少 rel=canonical |
| `H1_MISSING` | 中 | 缺少 h1 |
| `H1_MULTIPLE` | 低 | 存在多个 h1 |
| `SCHEMA_MISSING` | 中 | 未发现 JSON-LD |
| `THIN_CONTENT` | 中 | 可读正文少于 200 字符 |
| `ROBOTS_UNAVAILABLE` | 低 | robots.txt 不可访问 |
| `SITEMAP_UNAVAILABLE` | 低 | sitemap.xml 不可访问 |

评分从 100 开始，严重/高/中/低分别扣 30/18/10/4，最低 0 分。

## 错误码

| HTTP | code | 说明 |
| --- | --- | --- |
| 400 | `VALIDATION_ERROR` | 参数或状态非法 |
| 404 | `RESOURCE_NOT_FOUND` | 网站/扫描/问题不存在 |
| 409 | `SCAN_IN_PROGRESS` | 同一网站已有排队或执行中的扫描 |
| 500 | `INTERNAL_ERROR` | 数据库或持久化失败 |
| 503 | `SERVICE_UNAVAILABLE` | 数据库不可用（健康检查） |
