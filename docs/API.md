# GoJira API 文档

基址：`http://localhost:18232`（开发随机端口）或经前端反代 `http://localhost:18231/api`。
前缀：`/api/v1`。时区：GMT+8，时间字段 ISO-8601。

## 1. 通用约定

### 成功

```json
{ "data": { }, "meta": { "page": 1, "per_page": 20, "total": 0 } }
```

### 错误

```json
{
  "code": "FORBIDDEN",
  "message": "只有测试人员能将 Bug 置为已解决",
  "details": { "from": "FIXED", "to": "RESOLVED", "allowed_roles": ["QA"] },
  "request_id": "b7c1e2a0"
}
```

### 错误码表

| HTTP | code | 含义 |
|---|---|---|
| 400 | BAD_REQUEST | JSON 无法解析 |
| 401 | UNAUTHORIZED | 缺少或过期 JWT |
| 403 | FORBIDDEN | 已登录但角色不允许（含 DEV→RESOLVED） |
| 404 | NOT_FOUND | 资源不存在 |
| 409 | CONFLICT | 乐观锁冲突 / 依赖环 / 硬依赖未满足 / 已有 ACTIVE Sprint |
| 422 | UNPROCESSABLE | 无此转换边、Guard 失败、字段校验失败 |
| 500 | INTERNAL | 未预期错误（无堆栈泄漏） |

Header：`Authorization: Bearer <access>`，`X-Request-ID` 原样回传。

---

## 2. 认证

### POST /api/v1/auth/login

请求：

```json
{ "username": "qa", "password": "Qa@123456" }
```

响应 200：

```json
{
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "user": { "id": 4, "username": "qa", "display_name": "QA Tester", "role": "QA" }
  }
}
```

### POST /api/v1/auth/register

`{username, password, email, display_name}` → 201 + 同上。密码 ≥ 8。

### POST /api/v1/auth/refresh

`{refresh_token}` → 新 access。

### GET /api/v1/auth/me

当前用户。

---

## 3. 项目与成员

### GET /api/v1/projects

当前用户加入的项目列表。

### POST /api/v1/projects

`{key, name, description}`，`key` 2–8 大写字母。201。

### GET /api/v1/projects/:id

含 `enforce_dependency_block`、成员摘要。

### PATCH /api/v1/projects/:id

ADMIN/PM：`{name, description, enforce_dependency_block}`。

### GET /api/v1/projects/:id/members

### POST /api/v1/projects/:id/members

`{user_id, role}`。

### DELETE /api/v1/projects/:id/members/:user_id

---

## 4. Issue

### GET /api/v1/projects/:id/issues

Query：`type, status, assignee_id, sprint_id, q, page, per_page, sort`。

### POST /api/v1/projects/:id/issues

```json
{
  "issue_type": "BUG",
  "title": "登录按钮无响应",
  "description": "复现：点击后无跳转",
  "priority": "HIGH",
  "severity": "MAJOR",
  "assignee_id": 3,
  "sprint_id": 1,
  "story_points": 3,
  "estimate_hours": 4,
  "start_date": "2026-08-21",
  "due_date": "2026-08-25",
  "labels": ["auth"]
}
```

编号 `GJ-12` 由 `project.key + seq_no` 生成。

### GET /api/v1/issues/:id

含 labels、合法后继状态 `legal_transitions[]`。

### PATCH /api/v1/issues/:id

字段更新（不含 status）。带 `version`。

### POST /api/v1/issues/:id/transition

```json
{ "to": "RESOLVED", "version": 2 }
```

- 无边：422，`details.legal` 列出后继
- 角色不符：403
- 版本冲突：409
- 成功：新 issue + `warnings`（软依赖）

### GET /api/v1/issues/:id/history

状态历史，含 duration_sec。

### GET|POST /api/v1/issues/:id/comments

`{body}`。

### GET|POST /api/v1/issues/:id/dependencies

`{predecessor_id, dep_type}`，`dep_type` 默认 FS。环 → 409 + `details.cycle`。

### DELETE /api/v1/issues/:id/dependencies/:dep_id

---

## 5. 看板

### GET /api/v1/projects/:id/board

```json
{
  "data": {
    "columns": [
      {
        "id": "TODO",
        "label": "待处理",
        "hint": "",
        "count": 3,
        "points": 8,
        "issues": []
      }
    ]
  }
}
```

Query：`assignee_id, type, mine=1`。

### PATCH /api/v1/issues/:id/rank

`{board_rank, version}` 列内排序。

---

## 6. Sprint

### GET|POST /api/v1/projects/:id/sprints

创建：`{name, goal, start_date, end_date}`。

### POST /api/v1/sprints/:id/start

同项目仅一个 ACTIVE；写入 `committed_points`。

### POST /api/v1/sprints/:id/close

`{move_to: "backlog"|"next", next_sprint_id?}`。

### POST /api/v1/sprints/:id/issues

`{issue_id}` 入列。DELETE 同路径 `?issue_id=` 出列。

---

## 7. 甘特

### GET /api/v1/projects/:id/gantt

```json
{
  "data": {
    "issues": [{ "id": 1, "key": "GJ-1", "title": "...", "start_date": "2026-08-21", "due_date": "2026-08-28", "status": "IN_PROGRESS" }],
    "dependencies": [{ "id": 1, "predecessor_id": 1, "successor_id": 2, "dep_type": "FS" }]
  }
}
```

---

## 8. 统计

### GET /api/v1/sprints/:id/burndown?metric=points|hours|count

```json
{
  "data": {
    "metric": "points",
    "ideal": [{ "date": "2026-08-17", "value": 40 }],
    "actual": [{ "date": "2026-08-17", "value": 40 }],
    "scope_changes": [{ "date": "2026-08-20", "delta": 5 }]
  }
}
```

实现：`generate_series` + `issue_status_history` 窗口聚合，见 `backend/internal/stats/queries.go`。EXPLAIN 在 QA 阶段附到 `docs/QA_Record.md`。

### GET /api/v1/projects/:id/velocity

按 ISO 周、成员聚合完成量。

### GET /api/v1/sprints/:id/progress

`{completion_rate, remaining_days, velocity, predicted_end}`。

### GET /api/v1/projects/:id/bug-stats

`{by_severity, by_status, mttr_hours}`。

---

## 9. 触发器

### GET /api/v1/projects/:id/triggers

### GET /api/v1/projects/:id/trigger-executions?page=1

---

## 10. 健康检查

### GET /api/health

```json
{ "status": "ok", "db": "ok", "smtp": "ok", "time": "2026-08-21T20:10:00+08:00" }
```

DB 或 SMTP 失败时 503，`status=degraded`。
