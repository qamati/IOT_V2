package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestNotificationTemplateInterpolation(t *testing.T) {
	tpl := "ALERT: [${alarm.severity}] on ${device.id}: ${alarm.type} - details: ${alarm.details}"
	vars := map[string]string{
		"alarm.severity": "CRITICAL",
		"device.id":      "dev-1234",
		"alarm.type":     "Overheating",
		"alarm.details":  "temp: 105C",
	}

	result := InterpolateTemplate(tpl, vars)
	expected := "ALERT: [CRITICAL] on dev-1234: Overheating - details: temp: 105C"
	if result != expected {
		t.Fatalf("expected '%s', got '%s'", expected, result)
	}
}

func TestNotificationLifecycleAndInbox(t *testing.T) {
	ctx := context.Background()
	svc := NewMemoryService(Config{SMTPHost: "127.0.0.1", SMTPPort: 1025})

	tenantID := ids.MustNew()
	userID := ids.MustNew()

	// 1. Create Target (Platform Users)
	targetCfg, _ := json.Marshal(map[string]any{
		"user_ids": []string{userID.String()},
	})
	target, err := svc.SaveTarget(ctx, &model.NotificationTarget{
		TenantID: tenantID,
		Name:     "Maintenance Team",
		Type:     model.TargetPlatformUsers,
		Config:   targetCfg,
	})
	if err != nil {
		t.Fatalf("SaveTarget failed: %v", err)
	}

	// 2. Create Template
	templateCfg, _ := json.Marshal(map[string]any{
		"subject": "CRITICAL ALARM: ${alarm.type}",
		"body":    "Device ${device.id} triggered ${alarm.type} with severity ${alarm.severity}.",
	})
	template, err := svc.SaveTemplate(ctx, &model.NotificationTemplate{
		TenantID: tenantID,
		Name:     "Alarm Notification Template",
		Type:     "ALARM",
		Config:   templateCfg,
	})
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}

	// 3. Create Rule with escalation
	rule, err := svc.SaveRule(ctx, &model.NotificationRule{
		TenantID:    tenantID,
		Name:        "High Severity Alarm Rule",
		Enabled:     true,
		TemplateID:  template.ID,
		TriggerType: "ALARM",
		RecipientsConfig: []model.EscalationStep{
			{Step: 1, DelaySeconds: 0, TargetIDs: []ids.ID{target.ID}},
			{Step: 2, DelaySeconds: 300, TargetIDs: []ids.ID{target.ID}},
		},
	})
	if err != nil {
		t.Fatalf("SaveRule failed: %v", err)
	}
	if rule.ID == ids.Nil {
		t.Fatalf("expected rule ID to be generated")
	}

	// 4. Trigger Alarm Event -> ProcessAlarmEvent
	alarmID := ids.MustNew()
	devID := ids.MustNew()
	testAlarm := &model.Alarm{
		ID:           alarmID,
		TenantID:     tenantID,
		OriginatorID: devID,
		Type:         "High Temp",
		Severity:     model.SeverityCritical,
		Status:       model.AlarmActiveUnack,
		CreatedTime:  time.Now().UnixMilli(),
		StartTs:      time.Now().UnixMilli(),
	}

	err = svc.ProcessAlarmEvent(ctx, bus.TypeAlarmCreated, testAlarm)
	if err != nil {
		t.Fatalf("ProcessAlarmEvent failed: %v", err)
	}

	// 5. Check In-App Inbox for user
	inbox, count, err := svc.ListInbox(ctx, tenantID, userID, 10, 0)
	if err != nil {
		t.Fatalf("ListInbox failed: %v", err)
	}
	if count != 1 || len(inbox) != 1 {
		t.Fatalf("expected 1 notification in inbox, got count=%d, len=%d", count, len(inbox))
	}
	if inbox[0].Subject != "CRITICAL ALARM: High Temp" {
		t.Fatalf("unexpected subject: %s", inbox[0].Subject)
	}
	if inbox[0].Status != model.NotificationUnread {
		t.Fatalf("expected unread notification, got %s", inbox[0].Status)
	}

	// 6. Check Unread Count
	unread, err := svc.GetUnreadCount(ctx, tenantID, userID)
	if err != nil || unread != 1 {
		t.Fatalf("expected unread count 1, got %d", unread)
	}

	// 7. Mark as Read
	err = svc.MarkRead(ctx, tenantID, inbox[0].ID)
	if err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}
	unread, _ = svc.GetUnreadCount(ctx, tenantID, userID)
	if unread != 0 {
		t.Fatalf("expected unread count 0 after mark read, got %d", unread)
	}

	// 8. Test Escalation Cancellation on Ack
	testAlarm.Status = model.AlarmActiveAck
	err = svc.ProcessAlarmEvent(ctx, "ALARM_ACKED", testAlarm)
	if err != nil {
		t.Fatalf("ProcessAlarmEvent ALARM_ACKED failed: %v", err)
	}

	// 9. Mark All Read
	err = svc.MarkAllRead(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("MarkAllRead failed: %v", err)
	}
}
