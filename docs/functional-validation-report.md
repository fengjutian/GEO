# 功能验证报告

日期：2026-11-02  
阶段：MVP 功能验证阶段（暂不开发登录）  
基线版本：v0.3.0  
规范：[开发规范](development-guidelines.md)

## 1. 验证环境

| 项目 | 版本/说明 |
| --- | --- |
| 操作系统 | Windows |
| Go | 1.27.1 |
| Node.js | 24.14.1 |
| MySQL | 8.0.46（本机临时实例，专用 `geopilot_test` 库） |
| 浏览器 | Chrome Headless 156 |
| 真实站点 | `https://example.org` |
| 失败场景 | `https://expired.badssl.com` |

验证辅助脚本：

```bash
go test -count=1 ./core/... ./backend/...
powershell -File scripts/verify.ps1 -Integration

# API 已启动时执行真实 HTTP 冒烟验证
powershell -File scripts/smoke-api.ps1 `
  -ApiBase http://127.0.0.1:18080/api `
  -Target https://example.org `
  -FailureTarget https://expired.badssl.com
```

冒烟结果：`SMOKE PASSED`，覆盖创建/扫描/问题状态/自动重开/对比/报告/工作台/失败扫描路径。

## 2. 验证结果

| 编号 | 场景 | 结果 | 证据/说明 |
| --- | --- | --- | --- |
| F-01 | 网站 CRUD 与非法目标拒绝 | 通过 | 私网 `http://127.0.0.1` 返回 `400 VALIDATION_ERROR`；创建、编辑成功；集成测试覆盖删除级联 |
| F-02 | 成功扫描公网站点 | 通过 | `example.org` 返回 `completed`、HTTP 200、最终 URL、评分 42、7 项问题 |
| F-03 | 失败扫描持久化 | 通过 | `expired.badssl.com` 返回 `failed`，错误码 `FETCH_TIMEOUT`，消息为 TLS handshake timeout；未停留在 `running` |
| F-04 | 问题状态更新与事件 | 通过 | `CANONICAL_MISSING` 由 `open` 更新为 `resolved`，`issue_events` 新增 1 条 |
| F-05 | 已修复问题再次出现 | 通过 | 第二次扫描后自动重开为 `open`，`firstSeenAt` 保留，新增 1 条自动重开事件 |
| F-06 | 扫描历史差异对比 | 通过 | 第二次扫描对比第一次：`new=0 / persisting=7 / resolved=0`；集成测试另覆盖 `1/1/1` 场景 |
| F-07 | HTML / JSON 报告 | 通过 | HTML 5967 字节，JSON `formatVersion=1.0`、7 条问题；转义测试通过 |
| F-08 | 工作台统计 | 通过 | 2 个网站、3 次扫描、7 个待处理问题、平均分 42，与数据库一致 |
| F-09 | SSRF 与目标安全 | 通过 | 核心单元测试覆盖 localhost、私网、元数据、IPv4 映射、非 80/443 端口、重定向校验 |
| F-10 | 前端关键路径 | 通过 | `npm run build` 通过；工作台与问题中心页面在 Chrome Headless 中完成截图验证 |

数据库一致性抽查：

```text
scans:        completed=2, failed=1
findings:     open=15（含失败扫描的 critical 记录）
issue_events: open->resolved=1, resolved->open=1
sites:        2
```

## 3. 观察与限制

1. **失败错误码受网络环境影响**：本次实时请求在 TLS 握手阶段超时，因此记录为 `FETCH_TIMEOUT` 而不是 `TLS_ERROR`；核心单元测试已覆盖证书错误到 `TLS_ERROR` 的分类。两者都满足“失败原因可解释、任务不悬挂”的功能要求。
2. **仅验证 Chrome 桌面端**：尚未做 Safari、Firefox 和真实移动设备验证。
3. **仅验证首页扫描**：多页面、JavaScript 渲染、内链发现不在当前版本范围。
4. **未验证公网部署**：当前无登录与限流，按规范只允许本地/受控内网运行。
5. **未发现阻塞性功能缺陷**：本轮验证没有发现需要阻断后续开发的问题。

## 4. 结论与下一步

当前 MVP 核心闭环已通过功能验证：

```text
创建网站 -> 扫描 -> 查看问题 -> 更新状态 -> 重新扫描 -> 自动重开/对比 -> 查看报告
```

下一阶段建议：

1. 将本报告中的验证流程接入 CI（MySQL service + `scripts/verify`）。
2. 增加移动端与跨浏览器回归。
3. 在功能回归稳定的前提下，进入异步扫描任务队列开发。
4. 登录认证继续暂缓，直到功能验证稳定并决定公网部署。
