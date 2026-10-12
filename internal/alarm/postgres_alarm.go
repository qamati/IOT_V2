package alarm

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// PostgresService persists alarm lifecycle states and alarm rules in PostgreSQL.
type PostgresService struct {
	pool      *pgxpool.Pool
	evaluator *Evaluator
	busMu     sync.RWMutex
	bus       bus.Bus
}

// NewPostgresService creates a new PostgreSQL-backed Alarm Service.
func NewPostgresService(pool *pgxpool.Pool) Service {
	return &PostgresService{
		pool:      pool,
		evaluator: NewEvaluator(),
	}
}

func (s *PostgresService) SetEventBus(b bus.Bus) {
	s.busMu.Lock()
	defer s.busMu.Unlock()
	s.bus = b
}

func (s *PostgresService) publishAlarmEvent(ctx context.Context, eventType string, a *model.Alarm) {
	s.busMu.RLock()
	b := s.bus
	s.busMu.RUnlock()
	if b == nil || a == nil {
		return
	}
	payload, _ := json.Marshal(a)
	msg := bus.NewMessage(a.TenantID.String(), "DEVICE", a.OriginatorID.String(), eventType, payload)
	_ = b.Publish(ctx, TopicAlarmEvents, msg)
}

func (s *PostgresService) ensureTenantExists(ctx context.Context, tenantID ids.ID) (ids.ID, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tenant WHERE id = $1)", tenantID).Scan(&exists)
	if err == nil && exists {
		return tenantID, nil
	}

	var primaryTenant ids.ID
	err = s.pool.QueryRow(ctx, "SELECT id FROM tenant ORDER BY created_time ASC LIMIT 1").Scan(&primaryTenant)
	if err != nil {
		return tenantID, nil
	}
	return primaryTenant, nil
}

func (s *PostgresService) CreateOrUpdateAlarm(ctx context.Context, in *model.Alarm) (*model.Alarm, error) {
	now := time.Now().UnixMilli()

	// 1. Look for existing un-cleared alarm of the same type and originator (deduplication)
	var existing model.Alarm
	var detailsBytes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, originator_id, type, severity, status, start_ts,
		       COALESCE(end_ts, 0), COALESCE(ack_ts, 0), COALESCE(clear_ts, 0), details
		FROM alarm
		WHERE tenant_id = $1 AND originator_id = $2 AND type = $3 AND cleared = false
		LIMIT 1
	`, in.TenantID, in.OriginatorID, in.Type).Scan(
		&existing.ID, &existing.CreatedTime, &existing.TenantID, &existing.OriginatorID,
		&existing.Type, &existing.Severity, &existing.Status, &existing.StartTs, &existing.EndTs,
		&existing.AckTs, &existing.ClearTs, &detailsBytes,
	)

	if err == nil {
		// Update existing active alarm
		if len(in.Details) > 0 {
			detailsBytes = in.Details
		}
		existing.EndTs = now
		existing.Severity = in.Severity
		existing.Details = detailsBytes

		_, updateErr := s.pool.Exec(ctx, `
			UPDATE alarm
			SET end_ts = $1, severity = $2, details = $3, version = version + 1
			WHERE id = $4
		`, now, in.Severity, detailsBytes, existing.ID)
		if updateErr != nil {
			return nil, updateErr
		}
		go s.publishAlarmEvent(context.Background(), bus.TypeAlarmCreated, &existing)
		return &existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 2. Create new alarm
	if in.ID == ids.Nil {
		in.ID = ids.MustNew()
	}
	if in.CreatedTime == 0 {
		in.CreatedTime = now
	}
	if in.StartTs == 0 {
		in.StartTs = now
	}
	if in.EndTs == 0 {
		in.EndTs = now
	}
	if in.Status == "" {
		in.Status = model.AlarmActiveUnack
	}
	if len(in.Details) == 0 {
		in.Details = json.RawMessage(`{}`)
	}

	// Safety check for foreign key tenant
	safeTenant, _ := s.ensureTenantExists(ctx, in.TenantID)
	in.TenantID = safeTenant

	_, insertErr := s.pool.Exec(ctx, `
		INSERT INTO alarm (
			id, created_time, tenant_id, type, originator_id, originator_type,
			severity, status, acknowledged, cleared, start_ts, end_ts, details, version
		) VALUES (
			$1, $2, $3, $4, $5, 'DEVICE',
			$6, $7, false, false, $8, $9, $10, 1
		)
	`, in.ID, in.CreatedTime, in.TenantID, in.Type, in.OriginatorID,
		in.Severity, in.Status, in.StartTs, in.EndTs, in.Details)
	if insertErr != nil {
		return nil, insertErr
	}

	go s.publishAlarmEvent(context.Background(), bus.TypeAlarmCreated, in)
	return in, nil
}

func (s *PostgresService) AckAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error) {
	now := time.Now().UnixMilli()

	var a model.Alarm
	var detailsBytes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, originator_id, type, severity, status, start_ts,
		       COALESCE(end_ts, 0), COALESCE(ack_ts, 0), COALESCE(clear_ts, 0), details
		FROM alarm
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, alarmID).Scan(
		&a.ID, &a.CreatedTime, &a.TenantID, &a.OriginatorID,
		&a.Type, &a.Severity, &a.Status, &a.StartTs, &a.EndTs,
		&a.AckTs, &a.ClearTs, &detailsBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("alarm not found")
		}
		return nil, err
	}

	switch a.Status {
	case model.AlarmActiveUnack:
		a.Status = model.AlarmActiveAck
	case model.AlarmClearedUnack:
		a.Status = model.AlarmClearedAck
	}
	a.AckTs = now
	a.Details = detailsBytes

	_, err = s.pool.Exec(ctx, `
		UPDATE alarm
		SET status = $1, acknowledged = true, ack_ts = $2, version = version + 1
		WHERE id = $3
	`, a.Status, now, alarmID)
	if err != nil {
		return nil, err
	}

	go s.publishAlarmEvent(context.Background(), "ALARM_ACKED", &a)
	return &a, nil
}

func (s *PostgresService) ClearAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error) {
	now := time.Now().UnixMilli()

	var a model.Alarm
	var detailsBytes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, originator_id, type, severity, status, start_ts,
		       COALESCE(end_ts, 0), COALESCE(ack_ts, 0), COALESCE(clear_ts, 0), details
		FROM alarm
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, alarmID).Scan(
		&a.ID, &a.CreatedTime, &a.TenantID, &a.OriginatorID,
		&a.Type, &a.Severity, &a.Status, &a.StartTs, &a.EndTs,
		&a.AckTs, &a.ClearTs, &detailsBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("alarm not found")
		}
		return nil, err
	}

	switch a.Status {
	case model.AlarmActiveUnack:
		a.Status = model.AlarmClearedUnack
	case model.AlarmActiveAck:
		a.Status = model.AlarmClearedAck
	}
	a.ClearTs = now
	a.Details = detailsBytes

	_, err = s.pool.Exec(ctx, `
		UPDATE alarm
		SET status = $1, cleared = true, clear_ts = $2, version = version + 1
		WHERE id = $3
	`, a.Status, now, alarmID)
	if err != nil {
		return nil, err
	}

	go s.publishAlarmEvent(context.Background(), bus.TypeAlarmCleared, &a)
	return &a, nil
}

func (s *PostgresService) GetAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error) {
	var a model.Alarm
	var detailsBytes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, originator_id, type, severity, status, start_ts,
		       COALESCE(end_ts, 0), COALESCE(ack_ts, 0), COALESCE(clear_ts, 0), details
		FROM alarm
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, alarmID).Scan(
		&a.ID, &a.CreatedTime, &a.TenantID, &a.OriginatorID,
		&a.Type, &a.Severity, &a.Status, &a.StartTs, &a.EndTs,
		&a.AckTs, &a.ClearTs, &detailsBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("alarm not found")
		}
		return nil, err
	}
	a.Details = detailsBytes
	return &a, nil
}

func (s *PostgresService) ListAlarms(ctx context.Context, tenantID ids.ID, originatorID *ids.ID, page, pageSize int) ([]model.Alarm, int, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := page * pageSize

	var total int
	var countQuery string
	var listQuery string

	if originatorID != nil && *originatorID != ids.Nil {
		countQuery = `SELECT count(*) FROM alarm WHERE tenant_id = $1 AND originator_id = $2`
		listQuery = `
			SELECT id, created_time, tenant_id, originator_id, type, severity, status, start_ts,
			       COALESCE(end_ts, 0), COALESCE(ack_ts, 0), COALESCE(clear_ts, 0), details
			FROM alarm
			WHERE tenant_id = $1 AND originator_id = $2
			ORDER BY created_time DESC
			LIMIT $3 OFFSET $4
		`
		if err := s.pool.QueryRow(ctx, countQuery, tenantID, *originatorID).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else {
		countQuery = `SELECT count(*) FROM alarm WHERE tenant_id = $1`
		listQuery = `
			SELECT id, created_time, tenant_id, originator_id, type, severity, status, start_ts,
			       COALESCE(end_ts, 0), COALESCE(ack_ts, 0), COALESCE(clear_ts, 0), details
			FROM alarm
			WHERE tenant_id = $1
			ORDER BY created_time DESC
			LIMIT $2 OFFSET $3
		`
		if err := s.pool.QueryRow(ctx, countQuery, tenantID).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	var rows pgx.Rows
	var err error
	if originatorID != nil && *originatorID != ids.Nil {
		rows, err = s.pool.Query(ctx, listQuery, tenantID, *originatorID, pageSize, offset)
	} else {
		rows, err = s.pool.Query(ctx, listQuery, tenantID, pageSize, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []model.Alarm
	for rows.Next() {
		var a model.Alarm
		var detailsBytes []byte
		if err := rows.Scan(
			&a.ID, &a.CreatedTime, &a.TenantID, &a.OriginatorID,
			&a.Type, &a.Severity, &a.Status, &a.StartTs, &a.EndTs,
			&a.AckTs, &a.ClearTs, &detailsBytes,
		); err != nil {
			return nil, 0, err
		}
		a.Details = detailsBytes
		result = append(result, a)
	}
	if result == nil {
		result = make([]model.Alarm, 0)
	}
	return result, total, nil
}

// Stage 5 Alarm Rules Persistence

func (s *PostgresService) SaveAlarmRule(ctx context.Context, rule *model.AlarmRule) (*model.AlarmRule, error) {
	if rule.ID == ids.Nil {
		rule.ID = ids.MustNew()
	}
	if rule.CreatedTime == 0 {
		rule.CreatedTime = time.Now().UnixMilli()
	}
	if rule.Version == 0 {
		rule.Version = 1
	}
	if rule.Scope == "" {
		rule.Scope = "DEVICE_PROFILE"
	}
	if len(rule.Propagate) == 0 {
		rule.Propagate = json.RawMessage(`{"toOwner": true, "toTenant": true}`)
	}
	if len(rule.AdditionalInfo) == 0 {
		rule.AdditionalInfo = json.RawMessage(`{}`)
	}

	safeTenant, _ := s.ensureTenantExists(ctx, rule.TenantID)
	rule.TenantID = safeTenant

	createRulesBytes, err := json.Marshal(rule.CreateRules)
	if err != nil {
		return nil, err
	}

	var clearRuleBytes []byte
	if rule.ClearRule != nil {
		clearRuleBytes, _ = json.Marshal(rule.ClearRule)
	}

	query := `
		INSERT INTO alarm_rule (
			id, created_time, tenant_id, name, scope, target_id, alarm_type,
			enabled, create_rules, clear_rule, propagate, additional_info, version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		ON CONFLICT (tenant_id, name) DO UPDATE SET
			scope = EXCLUDED.scope,
			target_id = EXCLUDED.target_id,
			alarm_type = EXCLUDED.alarm_type,
			enabled = EXCLUDED.enabled,
			create_rules = EXCLUDED.create_rules,
			clear_rule = EXCLUDED.clear_rule,
			propagate = EXCLUDED.propagate,
			additional_info = EXCLUDED.additional_info,
			version = alarm_rule.version + 1
		RETURNING version
	`

	err = s.pool.QueryRow(ctx, query,
		rule.ID, rule.CreatedTime, rule.TenantID, rule.Name, rule.Scope, rule.TargetID, rule.AlarmType,
		rule.Enabled, createRulesBytes, clearRuleBytes, rule.Propagate, rule.AdditionalInfo, rule.Version,
	).Scan(&rule.Version)
	if err != nil {
		return nil, err
	}

	return rule, nil
}

func (s *PostgresService) GetAlarmRule(ctx context.Context, tenantID, ruleID ids.ID) (*model.AlarmRule, error) {
	query := `
		SELECT id, created_time, tenant_id, name, scope, target_id, alarm_type,
		       enabled, create_rules, clear_rule, propagate, additional_info, version
		FROM alarm_rule
		WHERE tenant_id = $1 AND id = $2
	`
	var r model.AlarmRule
	var createRulesBytes, clearRuleBytes []byte
	err := s.pool.QueryRow(ctx, query, tenantID, ruleID).Scan(
		&r.ID, &r.CreatedTime, &r.TenantID, &r.Name, &r.Scope, &r.TargetID, &r.AlarmType,
		&r.Enabled, &createRulesBytes, &clearRuleBytes, &r.Propagate, &r.AdditionalInfo, &r.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("alarm rule not found")
		}
		return nil, err
	}

	_ = json.Unmarshal(createRulesBytes, &r.CreateRules)
	if len(clearRuleBytes) > 0 {
		var cr model.AlarmRuleCreateDef
		if err := json.Unmarshal(clearRuleBytes, &cr); err == nil {
			r.ClearRule = &cr
		}
	}

	return &r, nil
}

func (s *PostgresService) ListAlarmRules(ctx context.Context, tenantID ids.ID) ([]model.AlarmRule, error) {
	query := `
		SELECT id, created_time, tenant_id, name, scope, target_id, alarm_type,
		       enabled, create_rules, clear_rule, propagate, additional_info, version
		FROM alarm_rule
		WHERE tenant_id = $1
		ORDER BY created_time DESC
	`
	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]model.AlarmRule, 0)
	for rows.Next() {
		var r model.AlarmRule
		var createRulesBytes, clearRuleBytes []byte
		if err := rows.Scan(
			&r.ID, &r.CreatedTime, &r.TenantID, &r.Name, &r.Scope, &r.TargetID, &r.AlarmType,
			&r.Enabled, &createRulesBytes, &clearRuleBytes, &r.Propagate, &r.AdditionalInfo, &r.Version,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(createRulesBytes, &r.CreateRules)
		if len(clearRuleBytes) > 0 {
			var cr model.AlarmRuleCreateDef
			if err := json.Unmarshal(clearRuleBytes, &cr); err == nil {
				r.ClearRule = &cr
			}
		}
		rules = append(rules, r)
	}

	return rules, nil
}

func (s *PostgresService) DeleteAlarmRule(ctx context.Context, tenantID, ruleID ids.ID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM alarm_rule WHERE tenant_id = $1 AND id = $2`, tenantID, ruleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("alarm rule not found")
	}
	return nil
}

func (s *PostgresService) TestAlarmRule(ctx context.Context, req *model.AlarmRuleTestRequest) (*model.AlarmRuleTestResponse, error) {
	return TestSimulation(req), nil
}

func (s *PostgresService) EvaluateTelemetry(ctx context.Context, tenantID, deviceID ids.ID, ts int64, values map[string]any) ([]*model.Alarm, error) {
	rules, err := s.ListAlarmRules(ctx, tenantID)
	if err != nil || len(rules) == 0 {
		return nil, err
	}

	severities := []model.AlarmSeverity{
		model.SeverityCritical,
		model.SeverityMajor,
		model.SeverityMinor,
		model.SeverityWarning,
		model.SeverityIndeterminate,
	}

	var triggeredAlarms []*model.Alarm

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		matchedSeverity := model.AlarmSeverity("")
		var matchedReason string
		var detailsTemplate string

		for _, sev := range severities {
			createDef, exists := rule.CreateRules[sev]
			if !exists {
				continue
			}

			matched, reason, _, _ := s.evaluator.EvaluateCondition(deviceID, rule.ID, sev, createDef.Condition, ts, values)
			if matched {
				matchedSeverity = sev
				matchedReason = reason
				detailsTemplate = createDef.Details
				break
			}
		}

		if matchedSeverity != "" {
			// Trigger or Update Alarm
			detailsMap := map[string]any{
				"rule_id":   rule.ID.String(),
				"rule_name": rule.Name,
				"reason":    matchedReason,
				"values":    values,
			}
			if detailsTemplate != "" {
				detailsMap["template"] = detailsTemplate
			}
			detailsBytes, _ := json.Marshal(detailsMap)

			al := &model.Alarm{
				TenantID:     tenantID,
				OriginatorID: deviceID,
				Type:         rule.AlarmType,
				Severity:     matchedSeverity,
				StartTs:      ts,
				EndTs:        ts,
				Details:      detailsBytes,
			}
			saved, err := s.CreateOrUpdateAlarm(ctx, al)
			if err == nil && saved != nil {
				triggeredAlarms = append(triggeredAlarms, saved)
			}
		} else if rule.ClearRule != nil {
			// Test Clear Rule
			matched, _, _, _ := s.evaluator.EvaluateCondition(deviceID, rule.ID, "CLEAR", rule.ClearRule.Condition, ts, values)
			if matched {
				// Query active alarm if exists
				var activeAlarmID ids.ID
				err := s.pool.QueryRow(ctx, `
					SELECT id FROM alarm WHERE tenant_id = $1 AND originator_id = $2 AND type = $3 AND cleared = false LIMIT 1
				`, tenantID, deviceID, rule.AlarmType).Scan(&activeAlarmID)
				if err == nil && activeAlarmID != ids.Nil {
					cleared, err := s.ClearAlarm(ctx, tenantID, activeAlarmID)
					if err == nil && cleared != nil {
						triggeredAlarms = append(triggeredAlarms, cleared)
					}
				}
			}
		}
	}

	return triggeredAlarms, nil
}
