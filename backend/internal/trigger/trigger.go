package trigger

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"gojira/internal/config"
	"gojira/internal/domain"
	"gojira/internal/platform"
	"gojira/internal/workflow"

	"github.com/jmoiron/sqlx"
)

const MaxRetries = 3

type Action struct {
	Type    string `json:"type"`
	ToRole  string `json:"to_role"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	URL     string `json:"url"`
	UserID  int64  `json:"user_id"`
}

type Worker struct {
	DB     *sqlx.DB
	Cfg    *config.Config
	Log    *slog.Logger
	Engine *workflow.Engine
	Mailer Mailer
}

// Mailer is the real SMTP path. Tests inject a fake; production uses SMTPMailer.
type Mailer interface {
	Send(to []string, subject, body string) error
}

type SMTPMailer struct {
	Host string
	Port int
	User string
	Pass string
	From string
	TLS  bool
}

func (m SMTPMailer) Send(to []string, subject, body string) error {
	if len(to) == 0 {
		return &ClassifiedError{Class: domain.ErrorPermanent, Msg: "no recipients"}
	}
	addr := net.JoinHostPort(m.Host, fmt.Sprintf("%d", m.Port))
	msg := []byte("From: " + m.From + "\r\n" +
		"To: " + strings.Join(to, ",") + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body + "\r\n")
	var auth smtp.Auth
	if m.User != "" {
		auth = smtp.PlainAuth("", m.User, m.Pass, m.Host)
	}
	if m.TLS {
		return sendTLS(addr, m.Host, auth, m.From, to, msg)
	}
	return smtp.SendMail(addr, auth, m.From, to, msg)
}

func sendTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

type ClassifiedError struct {
	Class string
	Msg   string
}

func (e *ClassifiedError) Error() string { return e.Msg }

func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	var ce *ClassifiedError
	if errors.As(err, &ce) && ce.Class != "" {
		return ce.Class
	}
	msg := strings.ToLower(err.Error())
	permanentHints := []string{
		"auth", "authentication", "535", "530", "550", "553", "554",
		"invalid", "validation", "unauthorized", "forbidden", "recipient",
		"no recipients", "malformed",
	}
	for _, h := range permanentHints {
		if strings.Contains(msg, h) {
			return domain.ErrorPermanent
		}
	}
	transientHints := []string{
		"timeout", "temporar", "connection refused", "connection reset",
		"eof", "broken pipe", "i/o timeout", "421", "450", "451", "452",
		"try again", "unavailable",
	}
	for _, h := range transientHints {
		if strings.Contains(msg, h) {
			return domain.ErrorTransient
		}
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return domain.ErrorTransient
	}
	return domain.ErrorTransient
}

func ShouldRetry(class string, retryCount int) bool {
	if class != domain.ErrorTransient {
		return false
	}
	return retryCount < MaxRetries
}

func NextBackoff(retryCount int) time.Duration {
	// 2s, 4s, 8s
	shift := retryCount
	if shift < 0 {
		shift = 0
	}
	if shift > 4 {
		shift = 4
	}
	return time.Duration(2<<shift) * time.Second
}

func ExecutionKey(eventID string, triggerID int64, actionType string) string {
	return fmt.Sprintf("%s:%d:%s", eventID, triggerID, actionType)
}

func MatchCondition(cond map[string]any, payload domain.StatusChangedPayload) bool {
	if len(cond) == 0 {
		return true
	}
	if v, ok := cond["to_column"].(string); ok && v != "" && payload.ToColumn != v {
		return false
	}
	if v, ok := cond["to_status"].(string); ok && v != "" && payload.ToStatus != v {
		return false
	}
	if v, ok := cond["issue_type"].(string); ok && v != "" && payload.IssueType != v {
		return false
	}
	if v, ok := cond["from_status"].(string); ok && v != "" && payload.FromStatus != v {
		return false
	}
	return true
}

func (w *Worker) Start(ctx context.Context) {
	if w.Mailer == nil {
		w.Mailer = SMTPMailer{
			Host: w.Cfg.SMTPHost, Port: w.Cfg.SMTPPort,
			User: w.Cfg.SMTPUser, Pass: w.Cfg.SMTPPass,
			From: w.Cfg.SMTPFrom, TLS: w.Cfg.SMTPTLS,
		}
	}
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	w.Log.Info("outbox worker started")
	for {
		select {
		case <-ctx.Done():
			w.Log.Info("outbox worker stopped")
			return
		case <-t.C:
			if err := w.Poll(ctx); err != nil && ctx.Err() == nil {
				w.Log.Error("outbox poll", "err", err)
			}
		}
	}
}

func (w *Worker) Poll(ctx context.Context) error {
	now := platform.Now()
	var events []domain.OutboxEvent
	err := w.DB.SelectContext(ctx, &events, `
		SELECT * FROM outbox_events
		WHERE status='pending' AND (next_attempt_at IS NULL OR next_attempt_at <= $1)
		ORDER BY created_at
		LIMIT 20`, now)
	if err != nil {
		return err
	}
	for i := range events {
		w.deliver(ctx, &events[i])
	}
	return nil
}

func (w *Worker) deliver(ctx context.Context, ev *domain.OutboxEvent) {
	if ev.EventType != domain.EventIssueStatusChanged {
		w.mark(ctx, ev, domain.OutboxDone, "", "")
		return
	}
	var payload domain.StatusChangedPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		w.mark(ctx, ev, domain.OutboxDead, domain.ErrorPermanent, "invalid payload")
		return
	}
	var triggers []domain.Trigger
	if err := w.DB.SelectContext(ctx, &triggers, `
		SELECT * FROM triggers WHERE project_id=$1 AND event_type=$2 AND is_enabled=TRUE`,
		payload.ProjectID, ev.EventType); err != nil {
		w.Log.Error("load triggers", "err", err)
		return
	}
	allOK := true
	var lastClass, lastMsg string
	for _, tr := range triggers {
		var cond map[string]any
		_ = json.Unmarshal(tr.Condition, &cond)
		if !MatchCondition(cond, payload) {
			continue
		}
		var actions []Action
		if err := json.Unmarshal(tr.Actions, &actions); err != nil {
			allOK = false
			lastClass, lastMsg = domain.ErrorPermanent, "invalid actions json"
			continue
		}
		for _, act := range actions {
			if !w.execOnce(ctx, ev, tr, act, payload) {
				allOK = false
				lastClass = domain.ErrorTransient
				lastMsg = "action failed"
			}
		}
	}
	if allOK {
		w.mark(ctx, ev, domain.OutboxDone, "", "")
		return
	}
	if ShouldRetry(lastClass, ev.RetryCount+1) {
		next := platform.Now().Add(NextBackoff(ev.RetryCount))
		_, _ = w.DB.ExecContext(ctx, `
			UPDATE outbox_events SET retry_count=retry_count+1, error_class=$1, error_msg=$2,
				next_attempt_at=$3, updated_at=$4 WHERE id=$5`,
			lastClass, lastMsg, next, platform.Now(), ev.ID)
		return
	}
	status := domain.OutboxDead
	if lastClass == domain.ErrorTransient {
		status = domain.OutboxFailed
	}
	w.mark(ctx, ev, status, lastClass, lastMsg)
}

func (w *Worker) execOnce(ctx context.Context, ev *domain.OutboxEvent, tr domain.Trigger, act Action, payload domain.StatusChangedPayload) bool {
	key := ExecutionKey(ev.EventID, tr.ID, act.Type)
	_ = key
	var exists int
	_ = w.DB.GetContext(ctx, &exists, `
		SELECT COUNT(*) FROM trigger_executions
		WHERE event_id=$1 AND trigger_id=$2 AND action_type=$3`, ev.EventID, tr.ID, act.Type)
	if exists > 0 {
		return true
	}
	start := platform.Now()
	err := w.runAction(ctx, act, payload)
	dur := int(platform.Now().Sub(start).Milliseconds())
	if err != nil {
		c := ClassifyError(err)
		if ShouldRetry(c, ev.RetryCount) {
			// Leave no execution row so a later poll can retry this action.
			return false
		}
		m := err.Error()
		_, insErr := w.DB.ExecContext(ctx, `
			INSERT INTO trigger_executions (trigger_id, event_id, action_type, status, error_class, error_msg, retry_count, duration_ms, created_at)
			VALUES ($1,$2,$3,'skipped',$4,$5,0,$6,$7)
			ON CONFLICT (event_id, trigger_id, action_type) DO NOTHING`,
			tr.ID, ev.EventID, act.Type, c, m, dur, platform.Now())
		if insErr != nil {
			w.Log.Error("record execution", "err", insErr)
		}
		return true
	}
	_, insErr := w.DB.ExecContext(ctx, `
		INSERT INTO trigger_executions (trigger_id, event_id, action_type, status, error_class, error_msg, retry_count, duration_ms, created_at)
		VALUES ($1,$2,$3,'ok',NULL,NULL,0,$4,$5)
		ON CONFLICT (event_id, trigger_id, action_type) DO NOTHING`,
		tr.ID, ev.EventID, act.Type, dur, platform.Now())
	if insErr != nil {
		w.Log.Error("record execution", "err", insErr)
	}
	return true
}

func (w *Worker) runAction(ctx context.Context, act Action, payload domain.StatusChangedPayload) error {
	switch act.Type {
	case domain.ActionSendEmail:
		to, err := w.recipients(ctx, payload.ProjectID, act)
		if err != nil {
			return err
		}
		subj := act.Subject
		if subj == "" {
			subj = "GoJira: " + payload.IssueKey + " 已进入已完成"
		}
		body := act.Body
		if body == "" {
			body = fmt.Sprintf("事项 %s「%s」状态变更为 %s（列 %s）。", payload.IssueKey, payload.Title, payload.ToStatus, payload.ToColumn)
		}
		if err := w.Mailer.Send(to, subj, body); err != nil {
			return err
		}
		w.notifyPMs(ctx, payload, body)
		return nil
	case domain.ActionInAppNotify:
		w.notifyPMs(ctx, payload, act.Body)
		return nil
	case domain.ActionAddComment:
		if strings.TrimSpace(act.Body) == "" {
			return &ClassifiedError{Class: domain.ErrorPermanent, Msg: "empty comment"}
		}
		_, err := w.DB.ExecContext(ctx, `
			INSERT INTO issue_comments (issue_id, author_id, body, created_at)
			VALUES ($1,$2,$3,$4)`, payload.IssueID, payload.ActorID, act.Body, platform.Now())
		return err
	case domain.ActionAutoAssign:
		if act.UserID == 0 {
			return &ClassifiedError{Class: domain.ErrorPermanent, Msg: "missing user_id"}
		}
		_, err := w.DB.ExecContext(ctx, `UPDATE issues SET assignee_id=$1, updated_at=$2 WHERE id=$3`,
			act.UserID, platform.Now(), payload.IssueID)
		return err
	case domain.ActionWebhook:
		if act.URL == "" {
			return &ClassifiedError{Class: domain.ErrorPermanent, Msg: "missing webhook url"}
		}
		return postWebhook(act.URL, payload)
	default:
		return &ClassifiedError{Class: domain.ErrorPermanent, Msg: "unknown action " + act.Type}
	}
}

func postWebhook(url string, payload domain.StatusChangedPayload) error {
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(raw)))
	if err != nil {
		return &ClassifiedError{Class: domain.ErrorPermanent, Msg: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("webhook 5xx: %d", resp.StatusCode)
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 422 {
		return &ClassifiedError{Class: domain.ErrorPermanent, Msg: fmt.Sprintf("webhook %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 400 {
		return &ClassifiedError{Class: domain.ErrorPermanent, Msg: fmt.Sprintf("webhook %d", resp.StatusCode)}
	}
	return nil
}

func (w *Worker) recipients(ctx context.Context, projectID int64, act Action) ([]string, error) {
	if act.To != "" {
		return []string{act.To}, nil
	}
	role := act.ToRole
	if role == "" {
		role = domain.RolePM
	}
	var emails []string
	err := w.DB.SelectContext(ctx, &emails, `
		SELECT u.email FROM project_members m
		JOIN users u ON u.id=m.user_id
		WHERE m.project_id=$1 AND m.role=$2 AND u.is_active=TRUE`, projectID, role)
	if err != nil {
		return nil, err
	}
	if len(emails) == 0 {
		return nil, &ClassifiedError{Class: domain.ErrorPermanent, Msg: "no recipients"}
	}
	return emails, nil
}

func (w *Worker) notifyPMs(ctx context.Context, payload domain.StatusChangedPayload, body string) {
	if w.DB == nil {
		return
	}
	var ids []int64
	_ = w.DB.SelectContext(ctx, &ids, `
		SELECT user_id FROM project_members WHERE project_id=$1 AND role=$2`, payload.ProjectID, domain.RolePM)
	title := payload.IssueKey + " 已进入已完成"
	if body == "" {
		body = payload.Title
	}
	for _, id := range ids {
		_, _ = w.DB.ExecContext(ctx, `
			INSERT INTO notifications (user_id, type, title, body, is_read, created_at)
			VALUES ($1,'issue.done',$2,$3,FALSE,$4)`, id, title, body, platform.Now())
	}
}

func (w *Worker) mark(ctx context.Context, ev *domain.OutboxEvent, status, class, msg string) {
	var c, m any
	if class != "" {
		c = class
	}
	if msg != "" {
		m = msg
	}
	_, _ = w.DB.ExecContext(ctx, `
		UPDATE outbox_events SET status=$1, error_class=$2, error_msg=$3, updated_at=$4 WHERE id=$5`,
		status, c, m, platform.Now(), ev.ID)
}

type API struct {
	DB *sqlx.DB
}

func (a *API) List(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	var rows []domain.Trigger
	if err := a.DB.Select(&rows, `SELECT * FROM triggers WHERE project_id=$1 ORDER BY id`, pid); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, rows)
}

func (a *API) Executions(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	page, per, offset := platform.ParsePage(r)
	var total int64
	_ = a.DB.Get(&total, `
		SELECT COUNT(*) FROM trigger_executions e
		JOIN triggers t ON t.id=e.trigger_id WHERE t.project_id=$1`, pid)
	var rows []domain.TriggerExecution
	if err := a.DB.Select(&rows, `
		SELECT e.* FROM trigger_executions e
		JOIN triggers t ON t.id=e.trigger_id
		WHERE t.project_id=$1
		ORDER BY e.created_at DESC LIMIT $2 OFFSET $3`, pid, per, offset); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteDataMeta(w, http.StatusOK, rows, platform.NewMeta(page, per, total))
}
