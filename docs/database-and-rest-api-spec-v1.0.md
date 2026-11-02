# GeoPilot 数据库设计与 REST API 接口规范 V1.0

V1.0

数据库规范

API 规范

待评审冻结

本文承接《GeoPilot 产品需求规格说明书 V1.0》和《GeoPilot 功能实现与技术设计文档 V1.0》，将业务需求落实为可直接用于开发的数据库结构、REST API 契约、错误码、权限约束及验收要求。

## 一、技术基线与设计决策

| 项目     | V1.0 设计规范                               |
| ------ | --------------------------------------- |
| 项目名称   | GeoPilot                                |
| 数据库    | MySQL 8.0+                              |
| 数据库字符集 | `utf8mb4`                               |
| 数据库时区  | UTC 存储，API 使用 ISO 8601 时间格式             |
| 后端     | Go + REST API                           |
| ORM    | GORM 可作为实现选择，数据库迁移 SQL 为结构权威来源          |
| API 版本 | `/api/v1`                               |
| 数据格式   | JSON                                    |
| 主键     | `BIGINT UNSIGNED` 自增主键；外部任务 ID 使用独立公开标识 |
| 分页     | `page` + `page_size`                    |
| 报告格式   | HTML + JSON                             |
| 扫描范围   | 首版默认首页，可通过后续版本扩展                        |
| 部署建议   | 自托管、单实例优先；公网部署必须启用认证与安全防护               |

本规范中的默认产品决策采用：单用户管理员登录、首页扫描、保留历史快照、支持问题整改状态、HTML 与 JSON 报告。如后续决定改为多租户 SaaS，需要进一步增加租户模型、资源隔离和配额机制。

## 二、数据库总体设计

### 2.1 核心实体关系

\#chatgpt-mermaid-\_r_3c5\_{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;fill:rgb(13, 13, 13);}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#chatgpt-mermaid-\_r_3c5\_ .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_3c5\_ .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_3c5\_ .error-icon{fill:rgb(243, 243, 243);}#chatgpt-mermaid-\_r_3c5\_ .error-text{fill:rgb(13, 13, 13);stroke:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_3c5\_ .edge-thickness-normal{stroke-width:1px;}#chatgpt-mermaid-\_r_3c5\_ .edge-thickness-thick{stroke-width:3.5px;}#chatgpt-mermaid-\_r_3c5\_ .edge-pattern-solid{stroke-dasharray:0;}#chatgpt-mermaid-\_r_3c5\_ .edge-thickness-invisible{stroke-width:0;fill:none;}#chatgpt-mermaid-\_r_3c5\_ .edge-pattern-dashed{stroke-dasharray:3;}#chatgpt-mermaid-\_r_3c5\_ .edge-pattern-dotted{stroke-dasharray:2;}#chatgpt-mermaid-\_r_3c5\_ .marker{fill:rgb(143, 143, 143);stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_3c5\_ .marker.cross{stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_3c5\_ svg{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;}#chatgpt-mermaid-\_r_3c5\_ p{margin:0;}#chatgpt-mermaid-\_r_3c5\_ .entityBox{fill:rgb(222, 234, 251);stroke:rgb(83, 154, 248);}#chatgpt-mermaid-\_r_3c5\_ .relationshipLabelBox{fill:rgb(243, 243, 243);opacity:0.7;background-color:rgb(243, 243, 243);}#chatgpt-mermaid-\_r_3c5\_ .relationshipLabelBox rect{opacity:0.5;}#chatgpt-mermaid-\_r_3c5\_ .labelBkg{background-color:rgba(243, 243, 243, 0.5);}#chatgpt-mermaid-\_r_3c5\_ .edgeLabel{background-color:rgb(252, 252, 252);}#chatgpt-mermaid-\_r_3c5\_ .edgeLabel .label rect{fill:rgb(252, 252, 252);}#chatgpt-mermaid-\_r_3c5\_ .edgeLabel .label text{fill:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_3c5\_ .edgeLabel .label{fill:rgb(83, 154, 248);font-size:14px;}#chatgpt-mermaid-\_r_3c5\_ .label{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";color:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_3c5\_ .edge-pattern-dashed{stroke-dasharray:8,8;}#chatgpt-mermaid-\_r_3c5\_ .node rect,#chatgpt-mermaid-\_r_3c5\_ .node circle,#chatgpt-mermaid-\_r_3c5\_ .node ellipse,#chatgpt-mermaid-\_r_3c5\_ .node polygon{fill:rgb(222, 234, 251);stroke:rgb(83, 154, 248);stroke-width:1px;}#chatgpt-mermaid-\_r_3c5\_ .relationshipLine{stroke:rgb(143, 143, 143);stroke-width:1px;fill:none;}#chatgpt-mermaid-\_r_3c5\_ .marker{fill:none!important;stroke:rgb(143, 143, 143)!important;stroke-width:1;}#chatgpt-mermaid-\_r_3c5\_ [data-look=neo].labelBkg{background-color:rgba(243, 243, 243, 0.5);}#chatgpt-mermaid-\_r_3c5\_ .node .neo-node{stroke:rgb(83, 154, 248);}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].node rect,#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].cluster rect,#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].node polygon{stroke:url(#chatgpt-mermaid-\_r_3c5\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].swimlane.cluster rect{filter:none;}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].node path{stroke:url(#chatgpt-mermaid-\_r_3c5\_-gradient);stroke-width:1px;}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].node .outer-path{filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].node .neo-line path{stroke:rgb(83, 154, 248);filter:none;}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].node circle{stroke:url(#chatgpt-mermaid-\_r_3c5\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].node circle .state-start{fill:#000000;}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].icon-shape .icon{fill:url(#chatgpt-mermaid-\_r_3c5\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_3c5\_ [data-look="neo"].icon-shape .icon-neo path{stroke:url(#chatgpt-mermaid-\_r_3c5\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_3c5\_ :root{--mermaid-font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";}USERSbigintidPKvarcharemailUKvarcharpassword_hashvarcharstatusWEBSITESbigintidPKbigintowner_idFKvarcharnamevarcharbase_urlvarcharstatusAUDIT_LOGSSCAN_TASKSbigintidPKvarcharpublic_idUKbigintwebsite_idFKvarcharstatusintscorevarcharruleset_versionPAGE_SNAPSHOTSbigintidPKbigintscan_task_idFKvarcharpage_urlinthttp_statusvarcharcontent_hashSCAN_ISSUESbigintidPKbigintscan_task_idFKbigintsnapshot_idFKvarcharrule_idvarcharseverityvarcharissue_statusSCAN_REPORTSbigintidPKbigintscan_task_idFKvarcharformatvarcharstorage_pathISSUE_EVENTSbigintidPKbigintissue_idFKvarcharold_statusvarcharnew_statusownsperformshascapturesdiscoversevidence_forgeneratestracks

### 2.2 表清单

| 表名               | 说明        | V1.0 |
| ---------------- | --------- | ---- |
| `users`          | 管理员账户     | 必须   |
| `websites`       | 网站配置      | 必须   |
| `scan_tasks`     | 扫描任务及汇总结果 | 必须   |
| `page_snapshots` | 页面扫描快照    | 必须   |
| `scan_issues`    | 诊断问题与整改状态 | 必须   |
| `issue_events`   | 问题状态变化记录  | 必须   |
| `scan_reports`   | 报告元数据     | 必须   |
| `audit_logs`     | 安全敏感操作审计  | 建议启用 |

# 三、数据库字段规范

## 3.1 `users`：用户表

用于单用户管理员登录，同时为后续扩展多个账户预留结构。

| 字段              | 类型              | 约束               | 说明                    |
| --------------- | --------------- | ---------------- | --------------------- |
| `id`            | BIGINT UNSIGNED | PK               | 用户 ID                 |
| `email`         | VARCHAR(254)    | NOT NULL, UNIQUE | 登录邮箱                  |
| `password_hash` | VARCHAR(255)    | NOT NULL         | 密码哈希，不存明文             |
| `display_name`  | VARCHAR(100)    | NOT NULL         | 显示名称                  |
| `role`          | VARCHAR(20)     | NOT NULL         | `admin` / `viewer`    |
| `status`        | VARCHAR(20)     | NOT NULL         | `active` / `disabled` |
| `last_login_at` | DATETIME(3)     | NULL             | 最近登录时间                |
| `created_at`    | DATETIME(3)     | NOT NULL         | 创建时间                  |
| `updated_at`    | DATETIME(3)     | NOT NULL         | 更新时间                  |

安全约束：

- 密码使用经过验证的密码哈希算法，例如 Argon2id 或 bcrypt。
- 不提供公开注册接口，首个管理员通过安全的初始化流程创建。
- 不允许通过普通 API 将最后一个管理员降权或禁用。
- 登录失败应限流，必要时加入短期账户锁定机制。
- V1.0 可以使用服务端会话或安全 Cookie；不应将长期有效的认证令牌存入浏览器 `localStorage`。

## 3.2 `websites`：网站表

| 字段            | 类型              | 约束           | 说明                    |
| ------------- | --------------- | ------------ | --------------------- |
| `id`          | BIGINT UNSIGNED | PK           | 内部 ID                 |
| `owner_id`    | BIGINT UNSIGNED | FK, NOT NULL | 所属用户                  |
| `name`        | VARCHAR(120)    | NOT NULL     | 网站名称                  |
| `base_url`    | VARCHAR(2048)   | NOT NULL     | 标准化后的 URL             |
| `description` | VARCHAR(500)    | NULL         | 备注                    |
| `status`      | VARCHAR(20)     | NOT NULL     | `active` / `disabled` |
| `created_at`  | DATETIME(3)     | NOT NULL     | 创建时间                  |
| `updated_at`  | DATETIME(3)     | NOT NULL     | 更新时间                  |
| `deleted_at`  | DATETIME(3)     | NULL         | 软删除时间                 |

索引建议：

- `idx_websites_owner_status (owner_id, status)`
- `idx_websites_owner_created (owner_id, created_at)`
- 如需按规范化 URL 去重，应另存规范化后的唯一键；不建议直接对 2048 字符 URL 列创建普通完整长度索引。

网站删除采用软删除。历史扫描任务和报告默认保留，但网站被删除后，不再允许启动新扫描。

## 3.3 `scan_tasks`：扫描任务表

该表是任务生命周期和扫描结果汇总的核心。

| 字段                | 类型               | 约束               | 说明              |
| ----------------- | ---------------- | ---------------- | --------------- |
| `id`              | BIGINT UNSIGNED  | PK               | 内部 ID           |
| `public_id`       | VARCHAR(40)      | UNIQUE, NOT NULL | 外部任务 ID         |
| `website_id`      | BIGINT UNSIGNED  | FK, NOT NULL     | 所属网站            |
| `requested_by`    | BIGINT UNSIGNED  | FK, NULL         | 发起用户            |
| `retry_of_id`     | BIGINT UNSIGNED  | NULL             | 来源失败任务内部 ID     |
| `status`          | VARCHAR(20)      | NOT NULL         | 任务状态            |
| `scan_scope`      | VARCHAR(20)      | NOT NULL         | 首版固定 `homepage` |
| `requested_url`   | VARCHAR(2048)    | NOT NULL         | 请求目标            |
| `final_url`       | VARCHAR(2048)    | NULL             | 最终访问 URL        |
| `score`           | TINYINT UNSIGNED | NULL             | 0–100，扫描有效后生成   |
| `issue_count`     | INT UNSIGNED     | NOT NULL         | 问题总数            |
| `critical_count`  | INT UNSIGNED     | NOT NULL         | 严重问题数           |
| `high_count`      | INT UNSIGNED     | NOT NULL         | 高优先级问题数         |
| `medium_count`    | INT UNSIGNED     | NOT NULL         | 中优先级问题数         |
| `low_count`       | INT UNSIGNED     | NOT NULL         | 低优先级问题数         |
| `ruleset_version` | VARCHAR(40)      | NOT NULL         | 规则集版本           |
| `scoring_version` | VARCHAR(40)      | NOT NULL         | 评分算法版本          |
| `config_snapshot` | JSON             | NULL             | 本次扫描关键配置快照      |
| `error_code`      | VARCHAR(64)      | NULL             | 任务失败代码          |
| `error_summary`   | VARCHAR(500)     | NULL             | 安全的错误摘要         |
| `queued_at`       | DATETIME(3)      | NOT NULL         | 入队时间            |
| `started_at`      | DATETIME(3)      | NULL             | 开始时间            |
| `finished_at`     | DATETIME(3)      | NULL             | 结束时间            |
| `created_at`      | DATETIME(3)      | NOT NULL         | 创建时间            |
| `updated_at`      | DATETIME(3)      | NOT NULL         | 更新时间            |

索引：

- `UNIQUE KEY uk_scan_tasks_public_id (public_id)`
- `idx_scan_tasks_website_created (website_id, created_at)`
- `idx_scan_tasks_status_queued (status, queued_at)`
- `idx_scan_tasks_retry_of (retry_of_id)`

设计说明：

1. 外部 API 仅暴露 `public_id`，不暴露数据库自增主键。
2. `score` 为 `NULL` 表示尚未产生有效评分，不应使用 0 冒充失败结果。
3. 失败任务可以没有分数，但仍需保留错误代码和生命周期信息。
4. `config_snapshot` 只存本次扫描必要的非敏感配置，不能保存密码、Cookie 或 API 密钥。
5. `retry_of_id` 指向原任务，但重试创建的是新任务。
6. 任务状态变化应通过业务层控制，并用数据库条件更新防止并发重复领取。

### 3.3.1 任务状态

| 状态          | 说明           | 终态 |
| ----------- | ------------ | -- |
| `queued`    | 等待执行         | 否  |
| `running`   | 正在执行         | 否  |
| `succeeded` | 扫描完整成功       | 是  |
| `partial`   | 部分步骤失败，但结果可用 | 是  |
| `failed`    | 无法形成有效扫描结果   | 是  |
| `cancelled` | 任务已取消        | 是  |

合法状态变化：

```
queued -> running
queued -> cancelled

running -> succeeded
running -> partial
running -> failed
running -> cancelled
```

不允许通过普通更新接口将终态任务直接改回 `running`。

## 3.4 `page_snapshots`：页面快照表

每个任务可以对应一个或多个页面。V1.0 默认只有首页，但数据结构不限制未来扩展。

| 字段                  | 类型                     | 说明               |
| ------------------- | ---------------------- | ---------------- |
| `id`                | BIGINT UNSIGNED PK     | 快照 ID            |
| `scan_task_id`      | BIGINT UNSIGNED FK     | 所属任务             |
| `page_url`          | VARCHAR(2048)          | 本次请求 URL         |
| `final_url`         | VARCHAR(2048) NULL     | 最终 URL           |
| `http_status`       | SMALLINT UNSIGNED NULL | HTTP 状态码         |
| `content_type`      | VARCHAR(255) NULL      | 响应类型             |
| `response_bytes`    | BIGINT UNSIGNED        | 响应体字节数           |
| `duration_ms`       | INT UNSIGNED           | 抓取耗时             |
| `title`             | VARCHAR(1000) NULL     | 页面标题             |
| `meta_description`  | TEXT NULL              | Meta Description |
| `canonical_url`     | VARCHAR(2048) NULL     | Canonical        |
| `h1_count`          | INT UNSIGNED           | H1 数量            |
| `h1_texts`          | JSON NULL              | H1 文本数组          |
| `language`          | VARCHAR(35) NULL       | `html lang` 声明   |
| `word_count`        | INT UNSIGNED           | 抽取正文词数           |
| `schema_types`      | JSON NULL              | 检测到的结构化数据类型      |
| `robots_directives` | JSON NULL              | Robots Meta 指令   |
| `content_hash`      | CHAR(64) NULL          | 规范化内容 SHA-256    |
| `parse_warnings`    | JSON NULL              | 解析警告             |
| `created_at`        | DATETIME(3)            | 创建时间             |

索引：

- `idx_snapshots_task (scan_task_id)`
- `idx_snapshots_hash (content_hash)`

注意：快照存储的是页面分析特征，而非默认保存完整 HTML 正文。避免不必要地保存用户网站上的个人信息、会话数据或其他敏感内容。

## 3.5 `scan_issues`：诊断问题表

建议将“单次扫描发现的问题”与“问题整改状态”放在同一记录中，通过历史快照比较推导问题是否再次出现。

| 字段             | 类型                      | 说明                                     |
| -------------- | ----------------------- | -------------------------------------- |
| `id`           | BIGINT UNSIGNED PK      | 问题 ID                                  |
| `public_id`    | VARCHAR(40) UNIQUE      | 外部问题 ID                                |
| `scan_task_id` | BIGINT UNSIGNED FK      | 所属扫描任务                                 |
| `snapshot_id`  | BIGINT UNSIGNED FK NULL | 对应页面快照                                 |
| `rule_id`      | VARCHAR(64)             | 规则 ID                                  |
| `rule_version` | VARCHAR(40)             | 规则版本                                   |
| `category`     | VARCHAR(40)             | 问题分类                                   |
| `severity`     | VARCHAR(20)             | `critical` / `high` / `medium` / `low` |
| `title`        | VARCHAR(255)            | 问题标题                                   |
| `description`  | TEXT                    | 问题说明                                   |
| `evidence`     | JSON NULL               | 证据数据                                   |
| `suggestion`   | TEXT                    | 整改建议                                   |
| `deduction`    | TINYINT UNSIGNED        | 本次评分扣分                                 |
| `fingerprint`  | CHAR(64)                | 问题指纹                                   |
| `issue_status` | VARCHAR(20)             | `open` / `resolved` / `ignored`        |
| `status_note`  | VARCHAR(500) NULL       | 状态备注                                   |
| `resolved_at`  | DATETIME(3) NULL        | 标记修复时间                                 |
| `created_at`   | DATETIME(3)             | 创建时间                                   |
| `updated_at`   | DATETIME(3)             | 更新时间                                   |

索引：

- `idx_issues_task_severity (scan_task_id, severity)`
- `idx_issues_task_status (scan_task_id, issue_status)`
- `idx_issues_fingerprint (fingerprint)`
- `idx_issues_snapshot (snapshot_id)`

### 问题指纹设计

`fingerprint` 用于识别跨扫描重复出现的同类问题。建议由以下规范化内容计算：

```
SHA256(
  normalized_page_url
  + rule_id
  + normalized_evidence_key
)
```

不要直接将可能含有个人信息的正文片段拼入指纹。

V1.0 应明确一个语义边界：`scan_issues` 记录的是某次扫描的发现，`issue_status` 表示用户对这条发现的处理状态。跨扫描的“同一问题”可以通过 `fingerprint` 关联，但不应在没有明确规则的情况下自动把旧问题标记为已修复。

如果后续需要跨扫描维持一个长期问题身份，可升级为独立的 `issue_instances` 表。

## 3.6 `issue_events`：整改历史表

记录问题状态变化，便于追踪整改过程。

| 字段           | 类型                      | 说明    |
| ------------ | ----------------------- | ----- |
| `id`         | BIGINT UNSIGNED PK      | 事件 ID |
| `issue_id`   | BIGINT UNSIGNED FK      | 问题 ID |
| `actor_id`   | BIGINT UNSIGNED FK NULL | 操作者   |
| `old_status` | VARCHAR(20) NULL        | 原状态   |
| `new_status` | VARCHAR(20)             | 新状态   |
| `note`       | VARCHAR(500) NULL       | 变更备注  |
| `created_at` | DATETIME(3)             | 发生时间  |

所有状态变更必须先校验资源访问权限，再通过事务完成问题更新和事件写入，避免状态已改变但历史记录缺失。

## 3.7 `scan_reports`：报告表

| 字段                | 类型                   | 说明                                |
| ----------------- | -------------------- | --------------------------------- |
| `id`              | BIGINT UNSIGNED PK   | 报告 ID                             |
| `public_id`       | VARCHAR(40) UNIQUE   | 外部报告 ID                           |
| `scan_task_id`    | BIGINT UNSIGNED FK   | 所属任务                              |
| `format`          | VARCHAR(20)          | `html` / `json`                   |
| `status`          | VARCHAR(20)          | `generating` / `ready` / `failed` |
| `storage_path`    | VARCHAR(1024) NULL   | 服务端存储路径                           |
| `file_size_bytes` | BIGINT UNSIGNED NULL | 文件大小                              |
| `checksum`        | CHAR(64) NULL        | 文件 SHA-256                        |
| `error_summary`   | VARCHAR(500) NULL    | 生成失败摘要                            |
| `created_at`      | DATETIME(3)          | 创建时间                              |
| `updated_at`      | DATETIME(3)          | 更新时间                              |

`storage_path` 仅用于服务端内部访问，API 不应把真实文件系统绝对路径返回给客户端。

## 3.8 `audit_logs`：安全审计表

| 字段                   | 类型                   | 说明                    |
| -------------------- | -------------------- | --------------------- |
| `id`                 | BIGINT UNSIGNED PK   | 日志 ID                 |
| `actor_id`           | BIGINT UNSIGNED NULL | 操作者                   |
| `action`             | VARCHAR(80)          | 操作类型                  |
| `resource_type`      | VARCHAR(40)          | 资源类型                  |
| `resource_public_id` | VARCHAR(64) NULL     | 资源公开 ID               |
| `result`             | VARCHAR(20)          | `success` / `failure` |
| `request_id`         | VARCHAR(64) NULL     | 请求追踪 ID               |
| `ip_hash`            | CHAR(64) NULL        | 可选的 IP 哈希             |
| `details`            | JSON NULL            | 脱敏后的附加信息              |
| `created_at`         | DATETIME(3)          | 发生时间                  |

不得在审计日志中写入密码、认证 Cookie、会话令牌或完整的敏感请求正文。

# 四、数据库迁移与完整性要求

## 4.1 迁移规范

所有数据库变更通过版本化迁移脚本执行，例如：

```
backend/migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_websites.up.sql
├── 000002_create_websites.down.sql
├── 000003_create_scan_tasks.up.sql
├── 000003_create_scan_tasks.down.sql
├── 000004_create_page_snapshots.up.sql
├── 000004_create_page_snapshots.down.sql
├── 000005_create_scan_issues.up.sql
├── 000005_create_scan_issues.down.sql
├── 000006_create_issue_events.up.sql
├── 000006_create_issue_events.down.sql
├── 000007_create_scan_reports.up.sql
├── 000007_create_scan_reports.down.sql
└── 000008_create_audit_logs.up.sql
```

以上是建议的文件拆分。实际执行时必须验证外键创建顺序和迁移工具的兼容性。

## 4.2 数据一致性规则

1. 删除网站不得级联删除历史扫描记录。
2. 删除用户前必须明确处理其网站和审计记录；V1.0 可禁止删除最后一个管理员。
3. 扫描结果写入应尽量采用事务。
4. 报告生成与数据库事务分离时，必须设计失败补偿。
5. 所有计数字段不得为负数。
6. 评分只能为 `NULL` 或 0–100。
7. 问题级别、任务状态、报告格式等字段必须经过应用层枚举校验。
8. 查询接口必须校验资源归属，不能仅凭外部 ID 难以猜测就认为安全。

# 五、REST API 通用约定

## 5.1 URL 与 HTTP 方法

统一前缀：

```
/api/v1
```

资源命名使用复数名词，路径采用小写字母与连字符；请求体使用 JSON。

| 方法       | 语义        | 示例                             |
| -------- | --------- | ------------------------------ |
| `GET`    | 查询资源      | `GET /api/v1/websites`         |
| `POST`   | 创建资源或触发操作 | `POST /api/v1/websites`        |
| `PATCH`  | 局部更新      | `PATCH /api/v1/websites/{id}`  |
| `DELETE` | 删除资源      | `DELETE /api/v1/websites/{id}` |

## 5.2 统一成功响应

```
{
  "code": "OK",
  "message": "success",
  "data": {
    "id": "1001"
  },
  "request_id": "req_example"
}
```

说明：

- `code` 为稳定业务代码。
- `message` 用于展示或调试，不应作为程序判断条件。
- `data` 的结构由接口定义。
- `request_id` 用于日志关联。
- 示例 ID 仅用于说明格式。

对于 `BIGINT UNSIGNED`，建议在 JSON 中使用字符串表达，避免 JavaScript `number` 的安全整数范围问题。

## 5.3 统一错误响应

```
{
  "code": "VALIDATION_ERROR",
  "message": "The request is invalid.",
  "details": [
    {
      "field": "name",
      "reason": "required"
    }
  ],
  "request_id": "req_example"
}
```

错误响应不得包含数据库密码、堆栈、内部文件路径、完整内部网络地址或其他敏感实现细节。

## 5.4 分页规范

请求：

```
GET /api/v1/websites?page=1&page_size=20
```

约束：

- 默认 `page=1`。
- 默认 `page_size=20`。
- `page_size` 最大 100。
- 非法页码返回 `VALIDATION_ERROR`。
- 列表默认按 `created_at DESC, id DESC` 排序。
- 不允许将未经校验的客户端输入直接拼接为 SQL 排序字段。

响应：

```
{
  "code": "OK",
  "message": "success",
  "data": {
    "items": [],
    "page": 1,
    "page_size": 20,
    "total": 0
  },
  "request_id": "req_example"
}
```

# 六、认证与权限 API

## 6.1 接口列表

| 方法     | 路径                      | 权限     | 用途     |
| ------ | ----------------------- | ------ | ------ |
| `POST` | `/auth/login`           | 公开但需限流 | 管理员登录  |
| `POST` | `/auth/logout`          | 已登录    | 注销当前会话 |
| `GET`  | `/auth/me`              | 已登录    | 查询当前用户 |
| `POST` | `/auth/change-password` | 已登录    | 修改密码   |

推荐采用服务端会话与 `HttpOnly`、`Secure`、合适的 `SameSite` Cookie。采用 Cookie 认证时，必须防范 CSRF，并对修改操作实施来源验证或 CSRF Token 策略。

登录请求示例：

```
{
  "email": "admin@example.com",
  "password": "user-supplied-password"
}
```

这里的邮箱和密码只是字段示例，不能作为真实部署凭据。

成功后返回当前用户信息，并通过安全 Cookie 建立会话。不要将密码或长期有效会话令牌返回到页面 JavaScript 中。

# 七、网站管理 API

## 7.1 网站列表

`GET /api/v1/websites?page=1&page_size=20&status=active`

支持参数：

| 参数          | 类型      | 必填 | 说明                    |
| ----------- | ------- | -- | --------------------- |
| `page`      | integer | 否  | 页码                    |
| `page_size` | integer | 否  | 每页条数                  |
| `status`    | string  | 否  | `active` / `disabled` |
| `keyword`   | string  | 否  | 名称搜索                  |

列表建议返回最近扫描的摘要，但不要为每个网站额外执行实时扫描。

## 7.2 创建网站

`POST /api/v1/websites`

请求：

```
{
  "name": "Company Website",
  "base_url": "https://example.com",
  "description": "Official company website"
}
```

响应：

```
{
  "code": "OK",
  "message": "success",
  "data": {
    "id": "1001",
    "name": "Company Website",
    "base_url": "https://example.com",
    "status": "active",
    "created_at": "2026-11-02T08:00:00Z"
  },
  "request_id": "req_example"
}
```

创建时需要：

1. 验证字段长度与必填项。
2. 标准化 URL。
3. 拒绝不支持的协议、凭据、非法端口。
4. 检查 DNS 和目标 IP。
5. 检查重复网站。
6. 保存当前用户为 `owner_id`。
7. 写入审计日志。

DNS 检查不能只发生在创建时。每次实际扫描都必须重新验证目标。

## 7.3 查询、更新与删除

| 方法       | 路径               | 说明          |
| -------- | ---------------- | ----------- |
| `GET`    | `/websites/{id}` | 查询网站详情      |
| `PATCH`  | `/websites/{id}` | 更新名称、备注、状态等 |
| `DELETE` | `/websites/{id}` | 软删除网站       |

不允许普通更新请求直接修改 `owner_id`。修改 `base_url` 时必须重新进行完整 URL 安全验证。

# 八、扫描任务 API

## 8.1 创建扫描

`POST /api/v1/websites/{id}/scans`

请求：

```
{
  "scope": "homepage"
}
```

服务端行为：

1. 校验用户是否拥有网站访问权限。
2. 检查网站是否处于启用状态。
3. 校验当前扫描配额和并发限制。
4. 生成唯一公开任务 ID。
5. 创建 `queued` 状态任务。
6. 将任务交给执行器。
7. 返回任务 ID。

响应：

```
{
  "code": "OK",
  "message": "scan accepted",
  "data": {
    "scan_id": "scn_example",
    "status": "queued",
    "created_at": "2026-11-02T08:00:00Z"
  },
  "request_id": "req_example"
}
```

扫描接口返回成功只代表任务已创建，不代表目标网站已经扫描成功。

## 8.2 查询扫描任务

`GET /api/v1/scans/{scan_id}`

响应示例：

```
{
  "code": "OK",
  "message": "success",
  "data": {
    "scan_id": "scn_example",
    "website_id": "1001",
    "status": "succeeded",
    "score": 82,
    "issue_summary": {
      "total": 3,
      "critical": 0,
      "high": 1,
      "medium": 1,
      "low": 1
    },
    "started_at": "2026-11-02T08:00:01Z",
    "finished_at": "2026-11-02T08:00:04Z",
    "ruleset_version": "1.0.0",
    "scoring_version": "1.0.0"
  },
  "request_id": "req_example"
}
```

这里的评分和问题数量是示例数据，不是实际扫描结果。

## 8.3 扫描历史列表

`GET /api/v1/scans`

查询参数建议：

- `website_id`
- `status`
- `from`
- `to`
- `page`
- `page_size`

日期筛选统一采用 ISO 8601 时间或明确约定的 UTC 时间格式。必须验证开始时间早于结束时间，并对查询范围进行限制。

## 8.4 取消扫描

`POST /api/v1/scans/{scan_id}/cancel`

规则：

- `queued` 任务可以直接取消。
- `running` 任务应通过取消上下文通知执行器。
- 终态任务不允许取消。
- 取消成功后保留已产生的诊断记录，并记录最终状态。
- 取消接口需要幂等处理：重复取消不能破坏数据状态。

## 8.5 重试扫描

`POST /api/v1/scans/{scan_id}/retry`

建议只允许对 `failed` 或符合策略的 `partial` 任务执行重试。

服务端应创建新的扫描任务，写入 `retry_of_id`，保留旧任务及其报告。不能覆盖旧任务的分数和证据。

# 九、问题中心 API

## 9.1 问题列表

`GET /api/v1/scans/{scan_id}/issues`

支持过滤：

| 参数             | 说明      |
| -------------- | ------- |
| `severity`     | 按严重程度筛选 |
| `rule_id`      | 按规则筛选   |
| `issue_status` | 按整改状态筛选 |
| `page`         | 页码      |
| `page_size`    | 每页条数    |

问题详情建议包含规则 ID、证据、整改建议、扣分值和整改状态。

## 9.2 更新问题状态

`PATCH /api/v1/issues/{issue_id}`

请求：

```
{
  "issue_status": "resolved",
  "status_note": "Updated the page title and verified the fix."
}
```

允许的状态：

- `open`：待处理。
- `resolved`：已修复。
- `ignored`：已忽略。

业务约束：

1. 只能更新有权限访问的问题。
2. 状态不变时应保持幂等，不重复产生无意义事件。
3. 状态变化必须写入 `issue_events`。
4. 标记为 `resolved` 仅表示用户确认修复，不等于系统已经重新验证。
5. 重新扫描后发现同指纹问题时，应提示用户重新确认状态，不能自动认定旧问题已经修复。

建议未来增加“重新验证”功能：用户标记已修复后，触发新扫描，比较同类问题是否仍存在。

# 十、报告 API

## 10.1 接口列表

| 方法     | 路径                                     | 说明        |
| ------ | -------------------------------------- | --------- |
| `POST` | `/api/v1/scans/{scan_id}/reports`      | 创建或重新生成报告 |
| `GET`  | `/api/v1/scans/{scan_id}/reports`      | 查询报告列表    |
| `GET`  | `/api/v1/reports/{report_id}`          | 查询报告元数据   |
| `GET`  | `/api/v1/reports/{report_id}/download` | 下载报告      |

创建报告请求：

```
{
  "format": "html"
}
```

允许格式为 `html` 和 `json`。

响应：

```
{
  "code": "OK",
  "message": "report generation accepted",
  "data": {
    "report_id": "rpt_example",
    "format": "html",
    "status": "generating"
  },
  "request_id": "req_example"
}
```

报告下载接口必须验证资源权限，不能因为用户知道报告 ID 就允许下载。

HTML 报告必须对页面标题、证据和其他外部内容执行上下文适配的 HTML 转义，防止恶意页面内容被作为脚本执行。

# 十一、错误码正式建议

| 错误码                   | HTTP 状态     | 说明         | 是否可重试 |
| --------------------- | ----------- | ---------- | ----- |
| `OK`                  | 200/201/202 | 成功         | 不适用   |
| `VALIDATION_ERROR`    | 400         | 参数非法       | 修改参数后 |
| `UNAUTHORIZED`        | 401         | 未认证或会话失效   | 重新认证  |
| `FORBIDDEN`           | 403         | 无资源权限      | 否     |
| `RESOURCE_NOT_FOUND`  | 404         | 资源不存在或不可见  | 否     |
| `DUPLICATE_RESOURCE`  | 409         | 资源重复       | 修改后   |
| `INVALID_STATE`       | 409         | 当前状态不允许操作  | 否     |
| `SCAN_LIMIT_REACHED`  | 429         | 并发或配额限制    | 稍后    |
| `RATE_LIMITED`        | 429         | 请求频率过高     | 稍后    |
| `TARGET_BLOCKED`      | 422         | 目标违反网络安全策略 | 修改目标  |
| `SCAN_NOT_RETRYABLE`  | 409         | 当前任务不可重试   | 否     |
| `REPORT_NOT_READY`    | 409         | 报告尚未生成     | 稍后    |
| `INTERNAL_ERROR`      | 500         | 内部错误       | 视情况   |
| `SERVICE_UNAVAILABLE` | 503         | 服务暂不可用     | 通常可以  |

### 扫描执行错误与 API 错误的区别

远端网站超时，不应导致已经成功创建的扫描任务对应 API 始终返回 504。正确做法是：

- 创建任务成功：返回 `202 Accepted`。
- 后台抓取超时：将任务结果标记为 `failed` 或 `partial`。
- 任务详情中记录 `FETCH_TIMEOUT`。
- 用户通过任务查询接口获取结果。

这样前端可以稳定显示扫描状态，并且便于排查问题。

# 十二、并发、事务与一致性

## 12.1 任务领取

扫描执行器应使用事务或条件更新，避免多个执行器领取同一任务。例如，任务从 `queued` 变为 `running` 时必须满足原状态仍为 `queued`。

概念 SQL：

```
UPDATE scan_tasks
SET status = 'running',
    started_at = UTC_TIMESTAMP(3),
    updated_at = UTC_TIMESTAMP(3)
WHERE id = ?
  AND status = 'queued';
```

应用层应检查受影响行数是否为 1。为 0 表示任务已被其他执行器领取或状态已经变化。

## 12.2 结果持久化

建议将以下操作放在同一个数据库事务中：

1. 创建页面快照。
2. 写入本次扫描发现的问题。
3. 写入问题汇总与评分。
4. 更新任务最终状态和完成时间。

报告文件的生成与保存不必与上述数据库事务绑定，但要通过 `scan_reports.status` 跟踪其生命周期。

## 12.3 幂等性

对于创建扫描、创建报告等可能被客户端重复提交的操作，建议支持 `Idempotency-Key`。

请求示例：

```
POST /api/v1/websites/1001/scans
Idempotency-Key: 8c9d-example-key
```

服务端应在一定保留时间内记录幂等键、请求主体摘要和执行结果。同一键但请求主体不同，应返回冲突错误，不能创建意外的第二个任务。

# 十三、API 安全规范

GeoPilot 的扫描功能会主动向外部网络发起请求，因此安全要求不仅包括用户权限，还包括扫描器自身的网络边界。

| 安全领域    | 必须实现的措施               |
| ------- | --------------------- |
| 认证      | 公网部署必须启用              |
| 授权      | 每次查询和修改都验证资源归属        |
| SSRF    | DNS、IP、端口、重定向和连接目标校验  |
| 网络出口    | 限制访问私网、环回、链路本地和元数据地址  |
| 请求限流    | 登录、创建扫描、报告生成等接口分别限流   |
| 响应限制    | 限制抓取大小、超时、重定向次数与并发    |
| HTML 注入 | 对报告中的外部内容进行正确转义       |
| SQL 注入  | 使用参数化查询，不拼接用户输入       |
| 敏感信息    | 日志与错误响应脱敏             |
| 文件安全    | 报告文件路径不能由用户直接控制       |
| 审计      | 登录、扫描、取消、重试、权限变更等操作留痕 |
| 跨站请求    | Cookie 认证需实施 CSRF 防护  |

上线门槛： 如果公网部署时尚未完成 SSRF 防护、身份认证、资源授权与网络出口限制，不应开放任意用户提交 URL 的扫描功能。

# 十四、前端 TypeScript 类型规范

前端应与 API 契约保持一致，不要让组件自行猜测字段类型。

```
export type ScanStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "partial"
  | "failed"
  | "cancelled";

export type IssueSeverity =
  | "critical"
  | "high"
  | "medium"
  | "low";

export type IssueStatus =
  | "open"
  | "resolved"
  | "ignored";

export interface Website {
  id: string;
  name: string;
  base_url: string;
  description?: string;
  status: "active" | "disabled";
  created_at: string;
  updated_at: string;
}

export interface ScanTask {
  scan_id: string;
  website_id: string;
  status: ScanStatus;
  score: number | null;
  issue_summary: {
    total: number;
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
  started_at: string | null;
  finished_at: string | null;
  ruleset_version: string;
  scoring_version: string;
}

export interface ScanIssue {
  issue_id: string;
  rule_id: string;
  severity: IssueSeverity;
  title: string;
  description: string;
  evidence: Record<string, unknown> | null;
  suggestion: string;
  deduction: number;
  issue_status: IssueStatus;
}
```

实际实现时，后端响应必须与这些类型严格对齐。推荐使用 OpenAPI 作为 API 契约来源，再生成或校验 TypeScript 类型，避免接口变更后前后端长期不一致。

# 十五、开发验收清单

## V1.0 验收进度

### 0/12

数据库初始化

所有表、索引、外键与迁移脚本可在空数据库执行。

管理员认证

未登录用户无法调用受保护接口。

资源权限

用户无法读取或修改无权访问的网站、任务、问题和报告。

网站 CRUD

网站创建、更新、列表、详情与软删除行为符合规范。

扫描状态机

非法状态转换被拒绝，重复领取不会导致重复执行。

页面快照

保存页面关键特征、响应信息、内容哈希与规则版本。

诊断问题

问题记录包含规则 ID、级别、证据和整改建议。

评分一致性

分数计算符合算法版本，抓取失败不会造成错误扣分。

历史整改

问题状态变化写入事件记录，旧扫描结果不会被覆盖。

报告下载

HTML 和 JSON 报告可访问，且路径安全、权限正确。

SSRF 防护

私网、环回、重定向绕过、DNS 变化和 IPv6 边界用例通过。

故障恢复

服务重启后能识别遗留任务，不会无条件重复执行。

重置清单复制验收清单

该清单用于跟踪验收准备情况。勾选只表示人工标记，不会自动执行测试，也不代表软件已通过测试。

# 十六、实施顺序与下一步

推荐按照以下顺序进入编码：

1. 冻结数据库模型与 API 契约：明确所有字段、状态枚举、错误码和权限语义。
2. 生成数据库迁移脚本：完成表结构、索引、外键和初始化管理员流程。
3. 实现网站管理 API：先打通认证、网站 CRUD 与资源权限。
4. 实现安全扫描器：先通过 SSRF 和网络边界测试，再接入规则引擎。
5. 实现扫描任务与问题管理：完成异步任务、结果持久化、历史快照和评分。
6. 实现报告与前端：根据已冻结的 API 契约开发工作台、问题中心和报告页面。
7. 执行验收与部署：完成安全测试、故障恢复测试及部署文档。

## 文档状态

当前版本已经覆盖数据库实体、字段、索引、状态机、主要 REST API、错误码、安全约束与验收基线。正式编码前仍应将它们整理为可执行的 SQL 迁移和完整的 OpenAPI 文件，并对关键安全行为进行测试。

建议下一步：编写《GeoPilot 数据库迁移 SQL + OpenAPI 3.1 完整接口定义 V1.0》。 这份文档将把本规范转化为可直接用于开发、生成接口文档、生成 TypeScript 类型和编写集成测试的工程文件。