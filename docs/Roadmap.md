# GoJira 路线图

| 项 | 值 |
|---|---|
| 状态 | ACTIVE |
| 版本 | v1.0 |
| 依据 | `docs/Requirements.md`（已冻结） |
| 规模 | Tier 2（≈ 12,000–14,300 LoC）→ **强制三段边界** |
| 阶段顺序 | **UI-First（默认）** |

---

## 阶段顺序决策

**选择：UI-First（不交换 Phase 2 / Phase 3）。**

理由：主界面是看板 / Backlog / 统计仪表盘，可对照 Requirements §9 的 schema 草图独立构建；甘特图是既有 Issue 起止与依赖的可视化，不是「组件结构由数据模型派生」的编辑器。后端契约以本文件 §API 草图为准，Phase 3 接线。

---

## 技术决策（Architect 定夺）

| 项 | 决策 |
|---|---|
| HTTP | **Chi v5**（轻量、与 `net/http` 兼容，避免 Gin 反射绑定掩盖校验） |
| 数据访问 | CRUD 用 `sqlx`；统计聚合 **强制手写 SQL**（`stats/queries.go`） |
| JWT | HS256，access 2h + refresh 7d，密钥来自环境变量 |
| 前端 | Vue 3 + TS + Pinia + Vue Router + Vite + Tailwind 3 |
| 拖拽 | `vuedraggable@next`（MIT / SortableJS） |
| 图表 | Apache ECharts 5 |
| 甘特 | **自研 SVG**（禁止 dhtmlx-gantt） |
| UI 角色 | 单应用 `frontend-user`，ADMIN/PM/DEV/QA 以 RBAC 门控；`frontend-admin` / `frontend-mp` 为结构占位（非小程序、非独立后台） |
| 开发端口 | 前端 **18231**、API **18232**、Mailpit UI **18233**、Postgres 主机映射 **18234**（探测空闲） |

---

## MVP / V1 / V2 边界（强制）

### MVP（P0）— 可运行看板内核

| 任务 ID | 内容 | 状态 |
|---|---|---|
| ARCH-01 | Git / .gitignore / 目录骨架 / compose 随机端口 | [x] |
| ARCH-02 | `config/workflow.yaml` 双工作流 + 列映射 | [x] |
| UI-01 | DesignSpec + 设计 token + 布局壳 | [x] |
| UI-02 | 登录页 + 项目列表 + 看板 4 列 + 卡片 + 拖拽 | [x] |
| UI-03 | Issue 抽屉（创建/编辑任务与 Bug） | [x] |
| BE-01 | Chi 路由、JWT、RBAC、统一 Logger、错误信封 | [x] |
| BE-02 | 迁移 + 种子账号 + 演示项目数据 | [x] |
| BE-03 | 状态机引擎（加载/校验/转换/历史/乐观锁） | [x] |
| BE-04 | Issue / Project / Member CRUD + 看板接口 | [x] |
| BE-05 | Bug `FIXED→RESOLVED` 仅 QA；DEV 返回 403 | [x] |
| BE-06 | 多阶段 Dockerfile，`docker compose up` 可访问 | [x] |
| DOC-01 | `docs/API.md` 端点 + 示例 + 错误码 | [x] |

**MVP 验收**：登录 → 拖拽推进任务 → DEV 置 RESOLVED 被拒 → QA 成功置 RESOLVED。

### V1（P1）— 敏捷闭环与三大内核

| 任务 ID | 内容 | 状态 |
|---|---|---|
| UI-04 | Sprint / Backlog 拖拽入列 | [x] |
| UI-05 | 自研甘特图（日/周/月、FS 依赖、今日线、逾期） | [x] |
| UI-06 | 统计页：燃尽图 + 生产力折线 + 进度卡片 + Bug 分布 | [x] |
| UI-07 | 评论时间线、触发器执行日志（只读） | [x] |
| BE-07 | Sprint 生命周期 + 承诺快照 + 范围变更 | [x] |
| BE-08 | 依赖 CRUD + DFS 环检测 | [x] |
| BE-09 | 触发器 Outbox + SMTP + 窄重试 + 幂等 | [x] |
| BE-10 | 种子触发器：进入已完成列 → 邮件通知项目 PM | [x] |
| BE-11 | 燃尽 / 生产力 / 进度原生 SQL 聚合 | [x] |
| BE-12 | 每日 00:05 GMT+8 快照 worker | [x] |
| QA-01 | 状态机 / 触发器 / 聚合 / 环检测单测 | [x] |
| QA-02 | Playwright E2E ≥ 6 条，Mock 模式 ¥0 | [x] |

**V1 验收**：Requirements N-01–N-18 基线。本轮 `/auto` **交付到 V1**（P2 不实现，仅留扩展点）。

### V2（P2）— 明确不做（本轮）

SSE 实时协同、CFD、CPM、触发器可视化配置、Redis 缓存、WIP 硬限制、附件上传。代码中可留接口注释，禁止假装已实现。

---

## 目录结构

```
GoJira/
├── backend/                 # Go 服务（目标 ≈ 30 个 .go 文件）
│   ├── cmd/server/
│   ├── internal/{config,logger,platform,middleware,domain,
│   │            workflow,auth,project,issue,sprint,board,
│   │            gantt,trigger,stats,comment,router,seed,health}
│   ├── migrations/
│   └── Dockerfile
├── frontend-user/           # Vue 3 主应用
├── frontend-admin/          # 占位：能力并入 frontend-user RBAC
├── frontend-mp/             # 占位：非微信小程序，不适用
├── config/workflow.yaml
├── nginx/default.conf
├── tests/e2e_flow.spec.ts
└── docker-compose.yml
```

---

## API 草图（Phase 2/3 共用契约）

前缀 `/api/v1`。成功体 `{ data, meta? }`；错误体 `{ code, message, details, request_id }`。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /auth/register | 注册 |
| POST | /auth/login | 登录 |
| POST | /auth/refresh | 刷新 |
| GET | /auth/me | 当前用户 |
| GET/POST | /projects | 列表 / 创建 |
| GET/PATCH | /projects/:id | 详情 / 更新 |
| GET/POST/DELETE | /projects/:id/members | 成员 |
| GET/POST | /projects/:id/issues | 列表 / 创建 |
| GET/PATCH | /issues/:id | 详情 / 更新 |
| POST | /issues/:id/transition | 状态机转换 `{to, version}` |
| GET | /issues/:id/history | 状态历史 |
| GET/POST | /issues/:id/comments | 评论 |
| GET/POST/DELETE | /issues/:id/dependencies | 依赖 |
| GET | /projects/:id/board | 4 列看板数据 |
| PATCH | /issues/:id/rank | 列内排序 |
| GET/POST | /projects/:id/sprints | Sprint |
| POST | /sprints/:id/start\|close | 启停 |
| POST | /sprints/:id/issues | Backlog 入列 |
| GET | /projects/:id/gantt | 甘特条 + 依赖 |
| GET | /sprints/:id/burndown?metric= | 燃尽 |
| GET | /projects/:id/velocity | 生产力 |
| GET | /sprints/:id/progress | 迭代进度 |
| GET | /projects/:id/bug-stats | Bug 统计 |
| GET | /projects/:id/triggers | 触发器列表 |
| GET | /projects/:id/trigger-executions | 执行日志 |
| GET | /health | DB + SMTP 探测 |

---

## 种子账号（F-A03）

| 用户名 | 密码 | 角色 |
|---|---|---|
| admin | Admin@123 | ADMIN |
| pm | Pm@123456 | PM |
| dev | Dev@123456 | DEV |
| qa | Qa@123456 | QA |

演示项目 Key=`GJ`，含 ACTIVE Sprint、任务+Bug、FS 依赖、历史记录（供燃尽图）。
