我建议把这个项目命名为 GeoPilot，定位为一款帮助开发者和企业诊断网站 GEO 问题、优化 AI 搜索可见度、跟踪优化效果的轻量级工具。

结合你已经确定的技术选型，第一版不做复杂的 SaaS 平台，不上重量级 UI 框架，先做一个能扫描真实网站、发现问题、给出整改建议、生成报告的可用系统。

# 一、GeoPilot 是什么？

[AI Search Visibility & SEO Platform | SEObolt](https://images.openai.com/static-rsc-4/VDj_ZhvHvHl2m30anRl7wWKzS6CgYUjO3KAMf7WmstFfaII-77Ha5H7wukraPdNzjZKVMkxfTPNkNzjPF7pWrO3pm6WtiUM-EPb5RR1MALGuPhbHqRqaufqrsGUIAGn2NZG2cZ25QLTDIOZxfTVxHvEVEMtIJHhSWrAy1cp8ttA?purpose=inline)

[seobolt.io](https://seobolt.io/?utm_source=chatgpt.com)

网站诊断

自动检查网站是否便于搜索引擎和 AI 系统读取。

内容优化

识别内容缺口，提供可执行的修改建议。

AI 可见度

记录品牌在 AI 问答中的提及和官网引用情况。

客户报告

将诊断结果整理成专业的 GEO 优化报告。

它不是简单的 SEO 检查器。传统 SEO 更关注搜索排名和关键词；GeoPilot 还要关注内容能否被理解、品牌信息是否完整，以及 AI 回答中是否出现品牌或引用官网。

需要注意：改善这些因素可以增加网站被发现和正确引用的机会，但不能保证某个 AI 平台一定收录或推荐网站。

# 二、第一版应该有哪些功能？

我建议把功能分成三个阶段，第一阶段只实现最重要的功能。

## P0：第一版必须完成

1\. 项目与网站管理

添加客户项目、网站名称、网站 URL 和行业信息；查看每个网站的诊断记录。

- 创建、编辑、删除项目
- 保存网站基本信息
- 查看最近一次扫描状态

2\. 网站扫描与技术诊断

输入网站地址，自动获取网页和关键配置，发现影响抓取、理解和内容发现的问题。

- HTTP 状态码、跳转和页面可访问性
- Title、Meta Description、Canonical 检查
- robots.txt 和 sitemap.xml 检查
- HTML 内容提取、标题层级检查
- JSON-LD 结构化数据检查
- 内部链接和页面发现情况

3\. 问题清单与整改建议

不只是展示检测结果，还要告诉用户问题是什么、为什么重要、如何修复。

- 严重、高、中、低四级优先级
- 问题对应的页面和证据
- 整改说明与建议示例
- 待处理、处理中、已完成状态

4\. 诊断报告

把扫描结果转换为可以交付给客户的报告。

- 总体评分与问题统计
- 关键问题及整改优先级
- 页面级诊断明细
- HTML 报告及 JSON 数据导出

## P1：基础版本稳定后增加

| 功能       | 具体作用                      | 优先级 |
| -------- | ------------------------- | --- |
| 内容质量分析   | 检查内容是否直接回答问题、信息是否完整       | 高   |
| 品牌信息检查   | 检查企业名称、产品、服务、联系方式是否一致     | 高   |
| FAQ 分析   | 找出客户常见问题及网站内容缺口           | 高   |
| AI 可见度记录 | 手工记录不同 AI 平台的回答、品牌提及和官网引用 | 高   |
| 竞品比较     | 对比多个网站的内容覆盖与技术问题          | 中   |
| 历史扫描对比   | 查看整改前后的问题变化               | 中   |

AI 可见度模块第一版建议先采用人工测试 + 结果录入，不要一开始就尝试自动控制各个 AI 平台的网页界面。后续再根据官方 API、平台规则和可用接口扩展。

## P2：有真实客户需求后再开发

- 定时扫描与异常提醒
- AI 生成内容优化草稿
- 自动生成 FAQ 和结构化数据草稿
- 多用户、团队协作与权限管理
- 客户订阅、套餐、用量限制
- 多租户 SaaS 和在线报告分享

第一版不需要这些功能，也不需要先开发复杂的工作流引擎。

# 三、技术架构怎么设计？

按照你确定的 Go、TypeScript、React 和 MySQL 组合，我建议使用简单的前后端分离架构。

前端：React + TypeScript

Vite · Tailwind CSS · shadcn/ui · Lucide

项目管理、扫描页面、问题清单、报告展示

后端：Go

REST API · 扫描任务 · 诊断规则 · 报告生成

可使用 Gin 或标准库；第一版选一种即可

数据库：MySQL

项目、网站、扫描任务、问题记录、报告元数据

### 前端 UI 和图标建议

你明确不使用 Ant Design，我建议：

| 领域    | 选择           | 原因                 |
| ----- | ------------ | ------------------ |
| UI 组件 | shadcn/ui    | 可按需引入组件，方便自定义设计    |
| 样式    | Tailwind CSS | 快速实现一致的布局、间距和响应式界面 |
| 图标    | Lucide       | 风格统一、简洁，适合后台工具     |
| 图表    | Recharts     | 展示评分趋势和问题分布        |
| 前端构建  | Vite         | 轻量、开发效率高           |

整体 UI 建议使用浅色工作台、深色文字、低饱和蓝色主色、简洁图表。避免过多渐变、卡片和动画，让诊断结果与问题处理成为页面重点。

### 后端实现原则

第一版不必使用微服务、Redis、消息队列或 Kubernetes。

- Go：负责 API、网站扫描和诊断规则。
- MySQL：保存业务数据和扫描结果。
- 扫描任务：先采用数据库任务状态 + Go 后台工作协程。
- 网页解析：优先使用 Go HTML 解析库；确实遇到依赖 JavaScript 才能呈现的页面，再考虑浏览器渲染。
- 报告：优先生成 HTML，后续再增加 PDF。

注意，网站扫描器必须防止 SSRF：限制访问本机、内网、云元数据地址和非 HTTP(S) 协议，并对重定向后的地址重新校验。

# 四、数据库先设计哪些表？

第一版只需要少量核心表，不必一开始建立几十张表。

| 表名               | 保存内容            |
| ---------------- | --------------- |
| `projects`       | 客户项目名称、行业、备注    |
| `sites`          | 网站地址、项目关联、网站信息  |
| `scan_jobs`      | 扫描任务、状态、开始和结束时间 |
| `pages`          | 扫描发现的页面及页面基本信息  |
| `audit_findings` | 诊断问题、级别、证据和修复建议 |
| `reports`        | 报告记录、生成时间及文件路径  |

初期可以不做登录注册和复杂权限。如果要把系统公开部署到互联网，至少应补上身份验证、访问控制和基础限流。

# 五、页面应该如何规划？

[Modern Analytics Dashboard – Clean React UI/UX by Ahmed hamdy on Dribbble](https://images.openai.com/static-rsc-4/Wqjg8fyIUUNzgSgJIG5fyUBXiaLUM6vypIwycuORSY_-PgHPSZA8WkLbUuXwOsn9A2lkt_coHG8UDmLiN6pUk5rCrfUPy-JMdigcNOv0EtwuW1fwu5QSWTL9XcfNqIbYciOe-CIXVW_76HsNfKzF0BkXgfoEUTfnSLlTh3bczns?purpose=inline)

01 · 工作台

展示网站数量、最近扫描、待处理问题和最近报告。

[Free SEO Checker: Check for Hidden SEO Issues](https://images.openai.com/static-rsc-4/IRKS5dg92T_1-nFTXjK87FSNi5UYGojFMIw-zN8u60PoML-bXaDB9ObRc1lgSez7ZE6UfmnNGQI3YSMYPpGNBSYn-YhpBQfj7ZYJfFihEnKJoRVfuTJ3tZMcgJOXYL4vl_f9J8oPIx7Ysu8Q49tBHEmrKQG1XGkk0Kctys8gIdk?purpose=inline)

02 · 网站诊断

填写网站 URL、启动扫描、查看进度和诊断分数。

[MyTaskManager - Reimagined for modern business](https://images.openai.com/static-rsc-4/BCTf33s1mfoJem_EE7WRkemR7qD_cVZ-KyD2KqFeBBVOdus--8sgqp3q3O0vnnu-eUJS4FkY96Nr70SVr6Ny2pDQ-bes3Z8why-re4dW1eAl9MhEePFvMmVXz8cmiZrOBiEMGn42_-g5Tr10rp1B4GFxVvxb08q51VlGmyMmlZY?purpose=inline)

03 · 问题中心

按优先级、页面、问题类型和处理状态筛选问题。

[WEBSITE AUDIT | The Haven Agency](https://images.openai.com/static-rsc-4/9zA42RwuZWkjCfeujylS0fnfA3Ox6-XOPZJEiWTd39yNBHNLf1wezeh-ALuTZ89MMwCNeV8XYh3MVmttL0KlIC6eQzV7rG0p9PBUzQhnKcKTlSDJR6_1YKx79B_ajWdI-ncAMIj62xXPUZAubLPhODJiVOdQpFPgRbgN1RDL5Zs?purpose=inline)

04 · 报告中心

查看每次扫描的报告，比较整改前后的结果。

# 六、如何分阶段开发？

建议按可验收的功能推进，而不是一次性生成庞大项目。

1. 阶段一：项目骨架

   创建 Go 后端、React 前端、MySQL 配置、基础路由和数据库迁移。验收：可以启动前后端并完成数据库连接。
2. 阶段二：网站扫描

   实现 URL 校验、页面请求、页面解析、基础技术规则。验收：能够扫描一个授权测试网站并保存结果。
3. 阶段三：问题管理

   将扫描发现的问题展示为列表，支持筛选、查看证据、标记处理状态。验收：用户可以完成从发现到处理的闭环。
4. 阶段四：报告

   根据扫描结果生成报告，支持浏览器查看和导出。验收：同一次扫描可以重复查看，历史结果不会被新扫描覆盖。
5. 阶段五：AI 可见度

   建立问题测试集，记录 AI 平台回答、品牌提及和官网引用，并与网站整改记录关联。

## 七、第一版验收标准

- 可以创建、编辑和删除网站项目。
- 输入网站 URL 后可以启动扫描并看到任务状态。
- 至少实现 HTTP 状态、标题、描述、Canonical、robots.txt、sitemap.xml、结构化数据等诊断规则。
- 每条问题都有级别、页面地址、证据和建议。
- 扫描结果保存在 MySQL 中，页面刷新后仍然存在。
- 可以查看历史扫描结果并生成 HTML 报告。
- 扫描器具备访问限制、超时和响应大小控制。
- 提供启动说明、环境变量示例、数据库迁移和基础测试。

我的建议：第一版就把上述四个核心页面和六张核心表做扎实。 先证明 GeoPilot 能准确发现真实网站问题、给出有价值的整改建议，再投入 AI 自动化、竞品分析和商业化功能。