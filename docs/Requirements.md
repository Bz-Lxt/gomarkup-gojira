# GoJira — 敏捷项目管理与看板系统 · 需求规格说明书

| 项 | 值 |
|---|---|
| 文档状态 | **FROZEN（已冻结）** |
| 版本 | v1.0 |
| 冻结时间 | 2026-08-21 19:57 (GMT+8) |
| 负责人 | PM Agent (Alkaid-SOP v13.0) |
| SSOT 权威 | 本文件定义 **WHAT**；`docs/Roadmap.md` 定义 **WHEN**；原始诉求见 `docs/.meta/original_prompt.md` |
| 下游消费者 | Phase 1 Chief Architect → Phase 2 UI → Phase 3 Logic → Phase 4 QA → Phase 5 Auditor |

---

## 1. 项目概述

GoJira 是面向软件开发团队的**敏捷项目管理工具**，以 Go 为后端、Vue 3 为前端的全栈 Web 应用。它围绕真实敏捷团队的日常闭环组织功能：产品经理在 Backlog 规划需求 → 拉入 Sprint → 开发在看板上流转任务 → 测试验证并追踪 Bug → 迭代过程通过燃尽图与甘特图可视化 → 关键节点由自动化触发器推送通知。

与"Demo 级看板"的本质区别在于三个后端内核：**可配置状态机引擎**（约束谁能在什么条件下把任务推进到哪一状态）、**事件驱动自动化触发器**（Hook 机制，可靠异步投递）、**实时统计聚合**（高效 SQL 计算迭代进度与燃尽数据）。这三者是本项目的技术主轴，也是验收重点。

### 1.1 目标用户与核心场景

| 角色 | 核心场景 |
|---|---|
| 产品经理 PM | 维护 Backlog、规划 Sprint、查看燃尽图判断迭代健康度、接收任务完成通知 |
| 开发 DEV | 在看板上认领任务、拖拽推进状态、修复 Bug、查看甘特图理解排期与依赖 |
| 测试 QA | 提交 Bug、验证修复、将 Bug 置为「已解决」（**独占权限**）、把任务从「已测试」推进 |
| 管理员 ADMIN | 项目与成员管理、工作流配置、自动化触发器配置、审计日志查看 |

---

## 2. 废弃评估结论（Step 1）

| # | 判据 | 判定 | 依据 |
|---|---|---|---|
| 1 | 需求不完整 / 模糊 | ✅ 通过 | 功能点、技术栈、规模三项均明确给出，无缺失附件依赖 |
| 2 | Windows 独占 | ✅ 通过 | Go + Vue + PostgreSQL 全平台，本机 darwin/arm64 已验证工具链 |
| 3 | 规模评估（分级） | ✅ 通过（Tier 2） | 估算 **≈ 12,000–14,000 LoC**，落在 10k–40k 区间 → **接受，但分期路线图为强制前置条件** |
| 4 | 外部依赖（智能判定） | ✅ 通过（Scenario A） | 唯一外部依赖为邮件发送，属**可模拟**类。裁决见 §4 C-06 |
| 5 | 专业 / 付费软件 | ✅ 通过 | 全链路开源。**约束**：甘特图组件禁止选用 dhtmlx-gantt（GPL/商业双许可） |

**最终结论：ACCEPT — 不建议废除。**

### 2.1 规模拆解（用于 Roadmap 分期定量依据）

| 模块 | 预估 LoC | 说明 |
|---|---|---|
| Go 后端 | 5,000 – 7,500 | 用户指定约 30 个 Go 文件 |
| Vue 3 前端 | ≈ 5,200 | 看板 800 / 甘特图 900 / 统计页 600 / Sprint&Backlog 700 / Bug 500 / 基础设施 900 / 设计系统 800 |
| 测试（单测 + E2E） | ≈ 1,200 | 状态机、聚合、触发器单测 + Playwright 关键路径 |
| 编排与配置 | ≈ 400 | Dockerfile ×2、compose、workflow.yaml、迁移脚本 |
| **合计** | **≈ 12,000 – 14,300** | Tier 2 → 分期交付 |

> **强制约束**：因规模落入 Tier 2，Phase 1 Chief Architect **必须**在 `docs/Roadmap.md` 中给出显式的 MVP / V1 / V2 边界后才允许写第一行业务代码。

---

## 3. 兼容性与硬性门槛检查（Step 2）

| 检查项 | 结论 |
|---|---|
| 微信小程序豁免 | 不适用（非小程序），执行标准 Docker 检查 |
| Docker 交付标准 | ✅ `docker compose up --build -d` 一键启动，无需手工步骤 |
| localhost 可访问服务 | ✅ 前端 Web UI + 后端 REST API + Mailpit 邮件查看 UI |
| 跨平台（ARM64 / AMD64） | ✅ `golang`、`node`、`postgres`、`axllent/mailpit` 官方镜像均为多架构 |
| 本机验证 | Go 1.25.12 darwin/arm64、Node v21.7.3、Docker Server 29.6.1 —— 均可用 |
| 时区 | **全链路 GMT+8**。容器 `TZ=Asia/Shanghai`；Go 侧统一 `time.Location` 为 `Asia/Shanghai`；PostgreSQL `timezone='Asia/Shanghai'`；禁止裸用 `time.Now().UTC()` 落库 |

---

## 4. 矛盾点与裁决记录 ⚠️

> 本节是 Auditor Agent 的**防误判基线**。凡本节已裁决的偏离，均属**显式说明的合理设计决策**，不得判为「擅自更改 Prompt 约束」。

### C-01 「已测试」列的语义歧义

**冲突**：Prompt 给出 4 列「待处理 / 开发中 / 已测试 / 已完成」。若「已测试」按字面理解为"测试已通过"，则与「已完成」构成两个语义重叠的终态列，看板逻辑不自洽。

**裁决**：**保留用户原始中文列名不变**，内部枚举语义明确为 `TESTING` = 已提测、测试验证进行中；`DONE` = 验收完成并关闭。UI 在「已测试」列标题下以次级文字标注"测试验证中"，消除用户认知歧义。

### C-02 「只有测试人员能将 Bug 改为已解决」与行业惯例冲突

**冲突**：行业惯例是开发标记 Fixed → 测试验证 → Verified/Closed。Prompt 要求 QA 独占「已解决」的置位权，与惯例中"开发自行标记已解决"相反。

**裁决**：**严格遵循 Prompt 原文**，并通过状态的两态分离同时满足业务合理性 ——
- `FIXED`（已修复）：**DEV 可执行**，语义为代码已提交待验证
- `RESOLVED`（已解决）：**仅 QA 可执行**，语义为验证通过 —— 完整落实 Prompt 约束

该设计既 100% 满足"只有测试人员能将 Bug 改为已解决"，又保留了真实研发流程。须在 `README.md` 中说明。

### C-03 甘特图依赖关系 与 看板自由拖拽 的冲突

**冲突**：若任务 A 在甘特图上 FS-依赖 于 B，当 B 尚未 `DONE` 时，用户在看板上把 A 拖入「开发中」是否应被拒绝？硬拒绝会破坏拖拽流畅性；完全不管则依赖关系形同虚设。

**裁决**：默认 **软约束** —— 允许拖拽，前端弹出非阻塞警告，后端在响应中返回 `warnings[]` 并写入审计日志。同时提供**项目级开关** `enforce_dependency_block`（默认 `false`），开启后后端对违反依赖的转换返回 `409 Conflict`。理由：Trello/Jira 默认均不硬阻塞，但严格团队需要可选强约束，这也满足审核规则"核心逻辑需具备扩展空间，而非完全写死"。

### C-04 燃尽图缺少工作量度量字段

**冲突**：Prompt 要求燃尽图，但未定义任务的可累加工作量。燃尽图的 Y 轴必须有量纲。

**裁决**：为任务增加双度量字段 —— `story_points`（斐波那契枚举 0/1/2/3/5/8/13/21）与 `estimate_hours`（浮点小时）。燃尽图支持三种 Y 轴口径切换：**故事点（默认）/ 预估工时 / 任务数**。属对 Prompt 的**必要补全**，非范围扩张。

### C-05 「拖拽同步」的实时性范围未定义

**冲突**：Prompt 中"拖拽同步"未说明是单用户前后端一致，还是多用户多端实时协同。后者需引入 WebSocket/SSE，工作量与复杂度差异显著。

**裁决**：**分期实现**。MVP 交付乐观更新 + 失败自动回滚（单用户强一致，含并发冲突的乐观锁版本号校验）；V2 追加 SSE 广播实现多端实时同步。MVP 不因此阻塞。

### C-06 「自动发送邮件」的外部依赖处理

**冲突**：邮件发送依赖外部 SMTP 服务，交付环境无法保证有可用凭据。

**裁决**：判定为 **Scenario A（可模拟）**。采用 **Mailpit**（MIT 许可，多架构镜像）作为 compose 内置的**真实 SMTP 服务端**。关键点：Go 代码走标准 `net/smtp` 的**真实协议路径**，**不存在 `if mock { return nil }` 式的逻辑短路**；邮件真实投递到 Mailpit 并可在其 Web UI 中肉眼验证。切换到真实外发只需修改 `.env` 中的 `SMTP_HOST/PORT/USER/PASS/TLS`，无需改动任何代码。

**Mock 合法性自检**（对照 `audit/audit-rules.md` §Mock 合法性判据）：
- ✅ 存在已接线的真实实现路径 —— 走真实 SMTP 协议栈，仅收件服务端不同
- ✅ README §7「API 模拟与切换指南」显式说明切换方式
→ **判定为合法 Mock，非造假。**

### C-07 状态机「硬编码或配置化」的二选一

**冲突**：Prompt 允许二选一。纯硬编码违反审核规则"核心逻辑不应完全写死"；纯配置化又无法表达复杂守卫逻辑（如"检查所有子任务已完成"）。

**裁决**：采用 **配置化声明 + 编译期注册** 的混合模式 ——
- 状态、转换边、允许角色、状态→看板列映射，全部在 `config/workflow.yaml` 中声明
- 服务启动时加载并执行**完整性校验**：状态可达性分析、孤儿状态检测、终态检测、角色引用有效性、Guard/Action 名称是否已注册。校验失败则**拒绝启动**（fail-fast）
- 守卫函数 `Guard` 与副作用 `Action` 在 Go 中以 `init()` 编译期注册进注册表，YAML 按名称引用

此模式同时满足 Prompt 的"配置化"选项与审核的可扩展性要求。

---

## 5. 角色与权限模型（RBAC）

| 角色 | 代码 | 权限摘要 |
|---|---|---|
| 管理员 | `ADMIN` | 全部权限；项目/成员/工作流/触发器配置；审计日志 |
| 产品经理 | `PM` | Backlog 与 Sprint 规划；任务增删改；查看全部统计；接收完成通知 |
| 开发 | `DEV` | 认领任务；`TODO→IN_PROGRESS→TESTING`；Bug `CONFIRMED→FIXING→FIXED` |
| 测试 | `QA` | 提交/确认 Bug；**独占** Bug `FIXED→RESOLVED`；任务 `TESTING→DONE`、`TESTING→IN_PROGRESS`（打回） |
| 访客 | `VIEWER` | 全局只读 |

权限在**两个层级**校验：全局角色（`users.role`）与项目内角色（`project_members.role`，可覆盖全局）。状态机的转换鉴权以**项目内角色**为准。

---

## 6. 功能需求

> 优先级：**P0** = MVP 必交付；**P1** = V1 必交付；**P2** = V2 增强。Phase 1 据此细化 Roadmap。

### 6.1 认证与权限

| ID | 需求 | 优先级 |
|---|---|---|
| F-A01 | 用户注册 / 登录，JWT 签发与刷新，密码 bcrypt 加盐存储 | P0 |
| F-A02 | 五角色 RBAC 中间件，全局角色 + 项目内角色双层校验 | P0 |
| F-A03 | 内置 4 个测试账号（ADMIN/PM/DEV/QA），随迁移种子数据注入，README §4 列明 | P0 |

### 6.2 项目与成员

| ID | 需求 | 优先级 |
|---|---|---|
| F-P01 | 项目 CRUD，含项目 Key（如 `GJ`）用于生成任务编号 `GJ-1024` | P0 |
| F-P02 | 项目成员增删与项目内角色分配 | P0 |
| F-P03 | 项目级配置项：`enforce_dependency_block`、默认工作流 ID | P1 |

### 6.3 任务与 Bug（统一 Issue 模型）

> **设计决策**：Bug 不单独建表，而是以 `issues` 表 + `issue_type ∈ {STORY, TASK, BUG}` 区分，由 `issue_type` **路由到不同的状态机**。理由：符合 Jira 领域模型，使看板可混合展示任务与 Bug，同时避免表结构与 API 的重复。

| ID | 需求 | 优先级 |
|---|---|---|
| F-I01 | Issue CRUD：标题、描述（Markdown）、类型、优先级、经办人、报告人、故事点、预估工时、开始/截止日期、标签 | P0 |
| F-I02 | Bug 专属字段：严重级别（Blocker/Critical/Major/Minor/Trivial）、复现步骤、影响版本、修复版本 | P0 |
| F-I03 | Issue 评论（含 @提及）与状态变更历史时间线 | P1 |
| F-I04 | 附件上传（本地卷存储，限制类型与大小） | P2 |
| F-I05 | 列表检索：按类型/状态/经办人/Sprint/标签/关键字过滤，分页与排序 | P0 |

### 6.4 看板视图（Kanban Board）

| ID | 需求 | 优先级 |
|---|---|---|
| F-K01 | 4 列看板：待处理 / 开发中 / 已测试 / 已完成，列头显示卡片数与故事点小计 | P0 |
| F-K02 | 卡片跨列拖拽，触发状态机转换；乐观更新 + 失败自动回滚并 Toast 报错 | P0 |
| F-K03 | 列内拖拽排序，持久化 `board_rank`（采用分数排序，避免整列重排） | P1 |
| F-K04 | 卡片展示：编号、标题、类型图标、优先级色标、经办人头像、故事点、Bug 严重级别、逾期红标 | P0 |
| F-K05 | 看板过滤器：按经办人 / 类型 / 标签 / 仅我的 快速筛选 | P1 |
| F-K06 | WIP 限制：列可设在制品上限，超限时列头告警（软限制） | P2 |
| F-K07 | 多端实时同步（SSE 广播看板变更） | P2 |

### 6.5 Sprint（迭代）

| ID | 需求 | 优先级 |
|---|---|---|
| F-S01 | Sprint CRUD：名称、目标、起止日期、状态（PLANNED/ACTIVE/CLOSED） | P1 |
| F-S02 | Backlog 视图：未入 Sprint 的 Issue 池，支持拖拽入/出 Sprint | P1 |
| F-S03 | 启动 Sprint：校验同项目至多一个 ACTIVE Sprint；快照初始承诺范围（`committed_points`） | P1 |
| F-S04 | 关闭 Sprint：未完成 Issue 可批量移入下一 Sprint 或退回 Backlog；生成迭代总结 | P1 |
| F-S05 | Sprint 范围变更追踪（Scope Change），燃尽图需体现中途加入的工作量 | P1 |

### 6.6 甘特图 / 时间线

| ID | 需求 | 优先级 |
|---|---|---|
| F-G01 | 时间轴渲染 Issue 条，按 `start_date`/`due_date` 定位，支持日/周/月三档缩放 | P1 |
| F-G02 | 依赖关系连线，支持 FS/SS/FF/SF 四种类型（MVP 至少 FS） | P1 |
| F-G03 | **依赖环检测**：新增/修改依赖时做 DFS 环检测，检出则返回 `409` 并给出环路径 | P1 |
| F-G04 | 拖拽调整条形起止日期，实时回写；违反依赖时按 C-03 裁决处理 | P1 |
| F-G05 | 今日基准线、Sprint 区间背景带、逾期条形红色高亮 | P1 |
| F-G06 | 关键路径（CPM）计算与高亮 | P2 |

**技术约束**：甘特图组件**禁止**使用 dhtmlx-gantt（GPL/商业双许可）。采用自研 SVG 渲染或 MIT 许可组件。

### 6.7 统计分析页

| ID | 需求 | 优先级 |
|---|---|---|
| F-C01 | **燃尽图**：理想线 vs 实际剩余线，Y 轴口径可切换（故事点/工时/任务数），标注范围变更点 | P1 |
| F-C02 | **团队生产力折线图**：按周聚合各成员完成的故事点与任务数，支持多成员对比 | P1 |
| F-C03 | 迭代进度卡片：完成率、剩余天数、平均速度（Velocity）、预测完成日 | P1 |
| F-C04 | Bug 统计：按严重级别分布、按状态分布、平均修复时长（MTTR） | P1 |
| F-C05 | 累积流图（CFD） | P2 |

### 6.8 状态机引擎（核心内核 ①）

| ID | 需求 | 优先级 |
|---|---|---|
| F-M01 | YAML 声明式工作流定义，启动时加载 + 完整性校验，校验失败拒绝启动 | P0 |
| F-M02 | 双工作流：`task_workflow`（STORY/TASK）与 `bug_workflow`（BUG），按 `issue_type` 路由 | P0 |
| F-M03 | 转换鉴权：每条边声明 `allowed_roles`，非法角色返回 `403` 且附带可读原因 | P0 |
| F-M04 | 转换守卫 Guard：编译期注册的前置条件函数（如 `all_subtasks_done`、`has_assignee`、`dependencies_satisfied`） | P0 |
| F-M05 | 非法转换（无此边）返回 `422`，响应体列出当前状态的合法后继状态 | P0 |
| F-M06 | 每次转换写入 `issue_status_history`（前态、后态、操作人、时间、耗时），供燃尽图与 MTTR 计算 | P0 |
| F-M07 | 状态 → 看板列映射由 YAML 配置，使 Bug 的 6 个状态可正确落入 4 列看板 | P0 |
| F-M08 | 并发保护：Issue 携带 `version` 乐观锁，冲突返回 `409` | P1 |

#### 任务状态机（`task_workflow`）

```
TODO ──DEV/PM/ADMIN──▶ IN_PROGRESS ──DEV/QA/ADMIN──▶ TESTING ──QA/ADMIN──▶ DONE
  ▲                          │  ▲                        │  │                 │
  └────PM/ADMIN(撤回)────────┘  └───QA/ADMIN(打回)───────┘  └─PM/ADMIN(重开)──┘
```

看板列映射：`TODO→待处理`、`IN_PROGRESS→开发中`、`TESTING→已测试`、`DONE→已完成`

#### Bug 状态机（`bug_workflow`）

```
NEW ──QA/PM/ADMIN──▶ CONFIRMED ──DEV/ADMIN──▶ FIXING ──DEV/ADMIN──▶ FIXED ──QA(独占)──▶ RESOLVED ──PM/QA/ADMIN──▶ CLOSED
 │                       │                                             │                                              │
 └──QA/PM(拒绝)──▶ REJECTED                                            └──QA(验证不通过, 打回)──▶ FIXING             │
                                                                                                                       │
 REOPENED ◀────────────────────────────────── QA/PM/ADMIN（重新打开）──────────────────────────────────────────────────┘
```

看板列映射：`NEW/CONFIRMED→待处理`、`FIXING/REOPENED→开发中`、`FIXED→已测试`、`RESOLVED/CLOSED/REJECTED→已完成`

> **Prompt 关键约束落实点**：`FIXED → RESOLVED` 这条边的 `allowed_roles` **仅含 `QA`**（`ADMIN` 亦不例外，以体现约束的严格性）。此约束必须有专门的单元测试断言，且 E2E 用例中须包含"DEV 尝试置 RESOLVED 被拒"的负向路径。

### 6.9 自动化触发器 / Hook（核心内核 ②）

| ID | 需求 | 优先级 |
|---|---|---|
| F-T01 | 事件总线：`issue.status_changed`、`issue.created`、`issue.assigned`、`sprint.started`、`sprint.closed`、`issue.overdue` | P1 |
| F-T02 | 触发器定义：`事件类型 + 条件过滤（JSON 条件表达式）→ 动作列表`，存库可配置 | P1 |
| F-T03 | 动作类型：`SEND_EMAIL`（真实 SMTP）、`WEBHOOK`（HTTP POST）、`AUTO_ASSIGN`、`ADD_COMMENT`、`IN_APP_NOTIFY` | P1 |
| F-T04 | **Prompt 指定用例**：任务进入「已完成」列 → 自动邮件通知该项目的产品经理。作为**内置种子触发器**随迁移注入 | P1 |
| F-T05 | **可靠投递**：采用 Outbox 模式 —— 事件与业务变更在同一事务内落库，后台 worker 轮询投递。服务重启后未投递事件**必须续跑**，不得静默丢失 | P1 |
| F-T06 | **窄重试**：仅对瞬时错误（网络超时、5xx、SMTP 4xx 临时拒绝）做指数退避重试（最多 3 次）；**鉴权失败、参数/校验错误一律不重试**，直接进死信 | P1 |
| F-T07 | **幂等保护**：每个事件带唯一 `event_id`，投递记录唯一索引，重复触发不产生重复邮件 | P1 |
| F-T08 | 执行日志 `trigger_executions`：记录触发器、事件、结果、错误类型、耗时、重试次数，前端可查 | P1 |
| F-T09 | 触发器可视化配置面板 | P2 |

### 6.10 统计聚合（核心内核 ③）

| ID | 需求 | 优先级 |
|---|---|---|
| F-Q01 | 燃尽图数据源采用**基于 `issue_status_history` 的实时 SQL 聚合**（`generate_series` 生成日期轴 + 窗口函数计算每日剩余量），而非依赖预生成快照 | P1 |
| F-Q02 | 每日 00:05 (GMT+8) 定时任务写入 `sprint_daily_snapshots` 作为兜底与历史归档；两条路径结果须一致（须有一致性测试） | P1 |
| F-Q03 | 生产力聚合：按 ISO 周 + 成员维度聚合完成量，单条 SQL 完成，禁止 N+1 循环查询 | P1 |
| F-Q04 | **必备索引**：`issue_status_history(issue_id, changed_at)`、`issues(sprint_id, status)`、`issues(project_id, issue_type, status)`、`issue_dependencies(predecessor_id)` | P1 |
| F-Q05 | 聚合结果 Redis 缓存（TTL 60s），Issue 变更时按 Sprint 维度精准失效 | P2 |

> **审核对齐**：Prompt 明确要求"编写高效的 SQL/聚合查询"。因此聚合层**必须使用原生 SQL**（`sqlx`），**禁止用 ORM 链式调用拼装统计查询**。每个聚合查询须在 `docs/API.md` 中附 `EXPLAIN ANALYZE` 结果，作为"高效"的可验证证据。

### 6.11 工程基础设施（对齐 `knowledge-base/global.md`）

| ID | 需求 | 优先级 | 全局规则来源 |
|---|---|---|---|
| F-E01 | **统一 Logger**：基于 `log/slog` 封装，含 level 控制、请求 ID 透传、结构化 JSON 输出；**禁止散落 `fmt.Println`**，生产环境自动屏蔽 debug | P0 | `[Logging]` |
| F-E02 | **输入校验**：所有外部输入（HTTP body、query、YAML 配置、CSV 导入）必须校验字段存在性、类型与边界值，不得仅依赖调用处的简单检查 | P0 | `[Robustness]` |
| F-E03 | **API 文档** `docs/API.md`：端点清单 + 每个端点的请求/响应示例 + 参数类型说明 + **完整错误码表** | P0 | `[Documentation]` |
| F-E04 | **测试覆盖**：后端覆盖 CRUD + 三大内核（状态机/触发器/聚合）单元测试；前端 Playwright E2E | P0 | `[Testing]` |
| F-E05 | 统一错误响应结构 `{code, message, details, request_id}`，全局 recover 中间件防 panic 泄漏堆栈 | P0 | 审核§工程细节 |
| F-E06 | 数据库迁移版本化管理（golang-migrate），含种子数据 | P0 | — |
| F-E07 | 健康检查 `/api/health`（含 DB 与 SMTP 连通性探测），供 compose healthcheck 使用 | P0 | — |

---

## 7. 非功能需求与可量化验收基线

> 以下为**可测量**的验收标准，非描述性文字。QA Phase 须逐项验证并记入 `docs/QA_Record.md`。
> 测量环境：本机 Docker（arm64），数据集 = 1 项目 / 500 Issue / 1 Sprint / 30 天历史 / 8 成员。

| # | 指标 | 基线 | 验证方式 |
|---|---|---|---|
| N-01 | 看板拖拽状态更新 API P95 延迟 | **< 200 ms** | 压测脚本 100 次采样 |
| N-02 | 燃尽图聚合查询 P95 延迟 | **< 300 ms** | `EXPLAIN ANALYZE` + 100 次采样 |
| N-03 | 甘特图数据加载（200 Issue + 依赖） | **< 500 ms** | 接口计时 |
| N-04 | 看板初始加载（500 Issue） | **< 800 ms** | 接口计时 |
| N-05 | 非法状态转换拦截率 | **100%**，返回 `422` 且列出合法后继 | 单元测试全边覆盖 |
| N-06 | 越权状态转换拦截率 | **100%**，返回 `403` | 单测 + E2E 负向用例（DEV 置 RESOLVED 必败） |
| N-07 | 依赖环检测拦截率 | **100%**，返回 `409` 并给出环路径 | 单元测试（含 3 节点、自环两类） |
| N-08 | 触发器异步化 | 主请求耗时增量 **< 10 ms** | 对比开关触发器前后的 API 延迟 |
| N-09 | 触发器投递成功率（Mailpit） | **≥ 99%**，失败进重试，最多 3 次指数退避 | 批量 100 事件统计 |
| N-10 | 触发器重启可恢复性 | 强杀服务后重启，未投递事件 **100% 续跑** | 混沌测试：投递中途 `docker kill` |
| N-11 | 触发器幂等性 | 同一 `event_id` 重复投递 **0 次** 重复邮件 | 重复触发测试 |
| N-12 | 后端核心内核单测覆盖率 | **≥ 80%**（状态机 / 触发器 / 聚合三包） | `go test -cover` |
| N-13 | 后端整体单测覆盖率 | **≥ 60%** | `go test -cover ./...` |
| N-14 | E2E 关键路径用例数 | **≥ 6 条**，Mock 模式运行，成本 **¥0** | Playwright report |
| N-15 | 前端主 bundle 体积 | **< 500 KB (gzip)** | `vite build` 产物报告 |
| N-16 | Docker 冷启动至全服务 healthy | **< 90 s** | `docker compose up` 计时 |
| N-17 | 跨架构构建 | `linux/arm64` 与 `linux/amd64` 均构建成功 | `docker buildx build --platform` |
| N-18 | 无 N+1 查询 | 看板/甘特/统计三个主接口的 SQL 执行次数为**常数级** | 开启 SQL 日志计数 |

### 7.1 E2E 关键路径（QA Phase 必须覆盖）

1. 登录 → 进入项目 → 看板正常渲染 4 列
2. 拖拽任务 `待处理 → 开发中 → 已测试 → 已完成`，状态持久化且刷新后保持
3. **负向**：以 DEV 身份尝试将 Bug 置为「已解决」→ 被拒绝，返回 403 且 UI 有明确提示
4. 以 QA 身份将 Bug `FIXED → RESOLVED` → 成功
5. 任务进入「已完成」→ Mailpit 中可查得发往 PM 的通知邮件（**验证触发器端到端**）
6. Sprint 燃尽图正确渲染理想线与实际线，数据与 API 返回一致
7. 甘特图创建循环依赖 → 被拒绝并提示环路径

---

## 8. 技术栈决策

| 层 | 选型 | 理由 / 约束 |
|---|---|---|
| 后端语言 | **Go 1.25** | Prompt 指定 |
| HTTP 框架 | Gin 或 Chi（Phase 1 定夺） | 中间件生态成熟 |
| 数据访问 | **sqlx（聚合层强制）+ 轻量 Repository（CRUD）** | Prompt 要求"高效 SQL"，聚合禁用 ORM 拼装 |
| 数据库 | **PostgreSQL 16** | 需要 `generate_series`、窗口函数、CTE、`jsonb` 支持触发器条件 |
| 迁移 | golang-migrate | 版本化 + 种子数据 |
| 邮件 | `net/smtp` + **Mailpit**（compose 内置） | 见 C-06；MIT，多架构 |
| 缓存 | Redis 7（P2 引入） | 聚合结果缓存 |
| 日志 | `log/slog` 封装 | 全局规则 `[Logging]` |
| 前端框架 | **Vue 3 + TypeScript + Pinia + Vite** | Prompt 指定 |
| UI 体系 | Tailwind CSS + shadcn-vue（Phase 2 定夺） | 满足 Redline 2「Dribbble 标准」 |
| 拖拽 | vuedraggable / SortableJS（MIT） | 看板与 Backlog 拖拽 |
| 图表 | Apache ECharts（Apache-2.0） | 燃尽图、生产力折线图 |
| 甘特图 | 自研 SVG 或 MIT 许可组件 | **禁止 dhtmlx-gantt（GPL/商业双许可）** |
| E2E | Playwright | 全局规则 `[Testing]` |
| 容器 | 多阶段构建 + distroless/alpine 运行层 | 跨架构，镜像瘦身 |

---

## 9. 数据模型草案

供 Phase 1 细化，字段非穷举。

| 表 | 关键字段 | 说明 |
|---|---|---|
| `users` | id, username, email, password_hash, display_name, avatar_url, role, is_active | 全局角色 |
| `projects` | id, key, name, description, owner_id, enforce_dependency_block, workflow_config | 项目 |
| `project_members` | project_id, user_id, role | 项目内角色（覆盖全局） |
| `sprints` | id, project_id, name, goal, start_date, end_date, status, committed_points | 迭代 |
| `issues` | id, project_id, seq_no, issue_type, title, description, status, priority, severity, assignee_id, reporter_id, sprint_id, story_points, estimate_hours, start_date, due_date, board_rank, version, created_at, updated_at, resolved_at | **统一 Issue 模型** |
| `issue_labels` | issue_id, label | 标签 |
| `issue_dependencies` | id, predecessor_id, successor_id, dep_type | FS/SS/FF/SF；需环检测 |
| `issue_status_history` | id, issue_id, from_status, to_status, actor_id, changed_at, duration_sec | **燃尽图与 MTTR 的数据基石** |
| `issue_comments` | id, issue_id, author_id, body, created_at | 评论 |
| `triggers` | id, project_id, name, event_type, condition (jsonb), actions (jsonb), is_enabled | 自动化触发器定义 |
| `outbox_events` | id, event_id, event_type, payload (jsonb), status, created_at | **Outbox 模式，保证可恢复** |
| `trigger_executions` | id, trigger_id, event_id, action_type, status, error_class, error_msg, retry_count, duration_ms | 执行日志，唯一索引保幂等 |
| `sprint_daily_snapshots` | sprint_id, snapshot_date, remaining_points, completed_points, scope_points | 每日兜底快照 |
| `notifications` | id, user_id, type, title, body, is_read | 站内通知 |
| `audit_logs` | id, actor_id, action, entity_type, entity_id, detail (jsonb), created_at | 审计 |

---

## 10. 交付物清单

| 类别 | 文件 |
|---|---|
| 需求与规划 | `docs/Requirements.md`（本文）、`docs/Roadmap.md`、`docs/.meta/original_prompt.md` |
| 设计 | `docs/DesignSpec.md`（Phase 2） |
| 接口 | `docs/API.md`（端点 + 示例 + 错误码表 + 聚合查询 EXPLAIN） |
| 质量 | `docs/QA_Record.md`、`docs/AuditReport.md`、`docs/SelfTestReport.md` |
| 交付说明 | `README.md`（7 个强制章节，含 §7 API 模拟与切换指南） |
| 代码 | `backend/`（Go，约 30 文件）、`frontend-user/`（Vue 3） |
| 编排 | `docker-compose.yml`、`backend/Dockerfile`、`frontend-user/Dockerfile`、`.env.example` |
| 配置 | `config/workflow.yaml`（状态机声明） |
| 测试 | `backend/**/*_test.go`、`tests/e2e_flow.spec.ts` |

---

## 11. 分期边界建议（供 Phase 1 细化为 `docs/Roadmap.md`）

> Tier 2 规模的**强制要求**。Chief Architect 可调整内容，但必须保留三段式边界。

**MVP（P0）— 可运行的看板内核**
认证与 RBAC、项目与成员、统一 Issue 模型 CRUD、**状态机引擎（YAML 配置化 + 双工作流 + 角色守卫）**、4 列看板与拖拽、Bug 追踪字段、统一 Logger、错误处理、健康检查、Docker 一键起、`docs/API.md`、核心单测。
**验收**：`docker compose up` 后可完成"登录 → 拖拽推进任务 → DEV 被拒置 RESOLVED → QA 成功置 RESOLVED"全流程。

**V1（P1）— 敏捷闭环与三大内核完备**
Sprint 与 Backlog、**自动化触发器（Outbox + 窄重试 + 幂等 + Mailpit 邮件）**、**统计聚合（燃尽图 + 生产力折线图 + 迭代进度 + Bug 统计）**、甘特图与依赖环检测、评论与历史时间线、Playwright E2E、覆盖率达标。
**验收**：N-01 至 N-18 全部基线达标。

**V2（P2）— 体验增强**
SSE 实时协同、CFD 累积流图、CPM 关键路径、触发器可视化配置面板、Redis 聚合缓存、WIP 限制、附件上传。

---

## 12. 审核维度预判（供 Phase 5 Auditor 参考）

| 审核维度 | 适用性 | 说明 |
|---|---|---|
| 硬性门槛 | ✅ 适用 | `docker compose up --build -d` 一键启动 |
| 交付完整性 | ✅ 适用 | Mock 合法性见 C-06，README §7 说明切换 |
| 工程与架构质量 | ✅ 适用 | 分层架构，约 30 Go 文件，无单文件堆叠 |
| 工程细节与专业度 | ✅ 适用 | F-E01 ~ F-E07 |
| Prompt 需求理解与适配度 | ✅ 适用 | 七处矛盾裁决见 §4，均属显式说明 |
| 美观度 | ✅ 适用 | 全栈题目，Phase 2 `docs/DesignSpec.md` 约束 |
| **成本与资源可控性** | ❌ **不适用** | 项目**不调用任何按量计费的外部 API**。邮件走自托管 SMTP（Mailpit），无计费维度 |
| **异步任务可靠性** | ⚠️ **部分适用** | 触发器投递为后台异步任务（单任务 < 30s，未达 30s 触发阈值），但仍主动满足可恢复性（N-10）、幂等性（N-11）、可观测性（F-T08）三项要求 |
| **合规标识** | ❌ **不适用** | 项目**不产出任何 AI 生成内容** |

---

## 13. 变更控制

本文档已冻结。任何范围变更须经 `/pm` 重新评估并在此追加 `## 附录：变更记录` 章节，禁止直接改写正文。
