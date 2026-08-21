SET timezone = 'Asia/Shanghai';

CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    display_name    TEXT NOT NULL DEFAULT '',
    avatar_url      TEXT,
    role            TEXT NOT NULL DEFAULT 'VIEWER',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE projects (
    id                         BIGSERIAL PRIMARY KEY,
    key                        TEXT NOT NULL UNIQUE,
    name                       TEXT NOT NULL,
    description                TEXT NOT NULL DEFAULT '',
    owner_id                   BIGINT NOT NULL REFERENCES users(id),
    enforce_dependency_block   BOOLEAN NOT NULL DEFAULT FALSE,
    workflow_config            TEXT NOT NULL DEFAULT 'default',
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE project_members (
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE sprints (
    id                BIGSERIAL PRIMARY KEY,
    project_id        BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    goal              TEXT NOT NULL DEFAULT '',
    start_date        DATE NOT NULL,
    end_date          DATE NOT NULL,
    status            TEXT NOT NULL DEFAULT 'PLANNED',
    committed_points  INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE issues (
    id              BIGSERIAL PRIMARY KEY,
    project_id      BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    seq_no          INT NOT NULL,
    issue_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL,
    priority        TEXT NOT NULL DEFAULT 'MEDIUM',
    severity        TEXT,
    reproduce_steps TEXT,
    affect_version  TEXT,
    fix_version     TEXT,
    assignee_id     BIGINT REFERENCES users(id),
    reporter_id     BIGINT NOT NULL REFERENCES users(id),
    sprint_id       BIGINT REFERENCES sprints(id),
    story_points    INT,
    estimate_hours  DOUBLE PRECISION,
    start_date      DATE,
    due_date        DATE,
    board_rank      DOUBLE PRECISION NOT NULL DEFAULT 0,
    version         INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    UNIQUE (project_id, seq_no)
);

CREATE TABLE issue_labels (
    issue_id  BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    label     TEXT NOT NULL,
    PRIMARY KEY (issue_id, label)
);

CREATE TABLE issue_dependencies (
    id              BIGSERIAL PRIMARY KEY,
    predecessor_id  BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    successor_id    BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    dep_type        TEXT NOT NULL DEFAULT 'FS',
    UNIQUE (predecessor_id, successor_id, dep_type)
);

CREATE TABLE issue_status_history (
    id            BIGSERIAL PRIMARY KEY,
    issue_id      BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    from_status   TEXT NOT NULL,
    to_status     TEXT NOT NULL,
    actor_id      BIGINT NOT NULL REFERENCES users(id),
    changed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_sec  INT NOT NULL DEFAULT 0
);

CREATE TABLE issue_comments (
    id          BIGSERIAL PRIMARY KEY,
    issue_id    BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id   BIGINT NOT NULL REFERENCES users(id),
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE triggers (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    condition   JSONB NOT NULL DEFAULT '{}'::jsonb,
    actions     JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE outbox_events (
    id               BIGSERIAL PRIMARY KEY,
    event_id         TEXT NOT NULL UNIQUE,
    event_type       TEXT NOT NULL,
    payload          JSONB NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending',
    retry_count      INT NOT NULL DEFAULT 0,
    error_class      TEXT,
    error_msg        TEXT,
    next_attempt_at  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trigger_executions (
    id            BIGSERIAL PRIMARY KEY,
    trigger_id    BIGINT NOT NULL REFERENCES triggers(id) ON DELETE CASCADE,
    event_id      TEXT NOT NULL,
    action_type   TEXT NOT NULL,
    status        TEXT NOT NULL,
    error_class   TEXT,
    error_msg     TEXT,
    retry_count   INT NOT NULL DEFAULT 0,
    duration_ms   INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_id, trigger_id, action_type)
);

CREATE TABLE sprint_daily_snapshots (
    sprint_id          BIGINT NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    snapshot_date      DATE NOT NULL,
    remaining_points   INT NOT NULL DEFAULT 0,
    completed_points   INT NOT NULL DEFAULT 0,
    scope_points       INT NOT NULL DEFAULT 0,
    PRIMARY KEY (sprint_id, snapshot_date)
);

CREATE TABLE notifications (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_logs (
    id           BIGSERIAL PRIMARY KEY,
    actor_id     BIGINT REFERENCES users(id),
    action       TEXT NOT NULL,
    entity_type  TEXT NOT NULL,
    entity_id    TEXT NOT NULL,
    detail       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_issue_status_history_issue_changed ON issue_status_history (issue_id, changed_at);
CREATE INDEX idx_issues_sprint_status ON issues (sprint_id, status);
CREATE INDEX idx_issues_project_type_status ON issues (project_id, issue_type, status);
CREATE INDEX idx_issue_dependencies_predecessor ON issue_dependencies (predecessor_id);
CREATE INDEX idx_outbox_events_status_created ON outbox_events (status, created_at);
CREATE INDEX idx_notifications_user ON notifications (user_id, is_read);
CREATE INDEX idx_audit_logs_entity ON audit_logs (entity_type, entity_id);
