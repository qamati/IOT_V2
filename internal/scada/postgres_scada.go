package scada

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// PostgresService implements industrial SCADA tag processing with durable PostgreSQL persistence.
type PostgresService struct {
	pool     *pgxpool.Pool
	mu       sync.RWMutex
	controls map[ids.ID]*ControlCommand
}

// NewPostgresService creates a PostgreSQL-backed SCADA service.
func NewPostgresService(pool *pgxpool.Pool) Service {
	svc := &PostgresService{
		pool:     pool,
		controls: make(map[ids.ID]*ControlCommand),
	}

	// Asynchronously seed default ThingsBoard symbols if not present
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = svc.SeedDefaultSymbols(ctx)
	}()

	return svc
}

func (s *PostgresService) SeedDefaultSymbols(ctx context.Context) error {
	for _, sym := range DefaultSymbols() {
		tagsJSON, _ := json.Marshal(sym.Tags)
		propsJSON, _ := json.Marshal(sym.Properties)
		behaviorsJSON, _ := json.Marshal(sym.Behaviors)
		animationsJSON, _ := json.Marshal(sym.Animations)

		_, _ = s.pool.Exec(ctx, `
			INSERT INTO scada_symbol (
				id, tenant_id, name, category, style, svg_content,
				tags, properties, behaviors, animations, version
			) VALUES (
				$1, NULL, $2, $3, $4, $5,
				$6, $7, $8, $9, 1
			)
			ON CONFLICT DO NOTHING
		`, sym.ID, sym.Name, sym.Category, sym.Style, sym.SVGContent,
			tagsJSON, propsJSON, behaviorsJSON, animationsJSON)
	}
	return nil
}

// ----------------------------------------------------------------------------
// 1. Tag Namespace & Quality Engine
// ----------------------------------------------------------------------------

func (s *PostgresService) RegisterTag(ctx context.Context, tag *Tag) error {
	if tag.ID == ids.Nil {
		tag.ID = ids.MustNew()
	}
	if tag.Quality == "" {
		tag.Quality = QualityGood
	}
	if tag.Timestamp == 0 {
		tag.Timestamp = time.Now().UnixMilli()
	}
	if tag.ScaleFactor == 0 {
		tag.ScaleFactor = 1.0
	}
	if tag.ScanRateMs == 0 {
		tag.ScanRateMs = 1000
	}
	tag.Version = 1

	valBytes, _ := json.Marshal(tag.Value)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO scada_tag (
			id, created_time, tenant_id, path, data_type, eng_unit,
			deadband, scale_factor, scale_offset, eu_min, eu_max, scan_rate_ms,
			val_json, quality, timestamp, version
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, 1
		)
		ON CONFLICT (tenant_id, path) DO UPDATE SET
			val_json = EXCLUDED.val_json,
			quality = EXCLUDED.quality,
			timestamp = EXCLUDED.timestamp,
			deadband = EXCLUDED.deadband,
			scale_factor = EXCLUDED.scale_factor,
			version = scada_tag.version + 1
	`, tag.ID, tag.Timestamp, tag.TenantID, tag.Path, tag.DataType, tag.EngUnit,
		tag.Deadband, tag.ScaleFactor, tag.ScaleOffset, tag.EUMin, tag.EUMax, tag.ScanRateMs,
		valBytes, tag.Quality, tag.Timestamp)
	return err
}

func (s *PostgresService) GetTag(ctx context.Context, tenantID, tagID ids.ID) (*Tag, error) {
	var t Tag
	var valBytes []byte
	var engUnit *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, path, data_type, eng_unit, deadband, scale_factor, scale_offset,
		       eu_min, eu_max, scan_rate_ms, val_json, quality, timestamp, version
		FROM scada_tag
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, tagID).Scan(
		&t.ID, &t.TenantID, &t.Path, &t.DataType, &engUnit, &t.Deadband, &t.ScaleFactor, &t.ScaleOffset,
		&t.EUMin, &t.EUMax, &t.ScanRateMs, &valBytes, &t.Quality, &t.Timestamp, &t.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("tag not found")
		}
		return nil, err
	}
	if engUnit != nil {
		t.EngUnit = *engUnit
	}
	if len(valBytes) > 0 {
		_ = json.Unmarshal(valBytes, &t.Value)
	}
	return &t, nil
}

func (s *PostgresService) GetTagByPath(ctx context.Context, tenantID ids.ID, path string) (*Tag, error) {
	var t Tag
	var valBytes []byte
	var engUnit *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, path, data_type, eng_unit, deadband, scale_factor, scale_offset,
		       eu_min, eu_max, scan_rate_ms, val_json, quality, timestamp, version
		FROM scada_tag
		WHERE (tenant_id = $1 OR tenant_id IS NULL) AND path = $2
		LIMIT 1
	`, tenantID, path).Scan(
		&t.ID, &t.TenantID, &t.Path, &t.DataType, &engUnit, &t.Deadband, &t.ScaleFactor, &t.ScaleOffset,
		&t.EUMin, &t.EUMax, &t.ScanRateMs, &valBytes, &t.Quality, &t.Timestamp, &t.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound(fmt.Sprintf("tag '%s' not found", path))
		}
		return nil, err
	}
	if engUnit != nil {
		t.EngUnit = *engUnit
	}
	if len(valBytes) > 0 {
		_ = json.Unmarshal(valBytes, &t.Value)
	}
	return &t, nil
}

func (s *PostgresService) ListTags(ctx context.Context, tenantID ids.ID) ([]*Tag, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, path, data_type, eng_unit, deadband, scale_factor, scale_offset,
		       eu_min, eu_max, scan_rate_ms, val_json, quality, timestamp, version
		FROM scada_tag
		WHERE tenant_id = $1 OR tenant_id IS NULL
		ORDER BY path
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*Tag, 0)
	for rows.Next() {
		var t Tag
		var valBytes []byte
		var engUnit *string
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.Path, &t.DataType, &engUnit, &t.Deadband, &t.ScaleFactor, &t.ScaleOffset,
			&t.EUMin, &t.EUMax, &t.ScanRateMs, &valBytes, &t.Quality, &t.Timestamp, &t.Version,
		); err != nil {
			return nil, err
		}
		if engUnit != nil {
			t.EngUnit = *engUnit
		}
		if len(valBytes) > 0 {
			_ = json.Unmarshal(valBytes, &t.Value)
		}
		result = append(result, &t)
	}
	return result, nil
}

func (s *PostgresService) UpdateTagValue(ctx context.Context, tenantID ids.ID, path string, rawValue any, quality string) (*Tag, bool, error) {
	tag, err := s.GetTagByPath(ctx, tenantID, path)
	if err != nil {
		return nil, false, err
	}

	// Apply scaling if analog
	scaledVal := rawValue
	if tag.DataType == DataTypeAnalog {
		if num, ok := toFloat64(rawValue); ok {
			scale := tag.ScaleFactor
			if scale == 0 {
				scale = 1.0
			}
			computed := num*scale + tag.ScaleOffset
			scaledVal = computed

			// Check Deadband
			if tag.Deadband > 0 && tag.Value != nil {
				if lastNum, ok := toFloat64(tag.Value); ok {
					diff := math.Abs(computed - lastNum)
					if diff < tag.Deadband && quality == tag.Quality {
						// Inside deadband: refresh timestamp only, suppress update propagation
						tag.Timestamp = time.Now().UnixMilli()
						_, _ = s.pool.Exec(ctx, `UPDATE scada_tag SET timestamp = $1 WHERE id = $2`, tag.Timestamp, tag.ID)
						return tag, false, nil
					}
				}
			}
		}
	}

	tag.LastValue = tag.Value
	tag.Value = scaledVal
	if quality != "" {
		tag.Quality = quality
	}
	tag.Timestamp = time.Now().UnixMilli()
	tag.Version++

	valBytes, _ := json.Marshal(tag.Value)
	_, err = s.pool.Exec(ctx, `
		UPDATE scada_tag
		SET val_json = $1, quality = $2, timestamp = $3, version = version + 1
		WHERE id = $4
	`, valBytes, tag.Quality, tag.Timestamp, tag.ID)
	if err != nil {
		return nil, false, err
	}

	// Check Process Alarm Thresholds (e.g. Tank Level > 90% HIGH_HIGH)
	if tag.DataType == DataTypeAnalog {
		if num, ok := toFloat64(scaledVal); ok && num > 90.0 {
			s.triggerISA182Alarm(ctx, tenantID, tag.Path, "HIGH_HIGH", num, 90.0, "Critical Tank High High Level Exceeded")
		}
	}

	return tag, true, nil
}

// ----------------------------------------------------------------------------
// 2. ISA-18.2 Process Alarms (Durable PostgreSQL)
// ----------------------------------------------------------------------------

func (s *PostgresService) triggerISA182Alarm(ctx context.Context, tenantID ids.ID, path, alarmType string, currentVal, setpoint float64, msg string) {
	now := time.Now().UnixMilli()

	// Check if alarm already exists for this tag & alarmType
	var existingID ids.ID
	var state string
	var shelveUntil *int64

	err := s.pool.QueryRow(ctx, `
		SELECT id, state, shelve_until
		FROM scada_alarm
		WHERE (tenant_id = $1 OR tenant_id IS NULL) AND tag_path = $2 AND alarm_type = $3
		LIMIT 1
	`, tenantID, path, alarmType).Scan(&existingID, &state, &shelveUntil)

	if err == nil {
		// Existing alarm
		if state == AlarmStateShelved && shelveUntil != nil && now < *shelveUntil {
			return // Still shelved
		}
		if state == AlarmStateNormal {
			_, _ = s.pool.Exec(ctx, `
				UPDATE scada_alarm
				SET state = 'UNACK_ALARM', active_ts = $1, message = $2, updated_at = NOW()
				WHERE id = $3
			`, now, fmt.Sprintf("%s (value: %.2f)", msg, currentVal), existingID)
		}
		return
	}

	// Create new ISA-18.2 alarm record
	alarmID := ids.MustNew()
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO scada_alarm (
			id, tenant_id, tag_path, alarm_type, setpoint, severity, state, active_ts, message
		) VALUES (
			$1, $2, $3, $4, $5, 'CRITICAL', 'UNACK_ALARM', $6, $7
		)
	`, alarmID, tenantID, path, alarmType, setpoint, now, fmt.Sprintf("%s (value: %.2f)", msg, currentVal))
}

func (s *PostgresService) ListAlarms(ctx context.Context, tenantID ids.ID) ([]*ISA182Alarm, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, tag_path, alarm_type, setpoint, severity, state, active_ts,
		       ack_ts, ack_user, shelve_until, message
		FROM scada_alarm
		WHERE tenant_id = $1 OR tenant_id IS NULL
		ORDER BY active_ts DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*ISA182Alarm, 0)
	for rows.Next() {
		var a ISA182Alarm
		var ackTs *int64
		var ackUser *string
		var shelveUntil *int64
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.TagPath, &a.AlarmType, &a.Setpoint, &a.Severity, &a.State, &a.ActiveTs,
			&ackTs, &ackUser, &shelveUntil, &a.Message,
		); err != nil {
			return nil, err
		}
		if ackTs != nil {
			a.AckTs = *ackTs
		}
		if ackUser != nil {
			a.AckUser = *ackUser
		}
		if shelveUntil != nil {
			a.ShelveUntil = *shelveUntil
		}
		result = append(result, &a)
	}
	return result, nil
}

func (s *PostgresService) AcknowledgeAlarm(ctx context.Context, tenantID, alarmID ids.ID, user string) (*ISA182Alarm, error) {
	now := time.Now().UnixMilli()
	var a ISA182Alarm
	var ackTs *int64
	var ackUser *string
	var shelveUntil *int64

	err := s.pool.QueryRow(ctx, `
		UPDATE scada_alarm
		SET state = 'ACKED_ALARM', ack_ts = $1, ack_user = $2, updated_at = NOW()
		WHERE id = $3 AND (tenant_id = $4 OR tenant_id IS NULL)
		RETURNING id, tenant_id, tag_path, alarm_type, setpoint, severity, state, active_ts,
		          ack_ts, ack_user, shelve_until, message
	`, now, user, alarmID, tenantID).Scan(
		&a.ID, &a.TenantID, &a.TagPath, &a.AlarmType, &a.Setpoint, &a.Severity, &a.State, &a.ActiveTs,
		&ackTs, &ackUser, &shelveUntil, &a.Message,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("alarm not found")
		}
		return nil, err
	}
	if ackTs != nil {
		a.AckTs = *ackTs
	}
	if ackUser != nil {
		a.AckUser = *ackUser
	}
	if shelveUntil != nil {
		a.ShelveUntil = *shelveUntil
	}
	return &a, nil
}

func (s *PostgresService) ShelveAlarm(ctx context.Context, tenantID, alarmID ids.ID, durationMs int64) (*ISA182Alarm, error) {
	if durationMs <= 0 {
		durationMs = 300000 // 5 minutes default
	}
	shelveUntilVal := time.Now().UnixMilli() + durationMs

	var a ISA182Alarm
	var ackTs *int64
	var ackUser *string
	var shelveUntil *int64

	err := s.pool.QueryRow(ctx, `
		UPDATE scada_alarm
		SET state = 'SHELVED', shelve_until = $1, updated_at = NOW()
		WHERE id = $2 AND (tenant_id = $3 OR tenant_id IS NULL)
		RETURNING id, tenant_id, tag_path, alarm_type, setpoint, severity, state, active_ts,
		          ack_ts, ack_user, shelve_until, message
	`, shelveUntilVal, alarmID, tenantID).Scan(
		&a.ID, &a.TenantID, &a.TagPath, &a.AlarmType, &a.Setpoint, &a.Severity, &a.State, &a.ActiveTs,
		&ackTs, &ackUser, &shelveUntil, &a.Message,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("alarm not found")
		}
		return nil, err
	}
	if ackTs != nil {
		a.AckTs = *ackTs
	}
	if ackUser != nil {
		a.AckUser = *ackUser
	}
	if shelveUntil != nil {
		a.ShelveUntil = *shelveUntil
	}
	return &a, nil
}

func (s *PostgresService) UnshelveAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*ISA182Alarm, error) {
	var a ISA182Alarm
	var ackTs *int64
	var ackUser *string
	var shelveUntil *int64

	err := s.pool.QueryRow(ctx, `
		UPDATE scada_alarm
		SET state = 'UNACK_ALARM', shelve_until = NULL, updated_at = NOW()
		WHERE id = $1 AND (tenant_id = $2 OR tenant_id IS NULL)
		RETURNING id, tenant_id, tag_path, alarm_type, setpoint, severity, state, active_ts,
		          ack_ts, ack_user, shelve_until, message
	`, alarmID, tenantID).Scan(
		&a.ID, &a.TenantID, &a.TagPath, &a.AlarmType, &a.Setpoint, &a.Severity, &a.State, &a.ActiveTs,
		&ackTs, &ackUser, &shelveUntil, &a.Message,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("alarm not found")
		}
		return nil, err
	}
	if ackTs != nil {
		a.AckTs = *ackTs
	}
	if ackUser != nil {
		a.AckUser = *ackUser
	}
	if shelveUntil != nil {
		a.ShelveUntil = *shelveUntil
	}
	return &a, nil
}

func (s *PostgresService) GetAlarmKPIs(ctx context.Context, tenantID ids.ID) (*AlarmKPIs, error) {
	alarms, err := s.ListAlarms(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	kpis := &AlarmKPIs{}
	now := time.Now().UnixMilli()
	tenMinAgo := now - 600000
	var totalAckTimeMs int64
	var ackCount int64

	for _, a := range alarms {
		kpis.TotalAlarms++
		switch a.State {
		case AlarmStateUnackAlarm:
			kpis.UnackAlarms++
		case AlarmStateAckedAlarm:
			kpis.AckedAlarms++
		case AlarmStateShelved:
			kpis.ShelvedAlarms++
		case AlarmStateRtnUnack:
			kpis.RtnUnackAlarms++
		case AlarmStateNormal:
			kpis.NormalAlarms++
		}

		if a.ActiveTs >= tenMinAgo {
			kpis.AlarmRate10Min++
		}
		if a.AckTs > a.ActiveTs {
			totalAckTimeMs += (a.AckTs - a.ActiveTs)
			ackCount++
		}
	}

	if ackCount > 0 {
		kpis.MTTASeconds = float64(totalAckTimeMs) / float64(ackCount) / 1000.0
	}
	kpis.InFlood = kpis.AlarmRate10Min > 10.0

	return kpis, nil
}

// ----------------------------------------------------------------------------
// 3. Select-Before-Operate (SBO) Safe Controls & Audit Log
// ----------------------------------------------------------------------------

func (s *PostgresService) SelectControl(ctx context.Context, tenantID ids.ID, path string, desiredValue any, operator string) (*ControlCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	selectID := ids.MustNew()

	cmd := &ControlCommand{
		SelectID:     selectID,
		TenantID:     tenantID,
		TagPath:      path,
		DesiredValue: desiredValue,
		Operator:     operator,
		SelectedAt:   now,
		Status:       "SELECTED",
	}

	s.controls[selectID] = cmd

	// Persist to audit log table
	valBytes, _ := json.Marshal(desiredValue)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO scada_control_log (
			id, tenant_id, tag_path, desired_value, operator, status, selected_at
		) VALUES (
			$1, $2, $3, $4, $5, 'SELECTED', $6
		)
	`, selectID, tenantID, path, valBytes, operator, now)

	return cmd, nil
}

func (s *PostgresService) OperateControl(ctx context.Context, tenantID ids.ID, selectID ids.ID) (*ControlCommand, error) {
	s.mu.Lock()
	cmd, ok := s.controls[selectID]
	s.mu.Unlock()

	if !ok || cmd.TenantID != tenantID {
		return nil, errs.NotFound("selected control not found or expired")
	}

	if cmd.Status != "SELECTED" {
		return nil, errs.InvalidArgument(fmt.Sprintf("command cannot be operated from status '%s'", cmd.Status))
	}

	now := time.Now().UnixMilli()
	if now-cmd.SelectedAt > 60000 {
		cmd.Status = "CANCELLED"
		cmd.Reason = "Select-Before-Operate window (60s) timed out"
		_, _ = s.pool.Exec(ctx, `
			UPDATE scada_control_log SET status = 'CANCELLED', reason = $1 WHERE id = $2
		`, cmd.Reason, selectID)
		return cmd, errs.InvalidArgument("control selection expired (timeout > 60s)")
	}

	// Apply value to tag
	_, _, err := s.UpdateTagValue(ctx, tenantID, cmd.TagPath, cmd.DesiredValue, QualityGood)
	if err != nil {
		cmd.Status = "CANCELLED"
		cmd.Reason = err.Error()
		_, _ = s.pool.Exec(ctx, `
			UPDATE scada_control_log SET status = 'CANCELLED', reason = $1 WHERE id = $2
		`, cmd.Reason, selectID)
		return cmd, err
	}

	cmd.Status = "OPERATED"
	cmd.OperatedAt = now

	_, _ = s.pool.Exec(ctx, `
		UPDATE scada_control_log SET status = 'OPERATED', operated_at = $1 WHERE id = $2
	`, now, selectID)

	return cmd, nil
}

func (s *PostgresService) ListControlLogs(ctx context.Context, tenantID ids.ID, limit int) ([]*ControlLogEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, tag_path, desired_value, operator, status, reason, selected_at, operated_at
		FROM scada_control_log
		WHERE tenant_id = $1 OR tenant_id IS NULL
		ORDER BY selected_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*ControlLogEntry, 0)
	for rows.Next() {
		var entry ControlLogEntry
		var valBytes []byte
		var reason *string
		var operatedAt *int64
		if err := rows.Scan(
			&entry.ID, &entry.TenantID, &entry.TagPath, &valBytes, &entry.Operator,
			&entry.Status, &reason, &entry.SelectedAt, &operatedAt,
		); err != nil {
			return nil, err
		}
		if reason != nil {
			entry.Reason = *reason
		}
		if operatedAt != nil {
			entry.OperatedAt = operatedAt
		}
		if len(valBytes) > 0 {
			_ = json.Unmarshal(valBytes, &entry.DesiredValue)
		}
		result = append(result, &entry)
	}
	return result, nil
}

// ----------------------------------------------------------------------------
// 4. ThingsBoard-Compatible SVG Symbols Catalog (Durable PostgreSQL)
// ----------------------------------------------------------------------------

func (s *PostgresService) ListSymbols(ctx context.Context, tenantID ids.ID, category string) ([]*Symbol, error) {
	query := `
		SELECT id, tenant_id, name, category, style, svg_content, tags, properties, behaviors, animations, version,
		       (EXTRACT(EPOCH FROM created_at) * 1000)::BIGINT
		FROM scada_symbol
		WHERE (tenant_id = $1 OR tenant_id IS NULL)
	`
	args := []any{tenantID}

	if category != "" {
		query += " AND category = $2 ORDER BY name ASC"
		args = append(args, category)
	} else {
		query += " ORDER BY category ASC, name ASC"
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*Symbol, 0)
	for rows.Next() {
		var sym Symbol
		var tID *ids.ID
		var tagsBytes, propsBytes, behavBytes, animBytes []byte
		if err := rows.Scan(
			&sym.ID, &tID, &sym.Name, &sym.Category, &sym.Style, &sym.SVGContent,
			&tagsBytes, &propsBytes, &behavBytes, &animBytes, &sym.Version, &sym.CreatedAt,
		); err != nil {
			return nil, err
		}
		sym.TenantID = tID
		_ = json.Unmarshal(tagsBytes, &sym.Tags)
		_ = json.Unmarshal(propsBytes, &sym.Properties)
		_ = json.Unmarshal(behavBytes, &sym.Behaviors)
		_ = json.Unmarshal(animBytes, &sym.Animations)
		result = append(result, &sym)
	}
	return result, nil
}

func (s *PostgresService) GetSymbol(ctx context.Context, tenantID ids.ID, symbolID ids.ID) (*Symbol, error) {
	var sym Symbol
	var tID *ids.ID
	var tagsBytes, propsBytes, behavBytes, animBytes []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, category, style, svg_content, tags, properties, behaviors, animations, version,
		       (EXTRACT(EPOCH FROM created_at) * 1000)::BIGINT
		FROM scada_symbol
		WHERE id = $1 AND (tenant_id = $2 OR tenant_id IS NULL)
	`, symbolID, tenantID).Scan(
		&sym.ID, &tID, &sym.Name, &sym.Category, &sym.Style, &sym.SVGContent,
		&tagsBytes, &propsBytes, &behavBytes, &animBytes, &sym.Version, &sym.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("symbol not found")
		}
		return nil, err
	}
	sym.TenantID = tID
	_ = json.Unmarshal(tagsBytes, &sym.Tags)
	_ = json.Unmarshal(propsBytes, &sym.Properties)
	_ = json.Unmarshal(behavBytes, &sym.Behaviors)
	_ = json.Unmarshal(animBytes, &sym.Animations)
	return &sym, nil
}

func (s *PostgresService) SaveSymbol(ctx context.Context, symbol *Symbol) error {
	if symbol.ID == ids.Nil {
		symbol.ID = ids.MustNew()
	}
	if symbol.CreatedAt == 0 {
		symbol.CreatedAt = time.Now().UnixMilli()
	}

	tagsJSON, _ := json.Marshal(symbol.Tags)
	propsJSON, _ := json.Marshal(symbol.Properties)
	behaviorsJSON, _ := json.Marshal(symbol.Behaviors)
	animationsJSON, _ := json.Marshal(symbol.Animations)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO scada_symbol (
			id, tenant_id, name, category, style, svg_content,
			tags, properties, behaviors, animations, version
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			category = EXCLUDED.category,
			style = EXCLUDED.style,
			svg_content = EXCLUDED.svg_content,
			tags = EXCLUDED.tags,
			properties = EXCLUDED.properties,
			behaviors = EXCLUDED.behaviors,
			animations = EXCLUDED.animations,
			version = scada_symbol.version + 1
	`, symbol.ID, symbol.TenantID, symbol.Name, symbol.Category, symbol.Style, symbol.SVGContent,
		tagsJSON, propsJSON, behaviorsJSON, animationsJSON, symbol.Version)
	return err
}

func (s *PostgresService) DeleteSymbol(ctx context.Context, tenantID ids.ID, symbolID ids.ID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM scada_symbol
		WHERE id = $1 AND (tenant_id = $2 OR tenant_id IS NULL)
	`, symbolID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("symbol not found")
	}
	return nil
}
