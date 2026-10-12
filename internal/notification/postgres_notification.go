package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// PostgresService persists notification center entities into TimescaleDB/PostgreSQL.
type PostgresService struct {
	pool          *pgxpool.Pool
	smtpHost      string
	smtpPort      int
	httpClient    *http.Client
	escalationsMu sync.Mutex
	escalations   map[string]context.CancelFunc
}

// NewPostgresService initializes PostgreSQL persistence for Notification Center.
func NewPostgresService(pool *pgxpool.Pool, cfg Config) Service {
	if cfg.SMTPHost == "" {
		cfg.SMTPHost = "127.0.0.1"
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 1025
	}
	return &PostgresService{
		pool:        pool,
		smtpHost:    cfg.SMTPHost,
		smtpPort:    cfg.SMTPPort,
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		escalations: make(map[string]context.CancelFunc),
	}
}

func (s *PostgresService) ensureTenantExists(ctx context.Context, tenantID ids.ID) ids.ID {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tenant WHERE id = $1)", tenantID).Scan(&exists)
	if err == nil && exists {
		return tenantID
	}

	var primaryTenant ids.ID
	err = s.pool.QueryRow(ctx, "SELECT id FROM tenant ORDER BY created_time ASC LIMIT 1").Scan(&primaryTenant)
	if err == nil {
		return primaryTenant
	}
	return tenantID
}

// Target Operations

func (s *PostgresService) SaveTarget(ctx context.Context, t *model.NotificationTarget) (*model.NotificationTarget, error) {
	if t.ID == ids.Nil {
		t.ID = ids.MustNew()
	}
	if t.CreatedTime == 0 {
		t.CreatedTime = time.Now().UnixMilli()
	}
	t.TenantID = s.ensureTenantExists(ctx, t.TenantID)
	if len(t.Config) == 0 {
		t.Config = json.RawMessage(`{}`)
	}

	query := `
		INSERT INTO notification_target (id, created_time, tenant_id, name, type, config)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, name) DO UPDATE SET
			type = EXCLUDED.type,
			config = EXCLUDED.config
	`
	_, err := s.pool.Exec(ctx, query, t.ID, t.CreatedTime, t.TenantID, t.Name, t.Type, t.Config)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *PostgresService) GetTarget(ctx context.Context, tenantID, targetID ids.ID) (*model.NotificationTarget, error) {
	var t model.NotificationTarget
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, name, type, config
		FROM notification_target
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, targetID).Scan(&t.ID, &t.CreatedTime, &t.TenantID, &t.Name, &t.Type, &t.Config)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("notification target not found")
		}
		return nil, err
	}
	return &t, nil
}

func (s *PostgresService) ListTargets(ctx context.Context, tenantID ids.ID) ([]model.NotificationTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, created_time, tenant_id, name, type, config
		FROM notification_target
		WHERE tenant_id = $1
		ORDER BY created_time DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]model.NotificationTarget, 0)
	for rows.Next() {
		var t model.NotificationTarget
		if err := rows.Scan(&t.ID, &t.CreatedTime, &t.TenantID, &t.Name, &t.Type, &t.Config); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func (s *PostgresService) DeleteTarget(ctx context.Context, tenantID, targetID ids.ID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notification_target WHERE tenant_id = $1 AND id = $2`, tenantID, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("notification target not found")
	}
	return nil
}

// Template Operations

func (s *PostgresService) SaveTemplate(ctx context.Context, tpl *model.NotificationTemplate) (*model.NotificationTemplate, error) {
	if tpl.ID == ids.Nil {
		tpl.ID = ids.MustNew()
	}
	if tpl.CreatedTime == 0 {
		tpl.CreatedTime = time.Now().UnixMilli()
	}
	tpl.TenantID = s.ensureTenantExists(ctx, tpl.TenantID)
	if tpl.Type == "" {
		tpl.Type = "ALARM"
	}
	if len(tpl.Config) == 0 {
		tpl.Config = json.RawMessage(`{}`)
	}

	query := `
		INSERT INTO notification_template (id, created_time, tenant_id, name, type, config)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, name) DO UPDATE SET
			type = EXCLUDED.type,
			config = EXCLUDED.config
	`
	_, err := s.pool.Exec(ctx, query, tpl.ID, tpl.CreatedTime, tpl.TenantID, tpl.Name, tpl.Type, tpl.Config)
	if err != nil {
		return nil, err
	}
	return tpl, nil
}

func (s *PostgresService) GetTemplate(ctx context.Context, tenantID, templateID ids.ID) (*model.NotificationTemplate, error) {
	var tpl model.NotificationTemplate
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, name, type, config
		FROM notification_template
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, templateID).Scan(&tpl.ID, &tpl.CreatedTime, &tpl.TenantID, &tpl.Name, &tpl.Type, &tpl.Config)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("notification template not found")
		}
		return nil, err
	}
	return &tpl, nil
}

func (s *PostgresService) ListTemplates(ctx context.Context, tenantID ids.ID) ([]model.NotificationTemplate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, created_time, tenant_id, name, type, config
		FROM notification_template
		WHERE tenant_id = $1
		ORDER BY created_time DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]model.NotificationTemplate, 0)
	for rows.Next() {
		var tpl model.NotificationTemplate
		if err := rows.Scan(&tpl.ID, &tpl.CreatedTime, &tpl.TenantID, &tpl.Name, &tpl.Type, &tpl.Config); err != nil {
			return nil, err
		}
		templates = append(templates, tpl)
	}
	return templates, nil
}

func (s *PostgresService) DeleteTemplate(ctx context.Context, tenantID, templateID ids.ID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notification_template WHERE tenant_id = $1 AND id = $2`, tenantID, templateID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("notification template not found")
	}
	return nil
}

// Rule Operations

func (s *PostgresService) SaveRule(ctx context.Context, r *model.NotificationRule) (*model.NotificationRule, error) {
	if r.ID == ids.Nil {
		r.ID = ids.MustNew()
	}
	if r.CreatedTime == 0 {
		r.CreatedTime = time.Now().UnixMilli()
	}
	r.TenantID = s.ensureTenantExists(ctx, r.TenantID)
	if r.TriggerType == "" {
		r.TriggerType = "ALARM"
	}
	if len(r.TriggerConfig) == 0 {
		r.TriggerConfig = json.RawMessage(`{}`)
	}

	recipientsBytes, err := json.Marshal(r.RecipientsConfig)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO notification_rule (
			id, created_time, tenant_id, name, enabled, template_id,
			trigger_type, trigger_config, recipients_config, throttle
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		ON CONFLICT (tenant_id, name) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			template_id = EXCLUDED.template_id,
			trigger_type = EXCLUDED.trigger_type,
			trigger_config = EXCLUDED.trigger_config,
			recipients_config = EXCLUDED.recipients_config,
			throttle = EXCLUDED.throttle
	`
	_, err = s.pool.Exec(ctx, query,
		r.ID, r.CreatedTime, r.TenantID, r.Name, r.Enabled, r.TemplateID,
		r.TriggerType, r.TriggerConfig, recipientsBytes, r.Throttle,
	)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *PostgresService) GetRule(ctx context.Context, tenantID, ruleID ids.ID) (*model.NotificationRule, error) {
	var r model.NotificationRule
	var recipientsBytes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, name, enabled, template_id,
		       trigger_type, trigger_config, recipients_config, throttle
		FROM notification_rule
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, ruleID).Scan(
		&r.ID, &r.CreatedTime, &r.TenantID, &r.Name, &r.Enabled, &r.TemplateID,
		&r.TriggerType, &r.TriggerConfig, &recipientsBytes, &r.Throttle,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("notification rule not found")
		}
		return nil, err
	}
	_ = json.Unmarshal(recipientsBytes, &r.RecipientsConfig)
	return &r, nil
}

func (s *PostgresService) ListRules(ctx context.Context, tenantID ids.ID) ([]model.NotificationRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, created_time, tenant_id, name, enabled, template_id,
		       trigger_type, trigger_config, recipients_config, throttle
		FROM notification_rule
		WHERE tenant_id = $1
		ORDER BY created_time DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]model.NotificationRule, 0)
	for rows.Next() {
		var r model.NotificationRule
		var recipientsBytes []byte
		if err := rows.Scan(
			&r.ID, &r.CreatedTime, &r.TenantID, &r.Name, &r.Enabled, &r.TemplateID,
			&r.TriggerType, &r.TriggerConfig, &recipientsBytes, &r.Throttle,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(recipientsBytes, &r.RecipientsConfig)
		rules = append(rules, r)
	}
	return rules, nil
}

func (s *PostgresService) DeleteRule(ctx context.Context, tenantID, ruleID ids.ID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notification_rule WHERE tenant_id = $1 AND id = $2`, tenantID, ruleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("notification rule not found")
	}
	return nil
}

// In-App Inbox Persistence

func (s *PostgresService) ListInbox(ctx context.Context, tenantID, recipientID ids.ID, limit, offset int) ([]model.Notification, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if recipientID != ids.Nil {
		_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM notification WHERE tenant_id = $1 AND (recipient_id = $2 OR recipient_id = '00000000-0000-0000-0000-000000000000')`, tenantID, recipientID).Scan(&total)
	} else {
		_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM notification WHERE tenant_id = $1`, tenantID).Scan(&total)
	}

	var rows pgx.Rows
	var err error
	if recipientID != ids.Nil {
		rows, err = s.pool.Query(ctx, `
			SELECT id, created_time, tenant_id, recipient_id, request_id, type, subject, body, info, status
			FROM notification
			WHERE tenant_id = $1 AND (recipient_id = $2 OR recipient_id = '00000000-0000-0000-0000-000000000000')
			ORDER BY created_time DESC
			LIMIT $3 OFFSET $4
		`, tenantID, recipientID, limit, offset)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, created_time, tenant_id, recipient_id, request_id, type, subject, body, info, status
			FROM notification
			WHERE tenant_id = $1
			ORDER BY created_time DESC
			LIMIT $2 OFFSET $3
		`, tenantID, limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	notifications := make([]model.Notification, 0)
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(
			&n.ID, &n.CreatedTime, &n.TenantID, &n.RecipientID, &n.RequestID,
			&n.Type, &n.Subject, &n.Body, &n.Info, &n.Status,
		); err != nil {
			return nil, 0, err
		}
		notifications = append(notifications, n)
	}

	return notifications, total, nil
}

func (s *PostgresService) MarkRead(ctx context.Context, tenantID, notificationID ids.ID) error {
	_, err := s.pool.Exec(ctx, `UPDATE notification SET status = 'READ' WHERE tenant_id = $1 AND id = $2`, tenantID, notificationID)
	return err
}

func (s *PostgresService) MarkAllRead(ctx context.Context, tenantID, recipientID ids.ID) error {
	if recipientID != ids.Nil {
		_, err := s.pool.Exec(ctx, `UPDATE notification SET status = 'READ' WHERE tenant_id = $1 AND (recipient_id = $2 OR recipient_id = '00000000-0000-0000-0000-000000000000')`, tenantID, recipientID)
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE notification SET status = 'READ' WHERE tenant_id = $1`, tenantID)
	return err
}

func (s *PostgresService) GetUnreadCount(ctx context.Context, tenantID, recipientID ids.ID) (int, error) {
	var count int
	var err error
	if recipientID != ids.Nil {
		err = s.pool.QueryRow(ctx, `SELECT count(*) FROM notification WHERE tenant_id = $1 AND (recipient_id = $2 OR recipient_id = '00000000-0000-0000-0000-000000000000') AND status = 'UNREAD'`, tenantID, recipientID).Scan(&count)
	} else {
		err = s.pool.QueryRow(ctx, `SELECT count(*) FROM notification WHERE tenant_id = $1 AND status = 'UNREAD'`, tenantID).Scan(&count)
	}
	return count, err
}

// Delivery Dispatcher

func (s *PostgresService) deliverToTarget(ctx context.Context, tenantID ids.ID, t *model.NotificationTarget, subject, body string, info json.RawMessage) {
	switch t.Type {
	case model.TargetPlatformUsers:
		var cfg struct {
			UserIDs []string `json:"user_ids"`
		}
		_ = json.Unmarshal(t.Config, &cfg)

		recipients := cfg.UserIDs
		if len(recipients) == 0 {
			// Find primary admin or tenant users
			rows, err := s.pool.Query(ctx, `SELECT id FROM users WHERE tenant_id = $1 LIMIT 5`, tenantID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var uid ids.ID
					if rows.Scan(&uid) == nil {
						recipients = append(recipients, uid.String())
					}
				}
			}
		}
		if len(recipients) == 0 {
			recipients = []string{ids.Nil.String()}
		}

		if len(info) == 0 {
			info = json.RawMessage(`{}`)
		}

		now := time.Now().UnixMilli()
		for _, u := range recipients {
			recID, _ := ids.Parse(u)
			msgID := ids.MustNew()
			_, _ = s.pool.Exec(ctx, `
				INSERT INTO notification (id, created_time, tenant_id, recipient_id, type, subject, body, info, status)
				VALUES ($1, $2, $3, $4, 'ALARM', $5, $6, $7, 'UNREAD')
			`, msgID, now, tenantID, recID, subject, body, info)
		}

	case model.TargetEmail:
		var cfg struct {
			Emails []string `json:"emails"`
		}
		_ = json.Unmarshal(t.Config, &cfg)
		if len(cfg.Emails) == 0 {
			cfg.Emails = []string{"alerts@iotp.local"}
		}

		addr := fmt.Sprintf("%s:%d", s.smtpHost, s.smtpPort)
		from := "noreply@iotp.platform"
		msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n",
			from, strings.Join(cfg.Emails, ","), subject, body)

		_ = smtp.SendMail(addr, nil, from, cfg.Emails, []byte(msg))

	case model.TargetSlack, model.TargetWebhook:
		var cfg struct {
			WebhookURL string `json:"webhook_url"`
		}
		_ = json.Unmarshal(t.Config, &cfg)
		if cfg.WebhookURL != "" {
			payload, _ := json.Marshal(map[string]any{
				"text":    fmt.Sprintf("*%s*\n%s", subject, body),
				"subject": subject,
				"body":    body,
			})
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewBuffer(payload))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				_, _ = s.httpClient.Do(req)
			}
		}
	}
}

func (s *PostgresService) SendNotification(ctx context.Context, tenantID ids.ID, req *model.NotificationSendRequest) (*model.NotificationRequest, error) {
	for _, tid := range req.Targets {
		t, err := s.GetTarget(ctx, tenantID, tid)
		if err == nil && t != nil {
			s.deliverToTarget(ctx, tenantID, t, req.Subject, req.Body, req.Info)
		}
	}

	return &model.NotificationRequest{
		ID:          ids.MustNew(),
		CreatedTime: time.Now().UnixMilli(),
		TenantID:    tenantID,
		Targets:     req.Targets,
		Info:        req.Info,
		Status:      "SENT",
	}, nil
}

func (s *PostgresService) CancelAlarmEscalations(alarmID ids.ID) {
	s.escalationsMu.Lock()
	defer s.escalationsMu.Unlock()
	if cancel, exists := s.escalations[alarmID.String()]; exists {
		cancel()
		delete(s.escalations, alarmID.String())
	}
}

func (s *PostgresService) ProcessAlarmEvent(ctx context.Context, eventType string, a *model.Alarm) error {
	if a == nil {
		return nil
	}

	if eventType == "ALARM_ACKED" || eventType == bus.TypeAlarmCleared || a.Status.IncludesAck() || a.Status.IsCleared() {
		s.CancelAlarmEscalations(a.ID)
	}

	if eventType == bus.TypeAlarmCleared {
		return nil
	}

	rules, err := s.ListRules(ctx, a.TenantID)
	if err != nil || len(rules) == 0 {
		return nil
	}

	vars := map[string]string{
		"alarm.id":       a.ID.String(),
		"alarm.type":     a.Type,
		"alarm.severity": string(a.Severity),
		"alarm.status":   string(a.Status),
		"device.id":      a.OriginatorID.String(),
		"time":           time.UnixMilli(a.CreatedTime).Format(time.RFC3339),
	}
	if len(a.Details) > 0 {
		vars["alarm.details"] = string(a.Details)
	}

	for _, r := range rules {
		if !r.Enabled || (r.TriggerType != "ALARM" && r.TriggerType != eventType) {
			continue
		}

		tpl, _ := s.GetTemplate(ctx, a.TenantID, r.TemplateID)

		subject := fmt.Sprintf("[%s] %s Alert on Device %s", a.Severity, a.Type, a.OriginatorID.String())
		body := fmt.Sprintf("Alarm %s of severity %s was triggered at %s.", a.Type, a.Severity, vars["time"])

		if tpl != nil {
			var tplCfg struct {
				Subject string `json:"subject"`
				Body    string `json:"body"`
			}
			_ = json.Unmarshal(tpl.Config, &tplCfg)
			if tplCfg.Subject != "" {
				subject = InterpolateTemplate(tplCfg.Subject, vars)
			}
			if tplCfg.Body != "" {
				body = InterpolateTemplate(tplCfg.Body, vars)
			}
		}

		alarmKey := a.ID.String()
		escCtx, cancel := context.WithCancel(context.Background())
		s.escalationsMu.Lock()
		s.escalations[alarmKey] = cancel
		s.escalationsMu.Unlock()

		for _, step := range r.RecipientsConfig {
			stepCopy := step
			if stepCopy.DelaySeconds <= 0 {
				for _, tid := range stepCopy.TargetIDs {
					target, err := s.GetTarget(ctx, a.TenantID, tid)
					if err == nil && target != nil {
						s.deliverToTarget(ctx, a.TenantID, target, subject, body, a.Details)
					}
				}
			} else {
				go func(delaySec int, targetIDs []ids.ID) {
					select {
					case <-time.After(time.Duration(delaySec) * time.Second):
						for _, tid := range targetIDs {
							target, err := s.GetTarget(context.Background(), a.TenantID, tid)
							if err == nil && target != nil {
								s.deliverToTarget(context.Background(), a.TenantID, target,
									fmt.Sprintf("[ESCALATION TIER] %s", subject),
									fmt.Sprintf("This alarm has not been acknowledged within %d seconds:\n%s", delaySec, body),
									a.Details)
							}
						}
					case <-escCtx.Done():
						return
					}
				}(stepCopy.DelaySeconds, stepCopy.TargetIDs)
			}
		}
	}

	return nil
}

func (s *PostgresService) Start(ctx context.Context, b bus.Bus) error {
	if b == nil {
		return nil
	}
	_, err := b.Subscribe(ctx, alarm.TopicAlarmEvents, "notification-worker", func(c context.Context, msg *bus.Message) error {
		var a model.Alarm
		if err := json.Unmarshal(msg.Payload, &a); err != nil {
			return err
		}
		return s.ProcessAlarmEvent(c, msg.Type, &a)
	})
	return err
}
