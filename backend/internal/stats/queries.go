package stats

// Hand-written aggregation SQL. Do not assemble these with an ORM.

const BurndownSQL = `
WITH bounds AS (
    SELECT s.id AS sprint_id, s.start_date::date AS start_d, s.end_date::date AS end_d,
           s.committed_points
    FROM sprints s
    WHERE s.id = $1
),
days AS (
    SELECT generate_series(b.start_d, b.end_d, interval '1 day')::date AS d, b.committed_points
    FROM bounds b
),
scoped AS (
    SELECT i.id, i.story_points, i.estimate_hours, i.status, i.created_at::date AS entered_on
    FROM issues i
    WHERE i.sprint_id = $1
),
status_on_day AS (
    SELECT d.d,
           sc.id AS issue_id,
           sc.story_points,
           sc.estimate_hours,
           COALESCE(
               (SELECT h.to_status
                FROM issue_status_history h
                WHERE h.issue_id = sc.id
                  AND h.changed_at < (d.d + interval '1 day')
                ORDER BY h.changed_at DESC
                LIMIT 1),
               sc.status
           ) AS status_on_day,
           (sc.entered_on <= d.d) AS in_scope
    FROM days d
    CROSS JOIN scoped sc
)
SELECT
    d.d AS day,
    COALESCE(SUM(
        CASE WHEN sod.in_scope AND sod.status_on_day NOT IN ('DONE','RESOLVED','CLOSED','REJECTED')
             THEN CASE
                    WHEN $2 = 'hours' THEN COALESCE(sod.estimate_hours, 0)
                    WHEN $2 = 'count' THEN 1
                    ELSE COALESCE(sod.story_points, 0)
                  END
             ELSE 0 END
    ), 0) AS remaining,
    COALESCE(SUM(
        CASE WHEN sod.in_scope THEN
             CASE
               WHEN $2 = 'hours' THEN COALESCE(sod.estimate_hours, 0)
               WHEN $2 = 'count' THEN 1
               ELSE COALESCE(sod.story_points, 0)
             END
        ELSE 0 END
    ), 0) AS scope,
    (SELECT committed_points FROM days LIMIT 1) AS committed
FROM days d
LEFT JOIN status_on_day sod ON sod.d = d.d
GROUP BY d.d
ORDER BY d.d
`

const VelocitySQL = `
SELECT
    to_char(h.changed_at AT TIME ZONE 'Asia/Shanghai', 'IYYY-IW') AS iso_week,
    date_trunc('week', h.changed_at AT TIME ZONE 'Asia/Shanghai')::date AS week_start,
    i.assignee_id,
    COALESCE(u.display_name, '未分配') AS assignee_name,
    COALESCE(SUM(i.story_points), 0) AS points,
    COUNT(*) AS issue_count
FROM issue_status_history h
JOIN issues i ON i.id = h.issue_id
LEFT JOIN users u ON u.id = i.assignee_id
WHERE i.project_id = $1
  AND h.to_status IN ('DONE', 'RESOLVED', 'CLOSED')
GROUP BY 1, 2, 3, 4
ORDER BY 2, 3
`

const ProgressSQL = `
WITH sp AS (
    SELECT * FROM sprints WHERE id = $1
),
iss AS (
    SELECT i.*
    FROM issues i
    WHERE i.sprint_id = $1
)
SELECT
    (SELECT committed_points FROM sp) AS committed_points,
    COALESCE((SELECT SUM(story_points) FROM iss), 0) AS scope_points,
    COALESCE((SELECT SUM(story_points) FROM iss WHERE status IN ('DONE','RESOLVED','CLOSED','REJECTED')), 0) AS completed_points,
    COALESCE((SELECT COUNT(*) FROM iss), 0) AS issue_count,
    COALESCE((SELECT COUNT(*) FROM iss WHERE status IN ('DONE','RESOLVED','CLOSED','REJECTED')), 0) AS completed_count,
    (SELECT start_date FROM sp) AS start_date,
    (SELECT end_date FROM sp) AS end_date
`

const BugStatsSQL = `
WITH bugs AS (
    SELECT * FROM issues WHERE project_id = $1 AND issue_type = 'BUG'
),
sev AS (
    SELECT COALESCE(severity, 'UNSET') AS key, COUNT(*) AS n FROM bugs GROUP BY 1
),
st AS (
    SELECT status AS key, COUNT(*) AS n FROM bugs GROUP BY 1
),
opened AS (
    SELECT issue_id, MIN(changed_at) AS opened_at
    FROM issue_status_history
    WHERE issue_id IN (SELECT id FROM bugs)
      AND (from_status IN ('NEW','CONFIRMED') OR to_status IN ('NEW','CONFIRMED'))
    GROUP BY issue_id
),
resolved AS (
    SELECT issue_id, MIN(changed_at) AS resolved_at
    FROM issue_status_history
    WHERE issue_id IN (SELECT id FROM bugs)
      AND to_status = 'RESOLVED'
    GROUP BY issue_id
)
SELECT
    (SELECT COALESCE(json_object_agg(key, n), '{}'::json) FROM sev) AS severity_dist,
    (SELECT COALESCE(json_object_agg(key, n), '{}'::json) FROM st) AS status_dist,
    (SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (r.resolved_at - o.opened_at)) / 3600.0), 0)
     FROM opened o JOIN resolved r ON r.issue_id = o.issue_id
     WHERE r.resolved_at >= o.opened_at) AS mttr_hours,
    (SELECT COUNT(*) FROM bugs) AS total
`

const SnapshotSQL = `
INSERT INTO sprint_daily_snapshots (sprint_id, snapshot_date, remaining_points, completed_points, scope_points)
SELECT
    $1,
    $2::date,
    COALESCE(SUM(CASE WHEN status NOT IN ('DONE','RESOLVED','CLOSED','REJECTED') THEN COALESCE(story_points,0) ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN status IN ('DONE','RESOLVED','CLOSED','REJECTED') THEN COALESCE(story_points,0) ELSE 0 END), 0),
    COALESCE(SUM(COALESCE(story_points,0)), 0)
FROM issues
WHERE sprint_id = $1
ON CONFLICT (sprint_id, snapshot_date) DO UPDATE SET
    remaining_points = EXCLUDED.remaining_points,
    completed_points = EXCLUDED.completed_points,
    scope_points = EXCLUDED.scope_points
`
