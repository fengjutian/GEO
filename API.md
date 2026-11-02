# API 快速说明

- `GET /api/health`：健康检查
- `GET /api/sites`：网站列表
- `POST /api/sites`：新增网站，JSON `{ "name": "官网", "url": "https://example.com", "description": "" }`
- `GET /api/sites/:id`：网站与最近 20 次扫描
- `DELETE /api/sites/:id`：删除网站和相关记录
- `POST /api/sites/:id/scans`：执行一次首页扫描
- `GET /api/sites/:id/findings?status=open`：问题清单
- `GET /api/scans/:id/report`：HTML 报告

扫描目前是同步单页扫描，适合验证核心闭环。下一阶段应增加多页面爬取、任务取消、规则测试与问题状态更新。
