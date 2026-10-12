package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Service defines notification management and delivery operations.
type Service interface {
	// Targets
	SaveTarget(ctx context.Context, target *model.NotificationTarget) (*model.NotificationTarget, error)
	GetTarget(ctx context.Context, tenantID, targetID ids.ID) (*model.NotificationTarget, error)
	ListTargets(ctx context.Context, tenantID ids.ID) ([]model.NotificationTarget, error)
	DeleteTarget(ctx context.Context, tenantID, targetID ids.ID) error

	// Templates
	SaveTemplate(ctx context.Context, template *model.NotificationTemplate) (*model.NotificationTemplate, error)
	GetTemplate(ctx context.Context, tenantID, templateID ids.ID) (*model.NotificationTemplate, error)
	ListTemplates(ctx context.Context, tenantID ids.ID) ([]model.NotificationTemplate, error)
	DeleteTemplate(ctx context.Context, tenantID, templateID ids.ID) error

	// Rules
	SaveRule(ctx context.Context, rule *model.NotificationRule) (*model.NotificationRule, error)
	GetRule(ctx context.Context, tenantID, ruleID ids.ID) (*model.NotificationRule, error)
	ListRules(ctx context.Context, tenantID ids.ID) ([]model.NotificationRule, error)
	DeleteRule(ctx context.Context, tenantID, ruleID ids.ID) error

	// In-App Inbox
	ListInbox(ctx context.Context, tenantID, recipientID ids.ID, limit, offset int) ([]model.Notification, int, error)
	MarkRead(ctx context.Context, tenantID, notificationID ids.ID) error
	MarkAllRead(ctx context.Context, tenantID, recipientID ids.ID) error
	GetUnreadCount(ctx context.Context, tenantID, recipientID ids.ID) (int, error)

	// Delivery & Escalation
	SendNotification(ctx context.Context, tenantID ids.ID, req *model.NotificationSendRequest) (*model.NotificationRequest, error)
	ProcessAlarmEvent(ctx context.Context, eventType string, alarm *model.Alarm) error
	CancelAlarmEscalations(alarmID ids.ID)
	Start(ctx context.Context, b bus.Bus) error
}

type memoryService struct {
	mu            sync.RWMutex
	targets       map[string]*model.NotificationTarget
	templates     map[string]*model.NotificationTemplate
	rules         map[string]*model.NotificationRule
	inbox         map[string]*model.Notification
	smtpHost      string
	smtpPort      int
	httpClient    *http.Client
	escalationsMu sync.Mutex
	escalations   map[string]context.CancelFunc // key: alarmID
}

// Config defines mailer and channel options for notification delivery.
type Config struct {
	SMTPHost string
	SMTPPort int
}

// NewMemoryService creates an in-memory notification service.
func NewMemoryService(cfg Config) Service {
	if cfg.SMTPHost == "" {
		cfg.SMTPHost = "127.0.0.1"
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 1025 // Default Mailpit
	}
	return &memoryService{
		targets:     make(map[string]*model.NotificationTarget),
		templates:   make(map[string]*model.NotificationTemplate),
		rules:       make(map[string]*model.NotificationRule),
		inbox:       make(map[string]*model.Notification),
		smtpHost:    cfg.SMTPHost,
		smtpPort:    cfg.SMTPPort,
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		escalations: make(map[string]context.CancelFunc),
	}
}

// Target Management

func (s *memoryService) SaveTarget(ctx context.Context, t *model.NotificationTarget) (*model.NotificationTarget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.ID == ids.Nil {
		t.ID = ids.MustNew()
	}
	if t.CreatedTime == 0 {
		t.CreatedTime = time.Now().UnixMilli()
	}
	saved := *t
	s.targets[t.ID.String()] = &saved
	return &saved, nil
}

func (s *memoryService) GetTarget(ctx context.Context, tenantID, targetID ids.ID) (*model.NotificationTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.targets[targetID.String()]
	if !ok || t.TenantID != tenantID {
		return nil, errs.NotFound("notification target not found")
	}
	return t, nil
}

func (s *memoryService) ListTargets(ctx context.Context, tenantID ids.ID) ([]model.NotificationTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var res []model.NotificationTarget
	for _, t := range s.targets {
		if t.TenantID == tenantID {
			res = append(res, *t)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedTime > res[j].CreatedTime
	})
	return res, nil
}

func (s *memoryService) DeleteTarget(ctx context.Context, tenantID, targetID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.targets[targetID.String()]
	if !ok || t.TenantID != tenantID {
		return errs.NotFound("notification target not found")
	}
	delete(s.targets, targetID.String())
	return nil
}

// Template Management

func (s *memoryService) SaveTemplate(ctx context.Context, tpl *model.NotificationTemplate) (*model.NotificationTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tpl.ID == ids.Nil {
		tpl.ID = ids.MustNew()
	}
	if tpl.CreatedTime == 0 {
		tpl.CreatedTime = time.Now().UnixMilli()
	}
	saved := *tpl
	s.templates[tpl.ID.String()] = &saved
	return &saved, nil
}

func (s *memoryService) GetTemplate(ctx context.Context, tenantID, templateID ids.ID) (*model.NotificationTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tpl, ok := s.templates[templateID.String()]
	if !ok || tpl.TenantID != tenantID {
		return nil, errs.NotFound("notification template not found")
	}
	return tpl, nil
}

func (s *memoryService) ListTemplates(ctx context.Context, tenantID ids.ID) ([]model.NotificationTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var res []model.NotificationTemplate
	for _, tpl := range s.templates {
		if tpl.TenantID == tenantID {
			res = append(res, *tpl)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedTime > res[j].CreatedTime
	})
	return res, nil
}

func (s *memoryService) DeleteTemplate(ctx context.Context, tenantID, templateID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tpl, ok := s.templates[templateID.String()]
	if !ok || tpl.TenantID != tenantID {
		return errs.NotFound("notification template not found")
	}
	delete(s.templates, templateID.String())
	return nil
}

// Rule Management

func (s *memoryService) SaveRule(ctx context.Context, r *model.NotificationRule) (*model.NotificationRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.ID == ids.Nil {
		r.ID = ids.MustNew()
	}
	if r.CreatedTime == 0 {
		r.CreatedTime = time.Now().UnixMilli()
	}
	saved := *r
	s.rules[r.ID.String()] = &saved
	return &saved, nil
}

func (s *memoryService) GetRule(ctx context.Context, tenantID, ruleID ids.ID) (*model.NotificationRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.rules[ruleID.String()]
	if !ok || r.TenantID != tenantID {
		return nil, errs.NotFound("notification rule not found")
	}
	return r, nil
}

func (s *memoryService) ListRules(ctx context.Context, tenantID ids.ID) ([]model.NotificationRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var res []model.NotificationRule
	for _, r := range s.rules {
		if r.TenantID == tenantID {
			res = append(res, *r)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedTime > res[j].CreatedTime
	})
	return res, nil
}

func (s *memoryService) DeleteRule(ctx context.Context, tenantID, ruleID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.rules[ruleID.String()]
	if !ok || r.TenantID != tenantID {
		return errs.NotFound("notification rule not found")
	}
	delete(s.rules, ruleID.String())
	return nil
}

// In-App Inbox

func (s *memoryService) ListInbox(ctx context.Context, tenantID, recipientID ids.ID, limit, offset int) ([]model.Notification, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var matched []model.Notification
	for _, n := range s.inbox {
		if n.TenantID == tenantID && (recipientID == ids.Nil || n.RecipientID == recipientID) {
			matched = append(matched, *n)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedTime > matched[j].CreatedTime
	})

	total := len(matched)
	if offset >= total {
		return []model.Notification{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return matched[offset:end], total, nil
}

func (s *memoryService) MarkRead(ctx context.Context, tenantID, notificationID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.inbox[notificationID.String()]
	if !ok || n.TenantID != tenantID {
		return errs.NotFound("notification not found")
	}
	n.Status = model.NotificationRead
	return nil
}

func (s *memoryService) MarkAllRead(ctx context.Context, tenantID, recipientID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, n := range s.inbox {
		if n.TenantID == tenantID && (recipientID == ids.Nil || n.RecipientID == recipientID) {
			n.Status = model.NotificationRead
		}
	}
	return nil
}

func (s *memoryService) GetUnreadCount(ctx context.Context, tenantID, recipientID ids.ID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, n := range s.inbox {
		if n.TenantID == tenantID && (recipientID == ids.Nil || n.RecipientID == recipientID) && n.Status == model.NotificationUnread {
			count++
		}
	}
	return count, nil
}

// Template Variable Interpolation

func InterpolateTemplate(tpl string, vars map[string]string) string {
	res := tpl
	for k, v := range vars {
		res = strings.ReplaceAll(res, fmt.Sprintf("${%s}", k), v)
	}
	return res
}

// Delivery & Escalation Implementation

func (s *memoryService) deliverToTarget(ctx context.Context, tenantID ids.ID, t *model.NotificationTarget, subject, body string, info json.RawMessage) {
	switch t.Type {
	case model.TargetPlatformUsers:
		// Save inbox message for all users or specified recipients
		var cfg struct {
			UserIDs []string `json:"user_ids"`
		}
		_ = json.Unmarshal(t.Config, &cfg)

		recipients := cfg.UserIDs
		if len(recipients) == 0 {
			recipients = []string{ids.MustNew().String()}
		}

		s.mu.Lock()
		for _, u := range recipients {
			recID, _ := ids.Parse(u)
			msgID := ids.MustNew()
			s.inbox[msgID.String()] = &model.Notification{
				ID:          msgID,
				CreatedTime: time.Now().UnixMilli(),
				TenantID:    tenantID,
				RecipientID: recID,
				Type:        "ALARM",
				Subject:     subject,
				Body:        body,
				Info:        info,
				Status:      model.NotificationUnread,
			}
		}
		s.mu.Unlock()

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

func (s *memoryService) SendNotification(ctx context.Context, tenantID ids.ID, req *model.NotificationSendRequest) (*model.NotificationRequest, error) {
	s.mu.RLock()
	var targets []*model.NotificationTarget
	for _, tid := range req.Targets {
		if t, ok := s.targets[tid.String()]; ok && t.TenantID == tenantID {
			targets = append(targets, t)
		}
	}
	s.mu.RUnlock()

	for _, t := range targets {
		s.deliverToTarget(ctx, tenantID, t, req.Subject, req.Body, req.Info)
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

func (s *memoryService) CancelAlarmEscalations(alarmID ids.ID) {
	s.escalationsMu.Lock()
	defer s.escalationsMu.Unlock()
	if cancel, exists := s.escalations[alarmID.String()]; exists {
		cancel()
		delete(s.escalations, alarmID.String())
	}
}

func (s *memoryService) ProcessAlarmEvent(ctx context.Context, eventType string, a *model.Alarm) error {
	if a == nil {
		return nil
	}

	// Cancel escalation on ACK or CLEAR
	if eventType == "ALARM_ACKED" || eventType == bus.TypeAlarmCleared || a.Status.IncludesAck() || a.Status.IsCleared() {
		s.CancelAlarmEscalations(a.ID)
	}

	if eventType == bus.TypeAlarmCleared {
		return nil
	}

	s.mu.RLock()
	rules := make([]model.NotificationRule, 0)
	for _, r := range s.rules {
		if r.TenantID == a.TenantID && r.Enabled && (r.TriggerType == "ALARM" || r.TriggerType == eventType) {
			rules = append(rules, *r)
		}
	}
	s.mu.RUnlock()

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
		// Template lookup
		s.mu.RLock()
		tpl, hasTpl := s.templates[r.TemplateID.String()]
		s.mu.RUnlock()

		subject := fmt.Sprintf("[%s] %s Alert on Device %s", a.Severity, a.Type, a.OriginatorID.String())
		body := fmt.Sprintf("Alarm %s of severity %s was triggered at %s.", a.Type, a.Severity, vars["time"])

		if hasTpl {
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

		// Execute escalation tiers
		alarmKey := a.ID.String()
		escCtx, cancel := context.WithCancel(context.Background())
		s.escalationsMu.Lock()
		s.escalations[alarmKey] = cancel
		s.escalationsMu.Unlock()

		for _, step := range r.RecipientsConfig {
			stepCopy := step
			if stepCopy.DelaySeconds <= 0 {
				// Immediate delivery
				for _, tid := range stepCopy.TargetIDs {
					s.mu.RLock()
					target, ok := s.targets[tid.String()]
					s.mu.RUnlock()
					if ok {
						s.deliverToTarget(ctx, a.TenantID, target, subject, body, a.Details)
					}
				}
			} else {
				// Scheduled delayed escalation step
				go func(delaySec int, targetIDs []ids.ID) {
					select {
					case <-time.After(time.Duration(delaySec) * time.Second):
						for _, tid := range targetIDs {
							s.mu.RLock()
							target, ok := s.targets[tid.String()]
							s.mu.RUnlock()
							if ok {
								s.deliverToTarget(context.Background(), a.TenantID, target,
									fmt.Sprintf("[ESCALATION TIER] %s", subject),
									fmt.Sprintf("This alarm has not been acknowledged within %d seconds:\n%s", delaySec, body),
									a.Details)
							}
						}
					case <-escCtx.Done():
						// Escalation cancelled by Ack or Clear!
						return
					}
				}(stepCopy.DelaySeconds, stepCopy.TargetIDs)
			}
		}
	}

	return nil
}

func (s *memoryService) Start(ctx context.Context, b bus.Bus) error {
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
