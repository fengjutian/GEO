下面继续编写 《GeoPilot 功能实现与技术设计文档 V1.0》。本文承接前面的《GeoPilot 产品需求规格说明书 V1.0》，将产品需求进一步落到可开发的模块、接口、数据模型、扫描流程、诊断规则、安全策略和测试要求上。

本文是工程实施基线，不代表这些功能已经开发完成。需求中尚未冻结的事项会标注为待确认，并给出首版建议。

## GeoPilot 功能实现与技术设计文档 V1.0

V1.0

工程设计

待评审

项目：GeoPilot · 轻量级 GEO 网站诊断工具

技术栈： Go + TypeScript + React + MySQL

前端： Vite + Tailwind CSS + shadcn/ui 风格组件 + Lucide React

后端： Go 单体应用 + REST API + 扫描引擎

核心目标： 形成从网站配置、页面扫描、问题诊断到整改报告的完整闭环。

# 一、总体技术架构

## 1.1 架构设计原则

GeoPilot V1.0 采用模块化单体架构，不提前引入复杂的分布式系统。

1. 前后端分离，前端通过 REST API 访问后端。
2. 扫描逻辑与 HTTP 接口解耦，方便增加诊断规则。
3. 每次扫描生成独立快照，历史数据不被后续扫描覆盖。
4. 规则采用统一接口，新增规则时尽量不修改扫描主流程。
5. 页面抓取与 HTML 分析分离，便于单独测试。
6. 所有用户可控输入均进行校验，尤其是 URL 和网络请求目标。
7. 优先保证扫描安全、结果可解释、问题可复现。

## 1.2 系统架构图

### React 前端应用

网站管理 · 扫描工作台 · 问题中心 · 报告

Vite + TypeScript + Tailwind CSS + Lucide React

HTTPS / JSON / REST API

### Go 后端应用

安全与权限

认证、授权、限流、URL 校验

网站与任务

网站配置、扫描生命周期

扫描引擎

HTTP 抓取、HTML 解析

诊断与报告

规则执行、评分、历史比较

MySQL 8

网站、任务、页面快照、问题、报告元数据

报告存储

HTML 报告文件及其元数据

### 架构边界

V1.0 暂不引入 Redis、Kafka、Kubernetes、独立微服务或独立搜索集群。

扫描任务可以先由 Go 进程内的受控任务执行器处理，并将任务状态持久化到 MySQL。后续如需要多实例部署、任务可靠重试或高并发扫描，再考虑将任务队列独立出来。

需要注意：进程内执行器不是持久化队列。进程异常退出时，正在执行的任务可能中断，因此必须提供任务恢复或失败标记机制。

# 二、工程目录结构

推荐采用前后端分目录的单仓库结构。

```
geopilot/
├── README.md
├── LICENSE
├── .gitignore
├── .env.example
├── docker-compose.yml
│
├── docs/
│   ├── requirements-v1.0.md
│   ├── technical-design-v1.0.md
│   ├── api-spec-v1.0.md
│   ├── database-design-v1.0.md
│   ├── acceptance-v1.0.md
│   └── deployment-v1.0.md
│
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── public/
│   └── src/
│       ├── app/
│       ├── components/
│       │   ├── ui/
│       │   ├── layout/
│       │   └── feedback/
│       ├── features/
│       │   ├── dashboard/
│       │   ├── websites/
│       │   ├── scans/
│       │   ├── issues/
│       │   ├── reports/
│       │   └── settings/
│       ├── lib/
│       ├── services/
│       ├── types/
│       ├── hooks/
│       └── main.tsx
│
├── backend/
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   └── geopilot/
│   │       └── main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── server/
│   │   ├── middleware/
│   │   ├── auth/
│   │   ├── website/
│   │   ├── scan/
│   │   ├── fetcher/
│   │   ├── analyzer/
│   │   ├── rules/
│   │   ├── scoring/
│   │   ├── issue/
│   │   ├── report/
│   │   ├── repository/
│   │   └── recovery/
│   ├── migrations/
│   └── tests/
│
├── deployments/
│   ├── Dockerfile.frontend
│   ├── Dockerfile.backend
│   └── nginx.conf
│
└── scripts/
    ├── dev.sh
    ├── test.sh
    └── migrate.sh
```

## 2.1 模块职责

| 模块           | 主要职责                  | 不应承担的职责      |
| ------------ | --------------------- | ------------ |
| `website`    | 网站增删改查、配置校验           | 实际网络抓取       |
| `scan`       | 创建任务、状态流转、执行调度        | 直接编写 HTML 规则 |
| `fetcher`    | HTTP 请求、重定向、安全检查、响应限制 | 计算 GEO 评分    |
| `analyzer`   | HTML 解析、抽取页面特征        | 修改任务状态       |
| `rules`      | 执行诊断规则并输出问题           | 直接操作 HTTP 请求 |
| `scoring`    | 汇总问题、计算分数             | 判断原始 HTML 结构 |
| `issue`      | 查询问题、更新整改状态           | 重新抓取网站       |
| `report`     | 生成 HTML 报告、提供下载       | 承担扫描调度       |
| `repository` | 数据持久化接口与实现            | 业务规则决策       |
| `recovery`   | 检查并处理遗留任务             | 自动重跑所有失败任务   |

# 三、核心业务流程实现

## 3.1 主流程

\#chatgpt-mermaid-\_r_33c\_{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;fill:rgb(13, 13, 13);}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#chatgpt-mermaid-\_r_33c\_ .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_33c\_ .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_33c\_ .error-icon{fill:rgb(243, 243, 243);}#chatgpt-mermaid-\_r_33c\_ .error-text{fill:rgb(13, 13, 13);stroke:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ .edge-thickness-normal{stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ .edge-thickness-thick{stroke-width:3.5px;}#chatgpt-mermaid-\_r_33c\_ .edge-pattern-solid{stroke-dasharray:0;}#chatgpt-mermaid-\_r_33c\_ .edge-thickness-invisible{stroke-width:0;fill:none;}#chatgpt-mermaid-\_r_33c\_ .edge-pattern-dashed{stroke-dasharray:3;}#chatgpt-mermaid-\_r_33c\_ .edge-pattern-dotted{stroke-dasharray:2;}#chatgpt-mermaid-\_r_33c\_ .marker{fill:rgb(143, 143, 143);stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_33c\_ .marker.cross{stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_33c\_ svg{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;}#chatgpt-mermaid-\_r_33c\_ p{margin:0;}#chatgpt-mermaid-\_r_33c\_ .label{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";color:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ .cluster-label text{fill:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ .cluster-label span{color:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ .cluster-label span p{background-color:transparent;}#chatgpt-mermaid-\_r_33c\_ .label text,#chatgpt-mermaid-\_r_33c\_ span{fill:rgb(13, 13, 13);color:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ .node rect,#chatgpt-mermaid-\_r_33c\_ .node circle,#chatgpt-mermaid-\_r_33c\_ .node ellipse,#chatgpt-mermaid-\_r_33c\_ .node polygon,#chatgpt-mermaid-\_r_33c\_ .node path{fill:rgb(222, 234, 251);stroke:rgb(83, 154, 248);stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ .rough-node .label text,#chatgpt-mermaid-\_r_33c\_ .node .label text,#chatgpt-mermaid-\_r_33c\_ .image-shape .label,#chatgpt-mermaid-\_r_33c\_ .icon-shape .label{text-anchor:middle;}#chatgpt-mermaid-\_r_33c\_ .node .katex path{fill:#000;stroke:#000;stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ .rough-node .label,#chatgpt-mermaid-\_r_33c\_ .node .label,#chatgpt-mermaid-\_r_33c\_ .image-shape .label,#chatgpt-mermaid-\_r_33c\_ .icon-shape .label{text-align:center;}#chatgpt-mermaid-\_r_33c\_ .node.clickable{cursor:pointer;}#chatgpt-mermaid-\_r_33c\_ .root .anchor path{fill:rgb(143, 143, 143)!important;stroke-width:0;stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_33c\_ .arrowheadPath{fill:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_33c\_ .edgePath .path{stroke:rgb(143, 143, 143);stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ .flowchart-link{stroke:rgb(143, 143, 143);fill:none;}#chatgpt-mermaid-\_r_33c\_ .edgeLabel{background-color:rgb(252, 252, 252);text-align:center;}#chatgpt-mermaid-\_r_33c\_ .edgeLabel p{background-color:rgb(252, 252, 252);}#chatgpt-mermaid-\_r_33c\_ .edgeLabel rect{opacity:0.5;background-color:rgb(252, 252, 252);fill:rgb(252, 252, 252);}#chatgpt-mermaid-\_r_33c\_ .labelBkg{background-color:rgba(252, 252, 252, 0.5);}#chatgpt-mermaid-\_r_33c\_ .cluster rect{fill:rgb(243, 243, 243);stroke:rgba(0, 0, 0, 0.1);stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ .cluster text{fill:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ .cluster span{color:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ div.mermaidTooltip{position:absolute;text-align:center;max-width:200px;padding:2px;font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:12px;background:rgb(243, 243, 243);border:1px solid rgba(0, 0, 0, 0.1);border-radius:2px;pointer-events:none;z-index:100;}#chatgpt-mermaid-\_r_33c\_ .flowchartTitleText{text-anchor:middle;font-size:18px;fill:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_33c\_ rect.text{fill:none;stroke-width:0;}#chatgpt-mermaid-\_r_33c\_ .icon-shape,#chatgpt-mermaid-\_r_33c\_ .image-shape{background-color:rgb(252, 252, 252);text-align:center;}#chatgpt-mermaid-\_r_33c\_ .icon-shape p,#chatgpt-mermaid-\_r_33c\_ .image-shape p{background-color:rgb(252, 252, 252);padding:2px;}#chatgpt-mermaid-\_r_33c\_ .icon-shape .label rect,#chatgpt-mermaid-\_r_33c\_ .image-shape .label rect{opacity:0.5;background-color:rgb(252, 252, 252);fill:rgb(252, 252, 252);}#chatgpt-mermaid-\_r_33c\_ .label-icon{display:inline-block;height:1em;overflow:visible;vertical-align:-0.125em;}#chatgpt-mermaid-\_r_33c\_ .node .label-icon path{fill:currentColor;stroke:revert;stroke-width:revert;}#chatgpt-mermaid-\_r_33c\_ .node .neo-node{stroke:rgb(83, 154, 248);}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].node rect,#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].cluster rect,#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].node polygon{stroke:url(#chatgpt-mermaid-\_r_33c\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].swimlane.cluster rect{filter:none;}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].node path{stroke:url(#chatgpt-mermaid-\_r_33c\_-gradient);stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].node .outer-path{filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].node .neo-line path{stroke:rgb(83, 154, 248);filter:none;}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].node circle{stroke:url(#chatgpt-mermaid-\_r_33c\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].node circle .state-start{fill:#000000;}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].icon-shape .icon{fill:url(#chatgpt-mermaid-\_r_33c\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_33c\_ [data-look="neo"].icon-shape .icon-neo path{stroke:url(#chatgpt-mermaid-\_r_33c\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_33c\_ .node text{font-size:14px;font-weight:600;letter-spacing:normal;fill:rgb(0, 79, 153);}#chatgpt-mermaid-\_r_33c\_ .edgeLabels text{font-size:13px;font-weight:600;letter-spacing:-0.08px;fill:rgb(0, 79, 153);}#chatgpt-mermaid-\_r_33c\_ .node tspan[font-weight="normal"],#chatgpt-mermaid-\_r_33c\_ .edgeLabels tspan[font-weight="normal"]{font-weight:600;}#chatgpt-mermaid-\_r_33c\_ .edgeLabel .label rect{opacity:1;rx:13px;ry:13px;fill:rgb(245, 250, 255);stroke:rgb(206, 219, 229);stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ .node rect,#chatgpt-mermaid-\_r_33c\_ .node circle,#chatgpt-mermaid-\_r_33c\_ .node ellipse,#chatgpt-mermaid-\_r_33c\_ .node polygon,#chatgpt-mermaid-\_r_33c\_ .node path{fill:rgb(229, 243, 255);stroke:rgba(0, 0, 0, 0.1);stroke-width:1px;}#chatgpt-mermaid-\_r_33c\_ .node rect{rx:16px;ry:16px;}#chatgpt-mermaid-\_r_33c\_ .mermaid-decision .label-container{fill:rgb(245, 250, 255);stroke:rgb(206, 219, 229);stroke-dasharray:2,2;}#chatgpt-mermaid-\_r_33c\_ .edgePaths .flowchart-link{stroke:rgb(143, 143, 143);stroke-width:1px;stroke-linecap:round;stroke-linejoin:round;}#chatgpt-mermaid-\_r_33c\_ .marker{fill:rgb(143, 143, 143);stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_33c\_ :root{--mermaid-font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";}添加网站校验 URL 和访问权限校验通过?返回明确错误保存网站创建扫描任务安全抓取页面抓取成功?记录抓取失败和证据解析 HTML执行诊断规则生成问题记录汇总扫描结果计算诊断分数保存扫描快照生成 HTML 报告展示结果和整改建议否是否是

## 3.2 扫描任务状态机

任务状态统一使用以下枚举：

| 状态          | 含义           | 允许进入的下一状态                                  |
| ----------- | ------------ | ------------------------------------------ |
| `queued`    | 已创建，等待执行     | `running`、`cancelled`                      |
| `running`   | 正在抓取或分析      | `succeeded`、`partial`、`failed`、`cancelled` |
| `succeeded` | 扫描完成，结果完整    | 终态                                         |
| `partial`   | 扫描完成，但存在部分失败 | 终态                                         |
| `failed`    | 扫描未能产生有效结果   | 终态                                         |
| `cancelled` | 用户取消或系统取消    | 终态                                         |

其中：

- `succeeded` 表示扫描流程完成且未发生影响完整性的错误。
- `partial` 表示部分页面或部分分析环节失败，但仍有可用结果。
- `failed` 表示扫描整体无法完成，例如目标无法访问且没有可分析页面。
- 规则发现严重 SEO 问题不应使任务变为 `failed`。问题严重程度与任务执行状态是两种不同概念。

### 任务执行要求

1. 任务创建后立即返回任务 ID，不让创建请求长时间等待扫描结束。
2. 执行器只领取符合条件的 `queued` 任务。
3. 使用数据库事务和条件更新防止同一任务被重复领取。
4. 每个任务记录开始时间、结束时间、耗时和错误摘要。
5. 服务启动时检查遗留的 `running` 任务。
6. 对无法确认执行结果的遗留任务，首版优先标记为失败或待人工重试，不应盲目自动重跑。

# 四、网站管理模块

## 4.1 网站实体

每个网站对应一个独立配置实体。

| 字段            | 类型               | 说明                    |
| ------------- | ---------------- | --------------------- |
| `id`          | BIGINT           | 主键                    |
| `name`        | VARCHAR(120)     | 网站名称                  |
| `base_url`    | VARCHAR(2048)    | 标准化后的网站地址             |
| `description` | VARCHAR(500)     | 可选备注                  |
| `status`      | VARCHAR(20)      | `active` / `disabled` |
| `created_at`  | DATETIME(3)      | 创建时间                  |
| `updated_at`  | DATETIME(3)      | 更新时间                  |
| `deleted_at`  | DATETIME(3) NULL | 软删除时间                 |

建议对 `base_url` 建立普通索引，但是否使用唯一约束需要结合租户模型和规范化规则决定。

## 4.2 网站创建流程

1. 接收网站名称、URL 和备注。
2. 去除 URL 前后空白。
3. 检查 URL 格式与协议。
4. 仅允许 `http` 和 `https`。
5. 拒绝 URL 中的用户名和密码。
6. 拒绝非标准端口或不在允许范围内的端口；V1.0 建议仅开放 `80` 和 `443`，特殊端口通过部署配置显式放行。
7. 解析域名并进行 DNS 和 IP 安全检查。
8. 规范化 URL，避免因尾部斜杠、默认端口等导致重复记录。
9. 保存网站记录。
10. 返回网站详情。

安全要求： 创建网站时的 URL 校验只是第一道防线。实际每次发起请求、解析域名和处理重定向时都必须再次执行目标安全校验。

# 五、页面抓取模块

## 5.1 V1.0 抓取范围

首版采用有限范围的 HTML 页面抓取。

| 项目        | 建议默认值      | 可配置性       |
| --------- | ---------- | ---------- |
| 每个任务页面数   | 1 页        | 首版默认仅首页    |
| HTTP 请求超时 | 10 秒       | 可通过服务端配置调整 |
| 响应体上限     | 2 MiB      | 可配置        |
| 最大重定向次数   | 3 次        | 可配置        |
| 单任务总时限    | 30 秒       | 可配置        |
| 并发扫描任务    | 2 个        | 可配置        |
| 允许协议      | HTTP、HTTPS | 固定安全边界     |
| 允许内容类型    | HTML、XHTML | 固定基础范围     |

以上是首版建议的工程默认值，不是经过所有部署环境验证的性能结论。实施时需要通过测试校准。

## 5.2 抓取输出结构

```
type FetchResult struct {
    RequestedURL  string
    FinalURL      string
    StatusCode    int
    ContentType   string
    Body          []byte
    ResponseBytes int64
    DurationMS    int64
    RedirectCount int
    FetchedAt     time.Time
}
```

实现时应注意：

- 不要在数据库中无条件保存完整响应正文。
- 页面正文用于分析时可短暂保留在内存中。
- 快照优先保存抽取后的特征与必要证据。
- 日志中避免记录可能包含敏感查询参数的完整 URL。
- 如果抓取失败，应保留可解释的错误代码，而不是只返回“扫描失败”。

## 5.3 SSRF 防护设计

由于 GeoPilot 会访问用户提交的 URL，扫描器必须视为安全敏感组件。

第一层：URL 解析

拒绝不支持的协议、异常 URL、嵌入式凭据和未允许的端口。

第二层：DNS 与 IP 检查

拒绝环回、私有、链路本地、未指定、多播及云元数据相关目标；同时处理 IPv4、IPv6、IPv4 映射 IPv6 等形式。

第三层：重定向检查

每次重定向均重新解析并验证目标，禁止通过公开域名跳转访问内网。

第四层：连接级约束

DNS 解析与实际连接目标应绑定校验，避免 DNS rebinding；配合网络出口 ACL 阻止访问内网和元数据服务。

特别说明： 仅在请求前检查一次 DNS 结果是不够的。实现时需要通过自定义 DNS 解析与 `DialContext` 等连接机制确保实际建立连接的 IP 仍符合策略，并正确处理 IPv4/IPv6。生产环境还应配置网络出口限制，作为应用层防护的补充。

# 六、HTML 分析模块

## 6.1 页面特征结构

抓取成功后，分析器将原始 HTML 转换为结构化页面特征，供规则引擎使用。

```
type PageFeatures struct {
    URL                string
    FinalURL           string
    HTTPStatus         int
    ContentType        string
    Title              string
    MetaDescription    string
    CanonicalURL       string
    H1Texts            []string
    H2Texts            []string
    Language           string
    VisibleText        string
    WordCount          int
    JSONLDScripts      []string
    RobotsMeta         []string
    RobotsTxtStatus    string
    ContentHash        string
    ParseWarnings      []string
}
```

以上为概念性结构。实际实现应明确字段是否允许为空，并限制 `VisibleText`、JSON-LD 等字段的最大长度。

## 6.2 抽取规则

| 特征               | 实现方式                        | 注意事项             |
| ---------------- | --------------------------- | ---------------- |
| 页面标题             | 提取 `title`                  | 处理空标题、重复标签       |
| Meta Description | 提取 `meta[name=description]` | HTML 属性名大小写不敏感   |
| Canonical        | 提取 `link[rel=canonical]`    | 可能出现多个值或相对 URL   |
| H1               | 收集所有 H1 文本                  | 保留数量和文本证据        |
| H2               | 收集 H2 文本                    | 供后续内容结构分析        |
| 正文文本             | 去除脚本、样式等非正文节点后提取            | 不将原始 HTML 标签计入字数 |
| 语言声明             | 提取 `html[lang]`             | 只检查声明，不推断真实语言    |
| JSON-LD          | 收集 `application/ld+json`    | 不执行脚本，不直接信任结构化数据 |
| Robots Meta      | 提取 robots 相关指令              | 注意多个标签的组合语义      |
| 内容哈希             | 对规范化文本计算哈希                  | 用于历史变化检测         |

建议使用成熟的 Go HTML 解析库，而不是用正则表达式解析整份 HTML。正则可用于局部匹配，不能代替 HTML 解析器。

# 七、诊断规则引擎

## 7.1 规则接口

所有诊断规则实现统一接口。

```
type Rule interface {
    ID() string
    Name() string
    Category() string
    Evaluate(page PageFeatures) []IssueFinding
}
```

`IssueFinding` 表示规则发现的问题：

```
type IssueFinding struct {
    RuleID      string
    Title       string
    Severity    string
    Description string
    Evidence    string
    Suggestion  string
    Category    string
}
```

抓取失败等没有有效 `PageFeatures` 的场景，应由扫描流程或独立的任务级规则生成问题，避免强行传入不完整页面数据。

## 7.2 V1.0 规则清单

| 规则 ID                      | 检查内容         | 默认严重级别 | 触发条件示例         |
| -------------------------- | ------------ | ------ | -------------- |
| `HTTP_FETCH_FAILED`        | 页面抓取失败       | 严重     | 无法获取目标页面       |
| `HTTP_STATUS`              | HTTP 状态异常    | 严重/高   | 页面返回 4xx、5xx   |
| `TITLE_MISSING`            | 标题缺失         | 高      | 未提取到有效标题       |
| `TITLE_LONG`               | 标题过长         | 低/中    | 超过可配置长度阈值      |
| `META_DESCRIPTION_MISSING` | 描述缺失         | 中      | 未提取到描述         |
| `CANONICAL_MISSING`        | Canonical 缺失 | 低      | 未声明规范 URL      |
| `H1_MISSING`               | H1 缺失        | 中      | 页面没有 H1        |
| `H1_MULTIPLE`              | H1 数量较多      | 低      | 超过规则阈值         |
| `SCHEMA_MISSING`           | 未检测到 JSON-LD | 低      | 页面没有 JSON-LD   |
| `THIN_CONTENT`             | 正文内容过少       | 中      | 正文文本低于阈值       |
| `RESOURCE_UNAVAILABLE`     | 页面资源或依赖不可用   | 中      | 仅在实际执行了对应检查时触发 |

严重程度只是产品内部的诊断优先级，不等同于搜索引擎或 AI 平台的官方评价。

### 规则实现约束

1. 每条规则有稳定的唯一 ID。
2. 规则输出必须包括触发证据。
3. 建议应尽量具体，避免只输出“请优化 SEO”。
4. 阈值通过配置集中管理，避免散落在业务代码中。
5. 对于不适用于某类页面的规则，应记录为“不适用”或不触发，不能误判为问题。
6. 缺少 JSON-LD 不一定意味着页面有缺陷，不能仅凭此项判定网站不具备 GEO 能力。
7. 首版规则均基于可观察的页面与 HTTP 特征，不宣称能直接测量 AI 平台的真实引用概率。

## 7.3 规则执行流程

```
FetchResult
    │
    ▼
HTML Analyzer
    │
    ▼
PageFeatures
    │
    ▼
Rule Registry
    │
    ├── HTTP_STATUS
    ├── TITLE_MISSING
    ├── META_DESCRIPTION_MISSING
    ├── CANONICAL_MISSING
    ├── H1_MISSING
    ├── SCHEMA_MISSING
    └── THIN_CONTENT
    │
    ▼
IssueFinding[]
    │
    ▼
Issue Persistence
```

每次扫描应记录实际执行的规则集版本，避免未来规则变化后无法解释旧报告的评分差异。

# 八、评分与优先级计算

## 8.1 初版评分算法

沿用产品需求文档中的启发式扣分规则：

| 严重程度 | 单项扣分 |
| ---- | ---- |
| 严重   | 30   |
| 高    | 18   |
| 中    | 10   |
| 低    | 4    |

初始分数为 100，最终分数限制在 0–100。

\\[ Score=\max(0,\min(100,100-\sum Penalty_i)) \\]

示例：某页面发现 1 个高优先级问题、2 个中优先级问题和 1 个低优先级问题。

\\[ Score=\max(0,100-18-10-10-4)=58 \\]

此算法用于帮助用户比较同一产品内的诊断结果，不是行业标准，也不是任何 AI 搜索平台的官方评分。

## 8.2 防止重复扣分

如果多个规则实际反映同一个根因，可能造成重复扣分。例如，页面抓取失败后不能再将标题、H1、描述等缺失全部视为独立问题。

建议采用以下策略：

- 页面无法抓取时，保留抓取失败问题，不运行依赖 HTML 的规则。
- 同一规则在同一页面的一次扫描中默认只计一次分。
- 同一规则若命中多个节点，证据可以多条，但评分按规则定义计算。
- 规则间支持 `suppression` 或前置条件机制，避免明显重复处罚。
- 评分时保存每条问题的扣分值和算法版本，确保报告可追溯。

## 8.3 问题优先级

问题排序不应完全依赖严重程度。建议综合：

1. 严重级别；
2. 是否阻断页面抓取或理解；
3. 影响范围；
4. 修复成本；
5. 问题证据是否充分。

V1.0 可以先采用“严重级别优先、阻断问题优先、同级按规则顺序”的确定性排序。影响范围与修复成本在没有可靠数据时不应伪装成精确量化指标。

# 九、数据库设计

## 9.1 核心表

| 表名               | 作用             | 关键字段                                                   |
| ---------------- | -------------- | ------------------------------------------------------ |
| `websites`       | 网站信息           | `id`、`name`、`base_url`、`status`                        |
| `scan_tasks`     | 扫描任务           | `id`、`website_id`、`status`、`started_at`、`finished_at`  |
| `page_snapshots` | 页面扫描快照         | `id`、`scan_task_id`、`url`、`http_status`、`content_hash` |
| `scan_issues`    | 诊断问题           | `id`、`scan_task_id`、`snapshot_id`、`rule_id`、`severity` |
| `scan_reports`   | 报告元数据          | `id`、`scan_task_id`、`format`、`storage_path`            |
| `users`          | 用户信息（启用认证时）    | `id`、`email`、`password_hash`、`status`                  |
| `audit_logs`     | 安全审计（公网部署建议启用） | `id`、`actor_id`、`action`、`created_at`                  |

如果 V1.0 采用单用户本地部署，`users` 和部分权限设计可以简化；但公网服务不得以“暂时只有一个用户”为由直接开放高风险扫描接口。

## 9.2 表关系

\#chatgpt-mermaid-\_r_34p\_{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;fill:rgb(13, 13, 13);}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#chatgpt-mermaid-\_r_34p\_ .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_34p\_ .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_34p\_ .error-icon{fill:rgb(243, 243, 243);}#chatgpt-mermaid-\_r_34p\_ .error-text{fill:rgb(13, 13, 13);stroke:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_34p\_ .edge-thickness-normal{stroke-width:1px;}#chatgpt-mermaid-\_r_34p\_ .edge-thickness-thick{stroke-width:3.5px;}#chatgpt-mermaid-\_r_34p\_ .edge-pattern-solid{stroke-dasharray:0;}#chatgpt-mermaid-\_r_34p\_ .edge-thickness-invisible{stroke-width:0;fill:none;}#chatgpt-mermaid-\_r_34p\_ .edge-pattern-dashed{stroke-dasharray:3;}#chatgpt-mermaid-\_r_34p\_ .edge-pattern-dotted{stroke-dasharray:2;}#chatgpt-mermaid-\_r_34p\_ .marker{fill:rgb(143, 143, 143);stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_34p\_ .marker.cross{stroke:rgb(143, 143, 143);}#chatgpt-mermaid-\_r_34p\_ svg{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;}#chatgpt-mermaid-\_r_34p\_ p{margin:0;}#chatgpt-mermaid-\_r_34p\_ .entityBox{fill:rgb(222, 234, 251);stroke:rgb(83, 154, 248);}#chatgpt-mermaid-\_r_34p\_ .relationshipLabelBox{fill:rgb(243, 243, 243);opacity:0.7;background-color:rgb(243, 243, 243);}#chatgpt-mermaid-\_r_34p\_ .relationshipLabelBox rect{opacity:0.5;}#chatgpt-mermaid-\_r_34p\_ .labelBkg{background-color:rgba(243, 243, 243, 0.5);}#chatgpt-mermaid-\_r_34p\_ .edgeLabel{background-color:rgb(252, 252, 252);}#chatgpt-mermaid-\_r_34p\_ .edgeLabel .label rect{fill:rgb(252, 252, 252);}#chatgpt-mermaid-\_r_34p\_ .edgeLabel .label text{fill:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_34p\_ .edgeLabel .label{fill:rgb(83, 154, 248);font-size:14px;}#chatgpt-mermaid-\_r_34p\_ .label{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";color:rgb(13, 13, 13);}#chatgpt-mermaid-\_r_34p\_ .edge-pattern-dashed{stroke-dasharray:8,8;}#chatgpt-mermaid-\_r_34p\_ .node rect,#chatgpt-mermaid-\_r_34p\_ .node circle,#chatgpt-mermaid-\_r_34p\_ .node ellipse,#chatgpt-mermaid-\_r_34p\_ .node polygon{fill:rgb(222, 234, 251);stroke:rgb(83, 154, 248);stroke-width:1px;}#chatgpt-mermaid-\_r_34p\_ .relationshipLine{stroke:rgb(143, 143, 143);stroke-width:1px;fill:none;}#chatgpt-mermaid-\_r_34p\_ .marker{fill:none!important;stroke:rgb(143, 143, 143)!important;stroke-width:1;}#chatgpt-mermaid-\_r_34p\_ [data-look=neo].labelBkg{background-color:rgba(243, 243, 243, 0.5);}#chatgpt-mermaid-\_r_34p\_ .node .neo-node{stroke:rgb(83, 154, 248);}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].node rect,#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].cluster rect,#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].node polygon{stroke:url(#chatgpt-mermaid-\_r_34p\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].swimlane.cluster rect{filter:none;}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].node path{stroke:url(#chatgpt-mermaid-\_r_34p\_-gradient);stroke-width:1px;}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].node .outer-path{filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].node .neo-line path{stroke:rgb(83, 154, 248);filter:none;}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].node circle{stroke:url(#chatgpt-mermaid-\_r_34p\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].node circle .state-start{fill:#000000;}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].icon-shape .icon{fill:url(#chatgpt-mermaid-\_r_34p\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_34p\_ [data-look="neo"].icon-shape .icon-neo path{stroke:url(#chatgpt-mermaid-\_r_34p\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_34p\_ :root{--mermaid-font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";}WEBSITESbigintidPKvarcharnamevarcharbase_urlvarcharstatusdatetimecreated_atSCAN_TASKSbigintidPKbigintwebsite_idFKvarcharstatusintscoredatetimestarted_atdatetimefinished_atPAGE_SNAPSHOTSbigintidPKbigintscan_task_idFKvarcharurlinthttp_statusvarcharcontent_hashSCAN_ISSUESbigintidPKbigintscan_task_idFKbigintsnapshot_idFKvarcharrule_idvarcharseveritySCAN_REPORTSbigintidPKbigintscan_task_idFKvarcharformatvarcharstorage_pathUSERSAUDIT_LOGShasproducesdiscoversevidence_forgeneratesperforms

## 9.3 关键约束

- 外键关系应在数据库层明确建立，或由经过测试的应用层约束替代。
- 任务、快照和问题的创建应尽量放在同一个结果持久化事务中。
- 报告文件生成可以在事务外执行，但必须处理文件已生成而元数据未保存的情况。
- 扫描结果需要保留规则版本、评分算法版本和关键配置快照。
- 任务 ID 应使用稳定、不可预测的外部标识，避免简单自增 ID 被当作权限校验。
- 所有查询均应校验当前用户是否有权访问目标网站和任务。
- 软删除网站后，应明确历史扫描是否保留；建议保留历史结果，避免删除配置导致审计记录丢失。

# 十、REST API 设计

统一前缀：

```
/api/v1
```

统一返回格式建议：

```
{
  "code": "OK",
  "message": "success",
  "data": {}
}
```

分页列表可以返回：

```
{
  "code": "OK",
  "message": "success",
  "data": {
    "items": [],
    "page": 1,
    "page_size": 20,
    "total": 0
  }
}
```

## 10.1 网站接口

| 方法       | 路径               | 用途     |
| -------- | ---------------- | ------ |
| `GET`    | `/websites`      | 网站列表   |
| `POST`   | `/websites`      | 创建网站   |
| `GET`    | `/websites/{id}` | 网站详情   |
| `PATCH`  | `/websites/{id}` | 更新网站配置 |
| `DELETE` | `/websites/{id}` | 软删除网站  |

## 10.2 扫描接口

| 方法     | 路径                     | 用途               |
| ------ | ---------------------- | ---------------- |
| `POST` | `/websites/{id}/scans` | 创建扫描任务           |
| `GET`  | `/scans`               | 扫描历史列表           |
| `GET`  | `/scans/{id}`          | 任务详情与状态          |
| `GET`  | `/scans/{id}/issues`   | 任务诊断结果           |
| `GET`  | `/scans/{id}/report`   | 获取或下载 HTML 报告    |
| `POST` | `/scans/{id}/cancel`   | 取消尚未结束的任务        |
| `POST` | `/scans/{id}/retry`    | 对符合条件的失败任务重新创建扫描 |

重试接口建议创建新任务并记录来源任务 ID，而不是清空原任务结果后复用同一个任务 ID。这样更容易审计和比较历史变化。

## 10.3 创建扫描示例

请求：

```
POST /api/v1/websites/1001/scans
Content-Type: application/json
```

响应：

```
{
  "code": "OK",
  "message": "scan accepted",
  "data": {
    "scan_id": "scn_01_example",
    "status": "queued"
  }
}
```

`scn_01_example` 仅为接口格式示例，并非实际任务 ID。

## 10.4 错误码建议

| 错误码                  | 含义          | HTTP 状态        |
| -------------------- | ----------- | -------------- |
| `VALIDATION_ERROR`   | 请求参数不合法     | 400            |
| `UNAUTHORIZED`       | 未认证         | 401            |
| `FORBIDDEN`          | 无权限         | 403            |
| `RESOURCE_NOT_FOUND` | 资源不存在       | 404            |
| `DUPLICATE_RESOURCE` | 资源重复        | 409            |
| `SCAN_LIMIT_REACHED` | 扫描并发或配额达到限制 | 429            |
| `TARGET_BLOCKED`     | 目标违反网络安全策略  | 422            |
| `FETCH_TIMEOUT`      | 抓取超时        | 200 或任务结果内部错误码 |
| `SCAN_FAILED`        | 扫描任务失败      | 200 或任务结果内部错误码 |
| `INTERNAL_ERROR`     | 服务内部异常      | 500            |

需要区分两种错误：

- API 请求错误：例如参数非法、没有权限，使用对应 HTTP 状态码。
- 扫描任务执行结果：任务已经成功创建，但远端网站无法访问。这种情况应通过任务状态和错误详情体现，而不是让创建扫描的 HTTP 请求一直等待远端访问结果。

# 十一、前端页面与交互实现

## 11.1 页面规划

[Free AI Website Audit Tool - SEO, Performance & Security Checker | ReviewMySite](https://images.openai.com/static-rsc-4/hCGWqBkA8OuEym27gnqmESB6lgYtWaSxlwkB_FYaUT8nflfRmvI6MNfkUrmOm7YnML_Ow-0Cm6o6CbojAs3wg9d0yybrTcnA-AvrcuLrfJYZG2C0ihu6DW0-KbIgkiBhpihjSOH7rZ153Y1VfstEUC_x4RzaeVtngi7fKCzPPPU?purpose=inline)

01 · 工作台

网站数量、最近扫描、诊断分数、待整改问题、最近任务状态。

[AltumCode on Instagram: "66audit ⚡️ websites page view 🔥" | App design, Website, Ui design](https://images.openai.com/static-rsc-4/LCASYm60oDjJm3eDZZSrJO3DDcsh02Cp4yTJXA19CAzARXwGy2pOKvcm9AmZLSQCAP4GiNXGXBd8e2J6O2n9dejTmUnW-HVla5214IJnbSZLjyz5H8RHf12Ux6nOweq07QZD85yIS8pvACAJwRBTNZnv-Qeig-SKo7DjFpx-9Xc?purpose=inline)

02 · 网站管理

网站列表、创建网站、编辑配置、启用与停用、触发扫描。

[Telescope Site Audit is Live! - Telescope Blog](https://images.openai.com/static-rsc-4/UpPKEg_Ye8d_7OKAfsNhaa182kcj2fDNOM2loku9kmD2OYlH3wo9l6Ep9gNpT7clVSAUpX5ZC67RkcoOLml8uW2dcAA7Sy8E8ZLhntqe3x9Z1UDcyyWa1m1qp9Nh0sdV44KUrMyOSvnRgO_7HP4aHIZ8-J3No9APRZCPEb7DKQA?purpose=inline)

03 · 问题中心

按严重程度、规则类型、网站和扫描批次筛选问题，查看证据与建议。

[SEO-Audit System für Agenturen: mehr Projekte gewinnen | Seitenreport](https://images.openai.com/static-rsc-4/j1ozhI981kYyG4N7TxpqHqeCXgB7i4t7Sle5_giT1nPXqfMzzqI_VNTvTXYRKSUF-D1sCRSpWkVMN9C6JzYVseLrKndo87ZxRM8DsxRejiTcUZC26QlyukIIAshznbMfz2g2xGuz5tzzBiYnl70FNPdxwJ37ugkrDnCfDBwtOvw?purpose=inline)

04 · 扫描详情与报告

展示任务状态、页面信息、诊断结果、评分、历史记录和 HTML 报告。

以上图片仅用于表达可能的视觉方向，不代表最终 UI 设计稿。

## 11.2 前端技术约束

- 使用 TypeScript 严格模式。
- Vite 负责本地开发与生产构建。
- Tailwind CSS 负责样式组织。
- 采用 shadcn/ui 风格的可组合组件与 Lucide React 图标。
- 不使用 Ant Design。
- 页面按功能模块组织，避免将全部业务逻辑堆在单一组件中。
- API 调用集中放在 `services/`，避免组件直接拼接请求地址。
- 统一管理加载、空状态、错误状态和权限错误。
- 表单应进行前端校验，但后端仍是最终校验方。
- 对扫描状态使用轮询即可；首版不必为了状态更新引入 WebSocket。

## 11.3 扫描详情页

扫描详情页建议包含：

1. 扫描任务状态及开始、结束时间。
2. 网站地址与最终访问 URL。
3. HTTP 状态码、响应耗时和页面标题。
4. 诊断总分及评分算法版本。
5. 严重、高、中、低优先级问题数量。
6. 问题列表与证据详情。
7. 扫描失败原因和重试入口。
8. HTML 报告查看或下载入口。

用户必须能够区分“扫描没有发现问题”和“扫描失败、没有足够证据得出结论”。两者不能共用同一个空白结果页。

# 十二、报告生成模块

## 12.1 报告内容

HTML 报告建议包含：

- 报告编号和生成时间；
- 网站名称、URL 和扫描时间；
- 扫描状态；
- 诊断分数与算法说明；
- 问题总数和严重程度分布；
- 问题详情、证据与整改建议；
- 抓取限制和分析范围；
- 规则版本与报告版本；
- GEO 能力边界声明。

报告应明确本次只分析了哪些页面，以及哪些检查没有执行。例如，只扫描首页时，不能将首页诊断结果描述成整个网站的完整审计。

## 12.2 报告安全要求

1. 对网站标题、URL、正文摘录和问题证据进行 HTML 转义。
2. 禁止将目标网站 HTML 原样插入报告模板。
3. 报告文件名使用安全 ID，不直接使用用户输入作为文件路径。
4. 下载接口必须校验任务访问权限。
5. 报告存储路径必须限制在指定目录内。
6. 如报告包含外部链接，应使用安全的链接处理策略。
7. 不将 API 密钥、认证 Cookie、内部地址等敏感信息写入报告。

# 十三、配置与运行模式

建议通过环境变量管理服务端配置。

```
APP_ENV=development
APP_ADDR=:8080

DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=geopilot
DB_USER=geopilot
DB_PASSWORD=replace_with_secure_password

SCAN_MAX_CONCURRENCY=2
SCAN_REQUEST_TIMEOUT_SECONDS=10
SCAN_TASK_TIMEOUT_SECONDS=30
SCAN_MAX_RESPONSE_BYTES=2097152
SCAN_MAX_REDIRECTS=3

REPORT_STORAGE_DIR=./data/reports
```

以上为配置示例，不应将真实生产密码提交到 Git。

数据库连接池、日志级别、允许端口、私网目标策略和认证开关也应纳入配置。对于高风险安全策略，建议采用默认拒绝原则，不允许通过随意修改一个普通环境变量就绕过所有保护。

## 13.1 部署方式

首版建议提供两种运行方式：

- 本地开发： 前端 Vite 开发服务器 + Go API + MySQL。
- 自托管部署： Docker Compose 启动 Go API、MySQL 与前端静态资源服务，或由现有反向代理提供前端文件。

公网部署必须启用认证、权限校验、HTTPS、请求限流和网络出口限制。若没有完成这些保护，建议只允许本机或可信内网访问。

# 十四、测试与质量保障

## 14.1 后端测试

| 测试类型   | 核心内容                        |
| ------ | --------------------------- |
| 单元测试   | URL 规范化、规则触发条件、评分计算         |
| 表驱动测试  | 空标题、多 H1、无描述、无 JSON-LD      |
| 集成测试   | 数据库 CRUD、扫描任务创建、结果持久化       |
| 网络安全测试 | 私网 IP、IPv6、重定向、DNS 变化、元数据地址 |
| 异常测试   | 超时、连接拒绝、非 HTML 响应、超大响应体     |
| 恢复测试   | 服务重启后遗留任务的处理                |
| 报告测试   | HTML 转义、下载权限、路径安全           |

## 14.2 前端测试

- 网站创建与表单校验。
- 扫描任务状态刷新。
- 任务失败与部分成功展示。
- 问题筛选和分页。
- 报告查看与下载。
- API 错误提示。
- 无数据、加载中、网络断开等状态。

## 14.3 安全测试重点

安全测试必须包括以下情况：

- `http://127.0.0.1` 和 `http://localhost`；
- RFC 1918 私有 IPv4 地址；
- IPv6 环回、链路本地和本地地址；
- IPv4 映射 IPv6 地址；
- 解析到公网地址、随后解析到内网地址的域名；
- 重定向到内网、跨协议重定向和重定向循环；
- 超大响应、压缩炸弹风险和慢速响应；
- 恶意 HTML、脚本标签、事件属性和异常字符；
- 未授权用户访问其他用户的任务及报告。

不能只验证 URL 字符串，而忽略实际网络连接目标。

# 十五、分阶段实施计划

建议按依赖关系分为六个阶段。以下是开发顺序建议，不是工期承诺。

## 01

工程骨架与数据层

初始化 Go、React、MySQL；建立迁移、配置、日志、错误处理与基础 API。

验收：项目可启动，数据库可迁移，基础接口可测试。

## 02

网站管理与安全抓取

实现网站 CRUD、URL 校验、受限 HTTP 客户端、重定向检查和抓取结果。

验收：能够安全地获取允许访问的测试页面，拒绝不安全目标。

## 03

HTML 分析与诊断规则

实现页面特征提取、规则注册、证据生成和问题持久化。

验收：固定测试 HTML 可以得到确定、可重复的诊断结果。

## 04

任务管理与评分

实现异步任务执行、状态流转、异常处理、评分与历史快照。

验收：从创建扫描到获取结果形成完整闭环。

## 05

前端工作台与报告

实现网站列表、扫描详情、问题中心、历史记录和 HTML 报告。

验收：用户不依赖 API 调试工具即可完成主要操作。

## 06

安全加固与交付

完成权限校验、限流、恢复机制、端到端测试、部署文档和验收材料。

验收：可重复部署，核心功能和安全用例通过测试。

# 十六、V1.0 验收基线

| 编号     | 验收项     | 通过标准                       |
| ------ | ------- | -------------------------- |
| TC-001 | 网站管理    | 能创建、查询、更新和软删除网站            |
| TC-002 | URL 安全  | 拒绝协议非法及受限网络目标              |
| TC-003 | 扫描创建    | 创建任务后返回任务 ID               |
| TC-004 | 状态管理    | 任务状态按状态机流转                 |
| TC-005 | 页面抓取    | 正确记录状态码、最终 URL、耗时与错误       |
| TC-006 | HTML 分析 | 正确提取标题、描述、H1、Canonical 等字段 |
| TC-007 | 规则诊断    | 固定测试用例触发预期规则               |
| TC-008 | 证据追溯    | 每条问题都能追溯到规则与证据             |
| TC-009 | 评分计算    | 评分与已定义算法一致                 |
| TC-010 | 历史记录    | 能查看不同扫描任务的独立结果             |
| TC-011 | 报告      | 能生成并安全查看或下载 HTML 报告        |
| TC-012 | 权限控制    | 未授权访问不能读取或修改受保护资源          |
| TC-013 | 故障处理    | 超时、异常退出和失败任务可解释、可恢复        |
| TC-014 | 部署交付    | 提供配置说明、数据库迁移和部署步骤          |

以上每项都需要落实为可重复执行的测试用例，并记录实际测试结果。不能仅凭页面能够打开就判定项目通过验收。

# 十七、需要在开发前冻结的事项

当前技术方案已经可以作为工程设计初稿，但以下决策会直接影响数据库、权限与实现范围。

## GeoPilot V1.0 设计决策确认

可直接采用推荐项，也可以按实际目标调整。提交后可据此继续冻结技术方案。

1\. 部署方式

本地单机：优先快速开发和个人使用

自托管服务器：推荐，支持个人或小团队使用

公网 SaaS：需要完整认证、租户隔离与安全防护

2\. 扫描范围

仅首页：首版最简单，风险更可控

首页 + 有限站内页面：需要页面发现、数量限制和更完整的调度

3\. 登录与权限

仅本机使用，不实现登录

单用户管理员登录

账号登录，为后续多用户扩展预留

4\. 历史与问题管理

只保留每次扫描快照与报告

增加历史问题差异、问题状态（待处理、已修复、已忽略）

5\. AI 平台可见度

V1.0 暂不接入，先完成可验证的技术诊断

加入人工采样记录，记录平台、查询词、时间和观察结果

6\. 报告格式

只生成 HTML

生成 HTML + JSON，便于后续集成

确认决策并继续完善文档

我的建议： 先选择自托管部署、有限首页扫描、单用户管理员登录、历史问题状态管理、暂不接入 AI 平台采样，以及 HTML + JSON 报告。这样既能让 GeoPilot V1.0 具备实用价值，也能控制首版的实现复杂度。

下一份配套文档应是 《GeoPilot 数据库设计与 REST API 接口规范 V1.0》，将本文中的概念表结构细化为字段、索引、约束、请求响应、分页规则和错误码，再据此拆分开发任务与验收用例。