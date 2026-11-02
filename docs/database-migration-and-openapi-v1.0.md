# GeoPilot 数据库迁移 SQL + OpenAPI 3.1 完整接口定义 V1.0

**项目名称：** GeoPilot  
**文档版本：** V1.0  
**文档状态：** 待评审  
**数据库：** MySQL 8.0+  
**后端：** Go + REST API  
**前端：** React + TypeScript + Vite  
**API 规范：** OpenAPI 3.1.0  
**字符集：** utf8mb4  
**时间规范：** 数据库 UTC 时间；API 使用 ISO 8601 UTC 时间  
**核心原则：** 数据库迁移和 OpenAPI 契约是实现依据，前后端代码必须遵守本规范。

---

# 第一部分：工程规范

## 1.1 交付目标

本规范定义以下工程资产：

1. MySQL 数据库初始化与版本化迁移 SQL。
2. 用户、网站、扫描任务、页面快照、诊断问题、整改事件、报告和审计日志的数据模型。
3. REST API 完整路径、HTTP 方法、请求参数和响应 Schema。
4. Cookie 会话认证、资源授权、错误响应与分页规则。
5. 扫描任务状态、评分算法、报告状态及问题整改状态。
6. OpenAPI 3.1.0 YAML 文件。
7. 数据库迁移和接口契约验收要求。

## 1.2 工程目录

```text
geopilot/
├── docs/
│   ├── database-design-v1.0.md
│   ├── api-spec-v1.0.md
│   └── acceptance-v1.0.md
├── backend/
│   ├── cmd/geopilot/main.go
│   ├── internal/
│   │   ├── auth/
│   │   ├── website/
│   │   ├── scan/
│   │   ├── fetcher/
│   │   ├── analyzer/
│   │   ├── rules/
│   │   ├── issue/
│   │   ├── report/
│   │   ├── repository/
│   │   └── middleware/
│   ├── migrations/
│   │   ├── 000001_init_schema.up.sql
│   │   └── 000001_init_schema.down.sql
│   └── openapi/
│       └── openapi-v1.0.yaml
└── frontend/
    └── src/
        ├── services/
        ├── types/
        └── features/
```

V1.0 使用一份完整初始化迁移作为基线。后续任何结构变更必须创建新的迁移版本，不得修改已经在正式环境执行过的历史迁移文件。

## 1.3 统一约定

- 所有外部 API 使用 `/api/v1` 前缀。
- 内部数据库主键不作为资源授权凭据。
- 网站、任务、问题和报告使用独立公开 ID。
- JSON 中所有数据库 BIGINT 标识均以字符串传输。
- 列表接口默认 `page=1`、`page_size=20`，最大 `page_size=100`。
- 认证采用服务端会话和安全 Cookie。
- 创建异步任务返回 HTTP 202。
- 错误使用统一 `ErrorResponse`。
- 所有涉及资源的请求都必须校验资源归属。
- URL 抓取功能必须具备 SSRF 防护。
- 所有时间戳统一使用 UTC。

---

# 第二部分：MySQL 数据库迁移 SQL

## 2.1 迁移文件

文件：`backend/migrations/000001_init_schema.up.sql`

以下 SQL 为首版数据库初始化基线，使用 MySQL 8.0.16 及以上版本，以支持实际执行的 CHECK 约束。

```sql
SET NAMES utf8mb4;
SET time_zone = '+00:00';

CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    email VARCHAR(254) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'admin',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    last_login_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    UNIQUE KEY uk_users_email (email),
    KEY idx_users_status (status),

    CONSTRAINT chk_users_role
        CHECK (role IN ('admin', 'viewer')),
    CONSTRAINT chk_users_status
        CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE websites (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(120) NOT NULL,
    base_url VARCHAR(2048) NOT NULL,
    description VARCHAR(500) NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_websites_public_id (public_id),
    KEY idx_websites_owner_status (owner_id, status),
    KEY idx_websites_owner_created (owner_id, created_at, id),
    KEY idx_websites_deleted_at (deleted_at),

    CONSTRAINT fk_websites_owner
        FOREIGN KEY (owner_id) REFERENCES users(id)
        ON DELETE RESTRICT ON UPDATE RESTRICT,

    CONSTRAINT chk_websites_status
        CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE scan_tasks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    website_id BIGINT UNSIGNED NOT NULL,
    requested_by BIGINT UNSIGNED NULL,
    retry_of_id BIGINT UNSIGNED NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    scan_scope VARCHAR(20) NOT NULL DEFAULT 'homepage',

    requested_url VARCHAR(2048) NOT NULL,
    final_url VARCHAR(2048) NULL,

    score TINYINT UNSIGNED NULL,
    issue_count INT UNSIGNED NOT NULL DEFAULT 0,
    critical_count INT UNSIGNED NOT NULL DEFAULT 0,
    high_count INT UNSIGNED NOT NULL DEFAULT 0,
    medium_count INT UNSIGNED NOT NULL DEFAULT 0,
    low_count INT UNSIGNED NOT NULL DEFAULT 0,

    ruleset_version VARCHAR(40) NOT NULL DEFAULT '1.0.0',
    scoring_version VARCHAR(40) NOT NULL DEFAULT '1.0.0',
    config_snapshot JSON NULL,

    error_code VARCHAR(64) NULL,
    error_summary VARCHAR(500) NULL,

    queued_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    started_at DATETIME(3) NULL,
    finished_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    UNIQUE KEY uk_scan_tasks_public_id (public_id),
    KEY idx_scan_tasks_website_created (website_id, created_at, id),
    KEY idx_scan_tasks_status_queued (status, queued_at, id),
    KEY idx_scan_tasks_retry_of (retry_of_id),
    KEY idx_scan_tasks_requested_by (requested_by),

    CONSTRAINT fk_scan_tasks_website
        FOREIGN KEY (website_id) REFERENCES websites(id)
        ON DELETE RESTRICT ON UPDATE RESTRICT,

    CONSTRAINT fk_scan_tasks_requested_by
        FOREIGN KEY (requested_by) REFERENCES users(id)
        ON DELETE SET NULL ON UPDATE RESTRICT,

    CONSTRAINT fk_scan_tasks_retry_of
        FOREIGN KEY (retry_of_id) REFERENCES scan_tasks(id)
        ON DELETE SET NULL ON UPDATE RESTRICT,

    CONSTRAINT chk_scan_tasks_status
        CHECK (status IN (
            'queued', 'running', 'succeeded',
            'partial', 'failed', 'cancelled'
        )),

    CONSTRAINT chk_scan_tasks_scope
        CHECK (scan_scope IN ('homepage')),

    CONSTRAINT chk_scan_tasks_score
        CHECK (score IS NULL OR score BETWEEN 0 AND 100),

    CONSTRAINT chk_scan_tasks_time
        CHECK (
            finished_at IS NULL
            OR started_at IS NULL
            OR finished_at >= started_at
        )
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE page_snapshots (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    scan_task_id BIGINT UNSIGNED NOT NULL,

    page_url VARCHAR(2048) NOT NULL,
    final_url VARCHAR(2048) NULL,
    http_status SMALLINT UNSIGNED NULL,
    content_type VARCHAR(255) NULL,

    response_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    duration_ms INT UNSIGNED NOT NULL DEFAULT 0,

    title VARCHAR(1000) NULL,
    meta_description TEXT NULL,
    canonical_url VARCHAR(2048) NULL,

    h1_count INT UNSIGNED NOT NULL DEFAULT 0,
    h1_texts JSON NULL,
    language VARCHAR(35) NULL,
    word_count INT UNSIGNED NOT NULL DEFAULT 0,

    schema_types JSON NULL,
    robots_directives JSON NULL,
    content_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    parse_warnings JSON NULL,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    KEY idx_snapshots_task (scan_task_id),
    KEY idx_snapshots_hash (content_hash),

    CONSTRAINT fk_snapshots_task
        FOREIGN KEY (scan_task_id) REFERENCES scan_tasks(id)
        ON DELETE RESTRICT ON UPDATE RESTRICT,

    CONSTRAINT chk_snapshots_http_status
        CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE scan_issues (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

    scan_task_id BIGINT UNSIGNED NOT NULL,
    snapshot_id BIGINT UNSIGNED NULL,

    rule_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    rule_version VARCHAR(40) NOT NULL DEFAULT '1.0.0',
    category VARCHAR(40) NOT NULL,

    severity VARCHAR(20) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    evidence JSON NULL,
    suggestion TEXT NOT NULL,

    deduction TINYINT UNSIGNED NOT NULL DEFAULT 0,
    fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

    issue_status VARCHAR(20) NOT NULL DEFAULT 'open',
    status_note VARCHAR(500) NULL,
    resolved_at DATETIME(3) NULL,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    UNIQUE KEY uk_scan_issues_public_id (public_id),
    KEY idx_issues_task_severity (scan_task_id, severity),
    KEY idx_issues_task_status (scan_task_id, issue_status),
    KEY idx_issues_fingerprint (fingerprint),
    KEY idx_issues_snapshot (snapshot_id),
    KEY idx_issues_rule (rule_id),

    CONSTRAINT fk_issues_task
        FOREIGN KEY (scan_task_id) REFERENCES scan_tasks(id)
        ON DELETE RESTRICT ON UPDATE RESTRICT,

    CONSTRAINT fk_issues_snapshot
        FOREIGN KEY (snapshot_id) REFERENCES page_snapshots(id)
        ON DELETE RESTRICT ON UPDATE RESTRICT,

    CONSTRAINT chk_issues_severity
        CHECK (severity IN ('critical', 'high', 'medium', 'low')),

    CONSTRAINT chk_issues_status
        CHECK (issue_status IN ('open', 'resolved', 'ignored')),

    CONSTRAINT chk_issues_deduction
        CHECK (deduction BETWEEN 0 AND 100)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE issue_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    issue_id BIGINT UNSIGNED NOT NULL,
    actor_id BIGINT UNSIGNED NULL,

    old_status VARCHAR(20) NULL,
    new_status VARCHAR(20) NOT NULL,
    note VARCHAR(500) NULL,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    KEY idx_issue_events_issue_created (issue_id, created_at),
    KEY idx_issue_events_actor (actor_id),

    CONSTRAINT fk_issue_events_issue
        FOREIGN KEY (issue_id) REFERENCES scan_issues(id)
        ON DELETE RESTRICT ON UPDATE RESTRICT,

    CONSTRAINT fk_issue_events_actor
        FOREIGN KEY (actor_id) REFERENCES users(id)
        ON DELETE SET NULL ON UPDATE RESTRICT,

    CONSTRAINT chk_issue_events_old_status
        CHECK (
            old_status IS NULL
            OR old_status IN ('open', 'resolved', 'ignored')
        ),

    CONSTRAINT chk_issue_events_new_status
        CHECK (new_status IN ('open', 'resolved', 'ignored'))
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE scan_reports (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    scan_task_id BIGINT UNSIGNED NOT NULL,

    format VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'generating',

    storage_path VARCHAR(1024) NULL,
    file_size_bytes BIGINT UNSIGNED NULL,
    checksum CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    error_summary VARCHAR(500) NULL,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    UNIQUE KEY uk_scan_reports_public_id (public_id),
    KEY idx_scan_reports_task_created (scan_task_id, created_at),

    CONSTRAINT fk_scan_reports_task
        FOREIGN KEY (scan_task_id) REFERENCES scan_tasks(id)
        ON DELETE RESTRICT ON UPDATE RESTRICT,

    CONSTRAINT chk_scan_reports_format
        CHECK (format IN ('html', 'json')),

    CONSTRAINT chk_scan_reports_status
        CHECK (status IN ('generating', 'ready', 'failed'))
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    actor_id BIGINT UNSIGNED NULL,

    action VARCHAR(80) NOT NULL,
    resource_type VARCHAR(40) NOT NULL,
    resource_public_id VARCHAR(64) NULL,

    result VARCHAR(20) NOT NULL,
    request_id VARCHAR(64) NULL,
    ip_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    details JSON NULL,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    KEY idx_audit_logs_created (created_at),
    KEY idx_audit_logs_actor_created (actor_id, created_at),
    KEY idx_audit_logs_resource (resource_type, resource_public_id),

    CONSTRAINT fk_audit_logs_actor
        FOREIGN KEY (actor_id) REFERENCES users(id)
        ON DELETE SET NULL ON UPDATE RESTRICT,

    CONSTRAINT chk_audit_logs_result
        CHECK (result IN ('success', 'failure'))
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;
```

### SQL 实施说明

1. `public_id` 必须由应用使用密码学安全随机数生成，不能依赖连续自增值。
2. `users.email` 的大小写和唯一性策略必须在注册或初始化逻辑中统一处理。
3. 不允许将完整 HTML 正文无限制写入数据库。
4. `config_snapshot` 不得包含密码、Cookie、会话令牌等敏感数据。
5. 数据库 CHECK 约束不能替代 Go 业务层校验。
6. 不要使用 `INSERT IGNORE` 隐藏初始化错误。
7. 迁移执行前必须备份已有数据，生产环境不允许直接重建数据库。

## 2.2 回滚迁移

文件：`backend/migrations/000001_init_schema.down.sql`

```sql
SET time_zone = '+00:00';

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS scan_reports;
DROP TABLE IF EXISTS issue_events;
DROP TABLE IF EXISTS scan_issues;
DROP TABLE IF EXISTS page_snapshots;
DROP TABLE IF EXISTS scan_tasks;
DROP TABLE IF EXISTS websites;
DROP TABLE IF EXISTS users;
```

**重要：** 该回滚脚本会删除所有 GeoPilot 数据，只能用于开发环境重置或经过明确审批的恢复操作。生产环境禁止把回滚脚本当作常规修复手段。

## 2.3 初始化管理员

不要把默认管理员密码写入迁移文件，也不要在 SQL 中创建公开已知的固定密码。

推荐提供一次性初始化命令：

```bash
./geopilot admin create \
  --email admin@example.com \
  --display-name "GeoPilot Admin"
```

命令应通过安全交互获取密码，计算密码哈希后写入 `users`。如果管理员已经存在，命令应默认拒绝覆盖。

---

# 第三部分：OpenAPI 3.1.0 完整定义

文件：`backend/openapi/openapi-v1.0.yaml`

以下定义覆盖 V1.0 的认证、网站管理、扫描任务、问题整改、报告和健康检查接口。

```yaml
openapi: 3.1.0

info:
  title: GeoPilot API
  version: 1.0.0
  description: >
    GeoPilot is a lightweight GEO website diagnostic service.
    The API provides website management, safe website scanning,
    issue tracking, and report generation.
  contact:
    name: GeoPilot API
  license:
    name: Proprietary

servers:
  - url: http://localhost:8080/api/v1
    description: Local development
  - url: https://geopilot.example.com/api/v1
    description: Example production endpoint

tags:
  - name: Health
  - name: Authentication
  - name: Websites
  - name: Scans
  - name: Issues
  - name: Reports

security:
  - SessionCookie: []

paths:

  /health:
    get:
      tags: [Health]
      operationId: getHealth
      summary: Health check
      security: []
      responses:
        "200":
          description: Service is healthy
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/HealthResponse"

  /auth/login:
    post:
      tags: [Authentication]
      operationId: login
      summary: Log in
      description: >
        Authenticates an account and establishes a server-side session.
        Apply rate limiting and CSRF-safe cookie handling.
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/LoginRequest"
      responses:
        "200":
          description: Login succeeded
          headers:
            Set-Cookie:
              description: Secure session cookie
              schema:
                type: string
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeMe"
        "400":
          $ref: "#/components/responses/BadRequest"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "429":
          $ref: "#/components/responses/RateLimited"

  /auth/logout:
    post:
      tags: [Authentication]
      operationId: logout
      summary: Log out
      responses:
        "204":
          description: Session invalidated
        "401":
          $ref: "#/components/responses/Unauthorized"

  /auth/me:
    get:
      tags: [Authentication]
      operationId: getCurrentUser
      summary: Get current user
      responses:
        "200":
          description: Current user
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeMe"
        "401":
          $ref: "#/components/responses/Unauthorized"

  /auth/change-password:
    post:
      tags: [Authentication]
      operationId: changePassword
      summary: Change current user's password
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/ChangePasswordRequest"
      responses:
        "204":
          description: Password changed; existing sessions may be revoked
        "400":
          $ref: "#/components/responses/BadRequest"
        "401":
          $ref: "#/components/responses/Unauthorized"

  /websites:
    get:
      tags: [Websites]
      operationId: listWebsites
      summary: List websites
      parameters:
        - $ref: "#/components/parameters/Page"
        - $ref: "#/components/parameters/PageSize"
        - name: status
          in: query
          schema:
            $ref: "#/components/schemas/WebsiteStatus"
        - name: keyword
          in: query
          schema:
            type: string
            maxLength: 120
      responses:
        "200":
          description: Website page
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeWebsitePage"
        "401":
          $ref: "#/components/responses/Unauthorized"

    post:
      tags: [Websites]
      operationId: createWebsite
      summary: Create a website
      description: >
        The server must validate the URL and enforce SSRF defenses.
        Creating a website does not guarantee that the target can be scanned.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateWebsiteRequest"
      responses:
        "201":
          description: Website created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeWebsite"
        "400":
          $ref: "#/components/responses/BadRequest"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "409":
          $ref: "#/components/responses/Conflict"
        "422":
          $ref: "#/components/responses/TargetBlocked"

  /websites/{websiteId}:
    parameters:
      - $ref: "#/components/parameters/WebsiteId"

    get:
      tags: [Websites]
      operationId: getWebsite
      summary: Get website details
      responses:
        "200":
          description: Website details
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeWebsite"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"

    patch:
      tags: [Websites]
      operationId: updateWebsite
      summary: Update website
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdateWebsiteRequest"
      responses:
        "200":
          description: Website updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeWebsite"
        "400":
          $ref: "#/components/responses/BadRequest"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"
        "422":
          $ref: "#/components/responses/TargetBlocked"

    delete:
      tags: [Websites]
      operationId: deleteWebsite
      summary: Soft-delete website
      responses:
        "204":
          description: Website soft-deleted
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"

  /websites/{websiteId}/scans:
    parameters:
      - $ref: "#/components/parameters/WebsiteId"

    post:
      tags: [Scans]
      operationId: createScan
      summary: Create scan task
      description: >
        Creates an asynchronous scan. A successful response means that
        the task was accepted, not that the remote site was successfully scanned.
      parameters:
        - $ref: "#/components/parameters/IdempotencyKey"
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateScanRequest"
      responses:
        "202":
          description: Scan accepted
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeScanAccepted"
        "400":
          $ref: "#/components/responses/BadRequest"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"
        "409":
          $ref: "#/components/responses/Conflict"
        "422":
          $ref: "#/components/responses/TargetBlocked"
        "429":
          $ref: "#/components/responses/RateLimited"

  /scans:
    get:
      tags: [Scans]
      operationId: listScans
      summary: List scan tasks
      parameters:
        - $ref: "#/components/parameters/Page"
        - $ref: "#/components/parameters/PageSize"
        - name: website_id
          in: query
          schema:
            $ref: "#/components/schemas/Identifier"
        - name: status
          in: query
          schema:
            $ref: "#/components/schemas/ScanStatus"
        - name: from
          in: query
          description: Inclusive UTC timestamp
          schema:
            type: string
            format: date-time
        - name: to
          in: query
          description: Exclusive UTC timestamp
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: Scan page
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeScanPage"
        "400":
          $ref: "#/components/responses/BadRequest"
        "401":
          $ref: "#/components/responses/Unauthorized"

  /scans/{scanId}:
    parameters:
      - $ref: "#/components/parameters/ScanId"

    get:
      tags: [Scans]
      operationId: getScan
      summary: Get scan task
      responses:
        "200":
          description: Scan details
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeScan"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"

  /scans/{scanId}/cancel:
    parameters:
      - $ref: "#/components/parameters/ScanId"

    post:
      tags: [Scans]
      operationId: cancelScan
      summary: Cancel scan
      responses:
        "204":
          description: Cancellation accepted or task cancelled
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"
        "409":
          $ref: "#/components/responses/Conflict"

  /scans/{scanId}/retry:
    parameters:
      - $ref: "#/components/parameters/ScanId"

    post:
      tags: [Scans]
      operationId: retryScan
      summary: Create retry task
      parameters:
        - $ref: "#/components/parameters/IdempotencyKey"
      responses:
        "202":
          description: Retry task accepted
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeScanAccepted"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"
        "409":
          $ref: "#/components/responses/Conflict"

  /scans/{scanId}/issues:
    parameters:
      - $ref: "#/components/parameters/ScanId"

    get:
      tags: [Issues]
      operationId: listScanIssues
      summary: List issues for a scan
      parameters:
        - $ref: "#/components/parameters/Page"
        - $ref: "#/components/parameters/PageSize"
        - name: severity
          in: query
          schema:
            $ref: "#/components/schemas/IssueSeverity"
        - name: rule_id
          in: query
          schema:
            type: string
            maxLength: 64
        - name: issue_status
          in: query
          schema:
            $ref: "#/components/schemas/IssueStatus"
      responses:
        "200":
          description: Issue page
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeIssuePage"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"

  /issues/{issueId}:
    parameters:
      - $ref: "#/components/parameters/IssueId"

    patch:
      tags: [Issues]
      operationId: updateIssue
      summary: Update issue status
      description: >
        The status change and its issue event must be committed atomically.
        Marking an issue resolved does not itself verify the fix.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdateIssueRequest"
      responses:
        "200":
          description: Issue updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeIssue"
        "400":
          $ref: "#/components/responses/BadRequest"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"
        "409":
          $ref: "#/components/responses/Conflict"

  /scans/{scanId}/reports:
    parameters:
      - $ref: "#/components/parameters/ScanId"

    get:
      tags: [Reports]
      operationId: listScanReports
      summary: List reports for a scan
      responses:
        "200":
          description: Report list
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeReportPage"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"

    post:
      tags: [Reports]
      operationId: createScanReport
      summary: Generate report
      parameters:
        - $ref: "#/components/parameters/IdempotencyKey"
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateReportRequest"
      responses:
        "202":
          description: Report generation accepted
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeReport"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"
        "409":
          $ref: "#/components/responses/Conflict"

  /reports/{reportId}:
    parameters:
      - $ref: "#/components/parameters/ReportId"

    get:
      tags: [Reports]
      operationId: getReport
      summary: Get report metadata
      responses:
        "200":
          description: Report metadata
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnvelopeReport"
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"

  /reports/{reportId}/download:
    parameters:
      - $ref: "#/components/parameters/ReportId"

    get:
      tags: [Reports]
      operationId: downloadReport
      summary: Download generated report
      responses:
        "200":
          description: Report file
          headers:
            Content-Disposition:
              schema:
                type: string
          content:
            text/html:
              schema:
                type: string
              example: "<!doctype html><html><body>GeoPilot report</body></html>"
            application/json:
              schema:
                type: object
        "401":
          $ref: "#/components/responses/Unauthorized"
        "404":
          $ref: "#/components/responses/NotFound"
        "409":
          $ref: "#/components/responses/ReportNotReady"

components:

  securitySchemes:
    SessionCookie:
      type: apiKey
      in: cookie
      name: geopilot_session
      description: >
        Opaque server-side session cookie. In production use Secure,
        HttpOnly, an appropriate SameSite policy, expiration, session rotation,
        and CSRF protection for state-changing requests.

  parameters:
    Page:
      name: page
      in: query
      schema:
        type: integer
        minimum: 1
        default: 1

    PageSize:
      name: page_size
      in: query
      schema:
        type: integer
        minimum: 1
        maximum: 100
        default: 20

    WebsiteId:
      name: websiteId
      in: path
      required: true
      schema:
        $ref: "#/components/schemas/Identifier"

    ScanId:
      name: scanId
      in: path
      required: true
      schema:
        type: string
        maxLength: 40

    IssueId:
      name: issueId
      in: path
      required: true
      schema:
        type: string
        maxLength: 40

    ReportId:
      name: reportId
      in: path
      required: true
      schema:
        type: string
        maxLength: 40

    IdempotencyKey:
      name: Idempotency-Key
      in: header
      required: false
      description: >
        Optional idempotency key for creation operations.
        Reuse with a different request body must return a conflict.
      schema:
        type: string
        minLength: 8
        maxLength: 128

  schemas:

    Identifier:
      type: string
      pattern: '^[1-9][0-9]*$'
      examples: ["1001"]

    Timestamp:
      type: string
      format: date-time
      examples: ["2026-11-02T08:00:00Z"]

    WebsiteStatus:
      type: string
      enum: [active, disabled]

    ScanStatus:
      type: string
      enum:
        - queued
        - running
        - succeeded
        - partial
        - failed
        - cancelled

    IssueSeverity:
      type: string
      enum: [critical, high, medium, low]

    IssueStatus:
      type: string
      enum: [open, resolved, ignored]

    ReportFormat:
      type: string
      enum: [html, json]

    ReportStatus:
      type: string
      enum: [generating, ready, failed]

    HealthResponse:
      type: object
      required: [status]
      properties:
        status:
          type: string
          enum: [ok]

    ErrorDetail:
      type: object
      required: [field, reason]
      properties:
        field:
          type: string
        reason:
          type: string

    ErrorResponse:
      type: object
      required: [code, message, request_id]
      properties:
        code:
          type: string
        message:
          type: string
        details:
          type: array
          items:
            $ref: "#/components/schemas/ErrorDetail"
        request_id:
          type: string

    LoginRequest:
      type: object
      additionalProperties: false
      required: [email, password]
      properties:
        email:
          type: string
          format: email
          maxLength: 254
        password:
          type: string
          minLength: 1
          maxLength: 1024

    ChangePasswordRequest:
      type: object
      additionalProperties: false
      required: [current_password, new_password]
      properties:
        current_password:
          type: string
          minLength: 1
          maxLength: 1024
        new_password:
          type: string
          minLength: 12
          maxLength: 1024

    User:
      type: object
      required: [id, email, display_name, role, status, created_at]
      properties:
        id:
          $ref: "#/components/schemas/Identifier"
        email:
          type: string
          format: email
        display_name:
          type: string
        role:
          type: string
          enum: [admin, viewer]
        status:
          type: string
          enum: [active, disabled]
        created_at:
          $ref: "#/components/schemas/Timestamp"

    Website:
      type: object
      required:
        - id
        - name
        - base_url
        - status
        - created_at
        - updated_at
      properties:
        id:
          $ref: "#/components/schemas/Identifier"
        name:
          type: string
          maxLength: 120
        base_url:
          type: string
          format: uri
          maxLength: 2048
        description:
          type: [string, "null"]
          maxLength: 500
        status:
          $ref: "#/components/schemas/WebsiteStatus"
        created_at:
          $ref: "#/components/schemas/Timestamp"
        updated_at:
          $ref: "#/components/schemas/Timestamp"

    CreateWebsiteRequest:
      type: object
      additionalProperties: false
      required: [name, base_url]
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 120
        base_url:
          type: string
          format: uri
          maxLength: 2048
        description:
          type: [string, "null"]
          maxLength: 500

    UpdateWebsiteRequest:
      type: object
      additionalProperties: false
      minProperties: 1
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 120
        base_url:
          type: string
          format: uri
          maxLength: 2048
        description:
          type: [string, "null"]
          maxLength: 500
        status:
          $ref: "#/components/schemas/WebsiteStatus"

    PageInfo:
      type: object
      required: [page, page_size, total]
      properties:
        page:
          type: integer
          minimum: 1
        page_size:
          type: integer
          minimum: 1
          maximum: 100
        total:
          type: integer
          minimum: 0

    WebsitePage:
      type: object
      required: [items, page, page_size, total]
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/Website"
        page:
          type: integer
        page_size:
          type: integer
        total:
          type: integer

    CreateScanRequest:
      type: object
      additionalProperties: false
      properties:
        scope:
          type: string
          enum: [homepage]
          default: homepage

    IssueSummary:
      type: object
      required: [total, critical, high, medium, low]
      properties:
        total:
          type: integer
          minimum: 0
        critical:
          type: integer
          minimum: 0
        high:
          type: integer
          minimum: 0
        medium:
          type: integer
          minimum: 0
        low:
          type: integer
          minimum: 0

    ScanAccepted:
      type: object
      required: [scan_id, status, created_at]
      properties:
        scan_id:
          type: string
          maxLength: 40
        status:
          type: string
          const: queued
        created_at:
          $ref: "#/components/schemas/Timestamp"

    ScanTask:
      type: object
      required:
        - scan_id
        - website_id
        - status
        - issue_summary
        - ruleset_version
        - scoring_version
        - created_at
      properties:
        scan_id:
          type: string
          maxLength: 40
        website_id:
          $ref: "#/components/schemas/Identifier"
        status:
          $ref: "#/components/schemas/ScanStatus"
        scope:
          type: string
          const: homepage
        requested_url:
          type: string
          maxLength: 2048
        final_url:
          type: [string, "null"]
          maxLength: 2048
        score:
          type: [integer, "null"]
          minimum: 0
          maximum: 100
        issue_summary:
          $ref: "#/components/schemas/IssueSummary"
        ruleset_version:
          type: string
        scoring_version:
          type: string
        error_code:
          type: [string, "null"]
        error_summary:
          type: [string, "null"]
          maxLength: 500
        queued_at:
          $ref: "#/components/schemas/Timestamp"
        started_at:
          type: [string, "null"]
          format: date-time
        finished_at:
          type: [string, "null"]
          format: date-time
        created_at:
          $ref: "#/components/schemas/Timestamp"

    ScanPage:
      type: object
      required: [items, page, page_size, total]
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/ScanTask"
        page:
          type: integer
        page_size:
          type: integer
        total:
          type: integer

    Issue:
      type: object
      required:
        - issue_id
        - scan_id
        - rule_id
        - severity
        - title
        - description
        - suggestion
        - deduction
        - issue_status
        - created_at
      properties:
        issue_id:
          type: string
          maxLength: 40
        scan_id:
          type: string
          maxLength: 40
        rule_id:
          type: string
          maxLength: 64
        category:
          type: string
        severity:
          $ref: "#/components/schemas/IssueSeverity"
        title:
          type: string
          maxLength: 255
        description:
          type: string
        evidence:
          type: [object, "null"]
          additionalProperties: true
        suggestion:
          type: string
        deduction:
          type: integer
          minimum: 0
          maximum: 100
        fingerprint:
          type: string
          pattern: '^[a-fA-F0-9]{64}$'
        issue_status:
          $ref: "#/components/schemas/IssueStatus"
        status_note:
          type: [string, "null"]
          maxLength: 500
        resolved_at:
          type: [string, "null"]
          format: date-time
        created_at:
          $ref: "#/components/schemas/Timestamp"
        updated_at:
          $ref: "#/components/schemas/Timestamp"

    IssuePage:
      type: object
      required: [items, page, page_size, total]
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/Issue"
        page:
          type: integer
        page_size:
          type: integer
        total:
          type: integer

    UpdateIssueRequest:
      type: object
      additionalProperties: false
      minProperties: 1
      properties:
        issue_status:
          $ref: "#/components/schemas/IssueStatus"
        status_note:
          type: [string, "null"]
          maxLength: 500

    CreateReportRequest:
      type: object
      additionalProperties: false
      required: [format]
      properties:
        format:
          $ref: "#/components/schemas/ReportFormat"

    Report:
      type: object
      required:
        - report_id
        - scan_id
        - format
        - status
        - created_at
      properties:
        report_id:
          type: string
          maxLength: 40
        scan_id:
          type: string
          maxLength: 40
        format:
          $ref: "#/components/schemas/ReportFormat"
        status:
          $ref: "#/components/schemas/ReportStatus"
        file_size_bytes:
          type: [integer, "null"]
          minimum: 0
        checksum:
          type: [string, "null"]
          pattern: '^[a-fA-F0-9]{64}$'
        error_summary:
          type: [string, "null"]
          maxLength: 500
        created_at:
          $ref: "#/components/schemas/Timestamp"

    ReportPage:
      type: object
      required: [items, page, page_size, total]
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/Report"
        page:
          type: integer
        page_size:
          type: integer
        total:
          type: integer

    EnvelopeMe:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          type: object
          required: [user]
          properties:
            user:
              $ref: "#/components/schemas/User"
        request_id:
          type: string

    EnvelopeWebsite:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/Website"
        request_id:
          type: string

    EnvelopeWebsitePage:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/WebsitePage"
        request_id:
          type: string

    EnvelopeScanAccepted:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/ScanAccepted"
        request_id:
          type: string

    EnvelopeScan:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/ScanTask"
        request_id:
          type: string

    EnvelopeScanPage:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/ScanPage"
        request_id:
          type: string

    EnvelopeIssue:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/Issue"
        request_id:
          type: string

    EnvelopeIssuePage:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/IssuePage"
        request_id:
          type: string

    EnvelopeReport:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/Report"
        request_id:
          type: string

    EnvelopeReportPage:
      type: object
      required: [code, message, data, request_id]
      properties:
        code:
          type: string
          const: OK
        message:
          type: string
        data:
          $ref: "#/components/schemas/ReportPage"
        request_id:
          type: string

  responses:
    BadRequest:
      description: Invalid request
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"

    Unauthorized:
      description: Authentication required
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"

    Forbidden:
      description: Permission denied
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"

    NotFound:
      description: Resource not found or not visible to the current user
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"

    Conflict:
      description: Resource conflict or invalid state transition
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"

    TargetBlocked:
      description: Target rejected by network security policy
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"

    RateLimited:
      description: Request rate or scan quota exceeded
      headers:
        Retry-After:
          schema:
            type: integer
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"

    ReportNotReady:
      description: Report is not ready
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ErrorResponse"
```

---

# 第四部分：接口行为约束

## 4.1 资源授权

以下规则必须落实在后端：

- 用户只能访问自己有权管理的网站。
- 查询扫描任务时必须通过任务关联的网站验证权限。
- 查询问题时必须通过问题关联的任务和网站验证权限。
- 下载报告时必须验证报告关联任务的网站权限。
- 不得将“外部 ID 难以猜测”当作授权机制。

如果后续引入多租户，必须为所有资源查询增加租户隔离，不能只依赖前端隐藏按钮。

## 4.2 扫描任务执行

扫描任务使用以下状态：

```text
queued -> running
queued -> cancelled

running -> succeeded
running -> partial
running -> failed
running -> cancelled
```

终态任务不得通过普通 API 修改为 `running`。

扫描失败属于任务结果，不应把已经成功接受的扫描请求改写为 HTTP 504。任务创建成功后，应通过任务查询接口获取最终结果。

## 4.3 评分规则

V1.0 使用内部启发式评分：

| 严重级别 | 单项扣分 |
|---|---:|
| critical | 30 |
| high | 18 |
| medium | 10 |
| low | 4 |

计算方式：

```text
score = max(0, min(100, 100 - total_deduction))
```

同一规则在同一页面的一次扫描中默认只扣分一次。

页面无法抓取时，不得因为缺少标题、H1、描述等信息重复扣分。没有有效扫描结果时，`score` 应为 `null`。

该评分不是搜索引擎或 AI 平台的官方评分，不得将其描述为 AI 引用概率或排名预测。

## 4.4 问题状态

允许状态：

```text
open
resolved
ignored
```

修改问题状态时必须：

1. 验证问题访问权限。
2. 验证目标状态。
3. 开启数据库事务。
4. 更新问题状态。
5. 插入 `issue_events`。
6. 提交事务。

如果状态没有变化，建议返回当前问题并避免创建重复事件。

`resolved` 表示用户确认已修复，不等于系统已重新验证。

## 4.5 报告生成

报告状态：

```text
generating
ready
failed
```

报告生成过程必须：

- 对外部文本进行正确的 HTML 转义。
- 禁止将原始网站 HTML 直接拼入报告。
- 限制报告文件大小。
- 防止路径穿越。
- 不向客户端返回服务器绝对路径。
- 下载前重新校验访问权限。

---

# 第五部分：错误码

| 错误码 | HTTP 状态 | 说明 |
|---|---:|---|
| `VALIDATION_ERROR` | 400 | 参数校验失败 |
| `UNAUTHORIZED` | 401 | 未认证或会话失效 |
| `FORBIDDEN` | 403 | 已认证但无权限 |
| `RESOURCE_NOT_FOUND` | 404 | 资源不存在或不可见 |
| `DUPLICATE_RESOURCE` | 409 | 资源重复 |
| `INVALID_STATE` | 409 | 状态转换不合法 |
| `SCAN_NOT_RETRYABLE` | 409 | 当前任务不可重试 |
| `REPORT_NOT_READY` | 409 | 报告尚未生成 |
| `TARGET_BLOCKED` | 422 | 目标被安全策略拒绝 |
| `SCAN_LIMIT_REACHED` | 429 | 扫描并发或配额达到限制 |
| `RATE_LIMITED` | 429 | 请求频率过高 |
| `INTERNAL_ERROR` | 500 | 内部异常 |
| `SERVICE_UNAVAILABLE` | 503 | 服务暂不可用 |

错误响应示例：

```json
{
  "code": "TARGET_BLOCKED",
  "message": "The target is not allowed by the network security policy.",
  "details": [
    {
      "field": "base_url",
      "reason": "target_not_allowed"
    }
  ],
  "request_id": "req_example"
}
```

不得将数据库连接字符串、内部堆栈、服务器文件路径或未经脱敏的内部 IP 暴露给客户端。

---

# 第六部分：迁移与契约验收

## 6.1 数据库迁移验收

- [ ] 空 MySQL 8.0.16+ 数据库可以执行初始化迁移。
- [ ] 所有表、主键、唯一键、普通索引和外键均创建成功。
- [ ] CHECK 约束可以正常阻止非法枚举值。
- [ ] 初始化管理员命令不使用默认固定密码。
- [ ] 外键不会意外删除历史扫描记录。
- [ ] 数据库连接统一使用 UTC。
- [ ] 数据库密码不提交到 Git。
- [ ] 回滚脚本已在隔离环境验证。

## 6.2 OpenAPI 验收

- [ ] OpenAPI 文件可由兼容 OpenAPI 3.1 的工具解析。
- [ ] 所有 `$ref` 均能解析。
- [ ] 所有 path parameter 均声明为必填。
- [ ] 所有请求和响应 Schema 均与 Go DTO 对齐。
- [ ] 所有 API 错误响应符合统一结构。
- [ ] BIGINT ID 在 JSON 中使用字符串。
- [ ] 前端 TypeScript 类型与 API 响应一致。
- [ ] 登录接口使用限流。
- [ ] 受保护接口实施资源授权。
- [ ] 扫描接口执行 SSRF 安全校验。

## 6.3 端到端验收

最小验收流程：

1. 初始化数据库。
2. 创建管理员。
3. 登录。
4. 创建网站。
5. 触发扫描。
6. 查询扫描状态。
7. 获取问题列表。
8. 修改问题整改状态。
9. 查询问题事件。
10. 生成 HTML 报告。
11. 下载报告。
12. 验证未授权访问被拒绝。
13. 验证内网目标和重定向绕过被拒绝。
14. 验证服务重启后遗留任务可被识别和处理。

---

# 第七部分：实施注意事项与待冻结项

## 7.1 实施注意事项

1. 本规范定义的是接口契约和迁移基线，不表示代码已经实现。
2. 正式提交代码前，应使用实际 OpenAPI 3.1 校验工具验证 YAML。
3. SQL 应在隔离数据库中执行并验证约束。
4. Go DTO、数据库字段和 TypeScript 类型必须保持一致。
5. 对于 Cookie 认证，必须完成 CSRF 防护。
6. 对于公网扫描服务，必须完成 SSRF 防护、网络出口限制、资源授权和限流。
7. 生产环境迁移必须备份并制定恢复方案。
8. 不得为了通过测试而关闭安全约束。

## 7.2 待冻结事项

| 事项 | 当前默认决定 | 影响 |
|---|---|---|
| 部署方式 | 自托管优先 | 影响认证和网络边界 |
| 用户体系 | 单用户管理员优先，预留 viewer | 影响权限实现 |
| 扫描范围 | 仅首页 | 影响任务和快照模型 |
| 问题状态 | 支持整改状态与事件历史 | 影响问题管理模块 |
| 报告格式 | HTML + JSON | 影响报告接口与存储 |
| 历史问题身份 | 以 fingerprint 关联 | 未来可能升级独立问题实例 |
| 任务调度 | Go 进程内执行器 | 后续可升级持久化队列 |
| 数据保留 | 暂不自动清理历史数据 | 正式部署前需制定保留策略 |

---

# 文档结论

本规范为 GeoPilot V1.0 建立了数据库迁移与 REST API 的统一基线，涵盖核心实体、扫描生命周期、问题整改、报告管理、错误处理、认证和安全约束。

下一阶段建议按以下顺序推进：

1. 将 SQL 保存为真实迁移文件并在隔离数据库验证。
2. 将 OpenAPI YAML 保存为独立文件并执行规范校验。
3. 根据契约生成或校验前端 TypeScript 类型。
4. 编写 Go API DTO、路由、业务服务与数据库仓储层。
5. 建立集成测试和安全测试。
6. 完成《GeoPilot V1.0 开发任务拆分与验收测试用例》。

**冻结条件：** SQL 通过迁移测试，OpenAPI 通过解析校验，核心接口与状态机完成评审，SSRF 与资源授权测试方案明确后，方可将本文升级为正式冻结版本。
