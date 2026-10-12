package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
)

// --- action.save_timeseries ---

type SaveTimeseriesConfig struct {
	UseServerTs bool `json:"useServerTs,omitempty"`
}

type SaveTimeseriesNode struct {
	cfg SaveTimeseriesConfig
}

func (n *SaveTimeseriesNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	return nil
}

func (n *SaveTimeseriesNode) OnMsg(ctx NodeCtx, msg *Message) {
	if ctx.Services() == nil || ctx.Services().Telemetry == nil {
		ctx.TellNext(msg, RelationFailure)
		return
	}

	ts := msg.Ts
	if n.cfg.UseServerTs || ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	points, err := telemetry.ParsePayload(msg.Data, ts)
	if err != nil {
		ctx.Log().Error("failed to parse telemetry points in save_timeseries node", "error", err)
		ctx.TellFailure(msg, err)
		return
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ctx.Services().Telemetry.Save(reqCtx, msg.Originator.ID, points); err != nil {
		ctx.Log().Error("failed to persist timeseries points", "originator", msg.Originator.ID, "error", err)
		ctx.TellFailure(msg, err)
		return
	}

	ctx.TellNext(msg, RelationSuccess)
}

func (n *SaveTimeseriesNode) Close() {}

// --- action.save_attributes ---

type SaveAttributesConfig struct {
	Scope string `json:"scope,omitempty"` // SERVER_SCOPE, SHARED_SCOPE, CLIENT_SCOPE
}

type SaveAttributesNode struct {
	cfg SaveAttributesConfig
}

func (n *SaveAttributesNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	if n.cfg.Scope == "" {
		n.cfg.Scope = string(model.ScopeServer)
	}
	return nil
}

func (n *SaveAttributesNode) OnMsg(ctx NodeCtx, msg *Message) {
	if ctx.Services() == nil || ctx.Services().Attributes == nil {
		ctx.TellNext(msg, RelationFailure)
		return
	}

	var attrs map[string]any
	if err := json.Unmarshal(msg.Data, &attrs); err != nil {
		ctx.Log().Error("failed to unmarshal attributes payload", "error", err)
		ctx.TellFailure(msg, err)
		return
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scope := model.AttributeScope(n.cfg.Scope)
	if err := ctx.Services().Attributes.SaveAttributes(reqCtx, msg.Originator.ID, scope, attrs); err != nil {
		ctx.Log().Error("failed to persist attributes", "originator", msg.Originator.ID, "error", err)
		ctx.TellFailure(msg, err)
		return
	}

	ctx.TellNext(msg, RelationSuccess)
}

func (n *SaveAttributesNode) Close() {}

// --- action.create_alarm ---

type CreateAlarmConfig struct {
	AlarmType string `json:"alarmType"`
	Severity  string `json:"severity"` // CRITICAL, MAJOR, MINOR, WARNING, INDETERMINATE
	Details   string `json:"details,omitempty"`
}

type CreateAlarmNode struct {
	cfg CreateAlarmConfig
}

func (n *CreateAlarmNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &n.cfg); err != nil {
			return fmt.Errorf("invalid action.create_alarm config: %w", err)
		}
	}
	if n.cfg.AlarmType == "" {
		n.cfg.AlarmType = "General Alarm"
	}
	if n.cfg.Severity == "" {
		n.cfg.Severity = string(model.SeverityCritical)
	}
	return nil
}

func (n *CreateAlarmNode) OnMsg(ctx NodeCtx, msg *Message) {
	if ctx.Services() == nil || ctx.Services().Alarms == nil {
		ctx.TellNext(msg, RelationFailure)
		return
	}

	details := json.RawMessage(msg.Data)
	if n.cfg.Details != "" {
		details = json.RawMessage(fmt.Sprintf(`{"message":%q,"data":%s}`, n.cfg.Details, string(msg.Data)))
	}

	alarmObj := &model.Alarm{
		TenantID:     msg.TenantID,
		OriginatorID: msg.Originator.ID,
		Type:         n.cfg.AlarmType,
		Severity:     model.AlarmSeverity(n.cfg.Severity),
		Details:      details,
		StartTs:      msg.Ts,
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := ctx.Services().Alarms.CreateOrUpdateAlarm(reqCtx, alarmObj)
	if err != nil {
		ctx.Log().Error("failed to create or update alarm", "error", err)
		ctx.TellFailure(msg, err)
		return
	}

	if res != nil && res.Status == model.AlarmActiveUnack {
		ctx.TellNext(msg, RelationCreated)
	} else {
		ctx.TellNext(msg, RelationUpdated)
	}
}

func (n *CreateAlarmNode) Close() {}

// --- action.clear_alarm ---

type ClearAlarmConfig struct {
	AlarmType string `json:"alarmType"`
}

type ClearAlarmNode struct {
	cfg ClearAlarmConfig
}

func (n *ClearAlarmNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	if n.cfg.AlarmType == "" {
		n.cfg.AlarmType = "General Alarm"
	}
	return nil
}

func (n *ClearAlarmNode) OnMsg(ctx NodeCtx, msg *Message) {
	if ctx.Services() == nil || ctx.Services().Alarms == nil {
		ctx.TellNext(msg, RelationFailure)
		return
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	alarms, _, err := ctx.Services().Alarms.ListAlarms(reqCtx, msg.TenantID, &msg.Originator.ID, 1, 50)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	clearedAny := false
	for _, a := range alarms {
		if a.Type == n.cfg.AlarmType && (a.Status == model.AlarmActiveUnack || a.Status == model.AlarmActiveAck) {
			_, clrErr := ctx.Services().Alarms.ClearAlarm(reqCtx, msg.TenantID, a.ID)
			if clrErr == nil {
				clearedAny = true
			}
		}
	}

	if clearedAny {
		ctx.TellNext(msg, RelationCleared)
	} else {
		ctx.TellNext(msg, RelationFalse)
	}
}

func (n *ClearAlarmNode) Close() {}

// --- action.log ---

type LogNodeConfig struct {
	Level string `json:"level,omitempty"` // INFO, WARN, ERROR, DEBUG
}

type LogNode struct {
	cfg LogNodeConfig
}

func (n *LogNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	return nil
}

func (n *LogNode) OnMsg(ctx NodeCtx, msg *Message) {
	logger := ctx.Log()
	logAttrs := []any{
		"msg_id", msg.ID,
		"tenant_id", msg.TenantID,
		"type", msg.Type,
		"originator", msg.Originator.ID,
		"data", string(msg.Data),
	}

	switch n.cfg.Level {
	case "WARN":
		logger.Warn("rule engine action.log", logAttrs...)
	case "ERROR":
		logger.Error("rule engine action.log", logAttrs...)
	case "DEBUG":
		logger.Debug("rule engine action.log", logAttrs...)
	default:
		logger.Info("rule engine action.log", logAttrs...)
	}

	ctx.TellNext(msg, RelationSuccess)
}

func (n *LogNode) Close() {}

func registerActionNodes(c *Catalog) {
	c.Register(Descriptor{
		Type:      "action.save_timeseries",
		Name:      "Save Time-Series",
		Category:  "ACTION",
		Relations: []string{RelationSuccess, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"useServerTs": {"type": "boolean", "default": false, "title": "Use Server Timestamp"}
			}
		}`),
		Version: 1,
		Docs:    "Persists parsed telemetry data points to TimescaleDB or active time-series store.",
	}, func() Node { return &SaveTimeseriesNode{} })

	c.Register(Descriptor{
		Type:      "action.save_attributes",
		Name:      "Save Attributes",
		Category:  "ACTION",
		Relations: []string{RelationSuccess, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"scope": {"type": "string", "enum": ["SERVER_SCOPE", "SHARED_SCOPE", "CLIENT_SCOPE"], "default": "SERVER_SCOPE", "title": "Attribute Scope"}
			}
		}`),
		Version: 1,
		Docs:    "Persists entity attributes to durable PostgreSQL store.",
	}, func() Node { return &SaveAttributesNode{} })

	c.Register(Descriptor{
		Type:      "action.create_alarm",
		Name:      "Create / Update Alarm",
		Category:  "ACTION",
		Relations: []string{RelationCreated, RelationUpdated, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"alarmType": {"type": "string", "default": "High Temperature Alert", "title": "Alarm Type"},
				"severity": {"type": "string", "enum": ["CRITICAL", "MAJOR", "MINOR", "WARNING", "INDETERMINATE"], "default": "CRITICAL", "title": "Severity"},
				"details": {"type": "string", "title": "Alarm Details Note"}
			},
			"required": ["alarmType", "severity"]
		}`),
		Version: 1,
		Docs:    "Creates or updates an active alarm on the originator entity with given type and severity.",
	}, func() Node { return &CreateAlarmNode{} })

	c.Register(Descriptor{
		Type:      "action.clear_alarm",
		Name:      "Clear Alarm",
		Category:  "ACTION",
		Relations: []string{RelationCleared, RelationFalse, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"alarmType": {"type": "string", "title": "Alarm Type to Clear"}
			},
			"required": ["alarmType"]
		}`),
		Version: 1,
		Docs:    "Clears any active alarm matching the specified type on the originator entity.",
	}, func() Node { return &ClearAlarmNode{} })

	c.Register(Descriptor{
		Type:      "action.log",
		Name:      "Log Message",
		Category:  "ACTION",
		Relations: []string{RelationSuccess},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"level": {"type": "string", "enum": ["INFO", "WARN", "ERROR", "DEBUG"], "default": "INFO", "title": "Log Level"}
			}
		}`),
		Version: 1,
		Docs:    "Emits structured log entry for the current message and passes execution along Success relation.",
	}, func() Node { return &LogNode{} })
}
