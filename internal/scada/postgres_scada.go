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

// PostgresService implements industrial SCADA tag processing with PostgreSQL persistence.
type PostgresService struct {
	pool     *pgxpool.Pool
	mu       sync.RWMutex
	alarms   map[ids.ID]*ISA182Alarm
	controls map[ids.ID]*ControlCommand
}

// NewPostgresService creates a PostgreSQL-backed SCADA service.
func NewPostgresService(pool *pgxpool.Pool) Service {
	return &PostgresService{
		pool:     pool,
		alarms:   make(map[ids.ID]*ISA182Alarm),
		controls: make(map[ids.ID]*ControlCommand),
	}
}

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
		WHERE tenant_id = $1 AND path = $2
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
		WHERE tenant_id = $1
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

	// Check Alarm Thresholds (e.g. Tank Level > 90% HIGH_HIGH)
	if tag.DataType == DataTypeAnalog {
		if num, ok := toFloat64(scaledVal); ok && num > 90.0 {
			s.triggerISA182Alarm(tenantID, tag.Path, "HIGH_HIGH", num, 90.0, "Critical Tank High High Level Exceeded")
		}
	}

	return tag, true, nil
}

func (s *PostgresService) triggerISA182Alarm(tenantID ids.ID, path, alarmType string, currentVal, setpoint float64, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range s.alarms {
		if a.TagPath == path && a.AlarmType == alarmType {
			if a.State == AlarmStateShelved && time.Now().UnixMilli() < a.ShelveUntil {
				return
			}
			if a.State == AlarmStateNormal {
				a.State = AlarmStateUnackAlarm
				a.ActiveTs = time.Now().UnixMilli()
			}
			return
		}
	}

	alarmID := ids.MustNew()
	s.alarms[alarmID] = &ISA182Alarm{
		ID:        alarmID,
		TenantID:  tenantID,
		TagPath:   path,
		AlarmType: alarmType,
		Setpoint:  setpoint,
		Severity:  "CRITICAL",
		State:     AlarmStateUnackAlarm,
		ActiveTs:  time.Now().UnixMilli(),
		Message:   fmt.Sprintf("%s (value: %.2f)", msg, currentVal),
	}
}

func (s *PostgresService) ListAlarms(ctx context.Context, tenantID ids.ID) ([]*ISA182Alarm, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ISA182Alarm, 0)
	for _, a := range s.alarms {
		if a.TenantID == ids.Nil || a.TenantID == tenantID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (s *PostgresService) AcknowledgeAlarm(ctx context.Context, tenantID, alarmID ids.ID, user string) (*ISA182Alarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.alarms[alarmID]
	if !ok || (a.TenantID != ids.Nil && a.TenantID != tenantID) {
		return nil, errs.NotFound("alarm not found")
	}

	if a.State == AlarmStateUnackAlarm {
		a.State = AlarmStateAckedAlarm
		a.AckTs = time.Now().UnixMilli()
		a.AckUser = user
	}
	return a, nil
}

func (s *PostgresService) ShelveAlarm(ctx context.Context, tenantID, alarmID ids.ID, durationMs int64) (*ISA182Alarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.alarms[alarmID]
	if !ok || (a.TenantID != ids.Nil && a.TenantID != tenantID) {
		return nil, errs.NotFound("alarm not found")
	}

	a.State = AlarmStateShelved
	a.ShelveUntil = time.Now().UnixMilli() + durationMs
	return a, nil
}

func (s *PostgresService) SelectControl(ctx context.Context, tenantID ids.ID, path string, desiredValue any, operator string) (*ControlCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := &ControlCommand{
		SelectID:     ids.MustNew(),
		TenantID:     tenantID,
		TagPath:      path,
		DesiredValue: desiredValue,
		Operator:     operator,
		SelectedAt:   time.Now().UnixMilli(),
		Status:       "SELECTED",
	}

	s.controls[cmd.SelectID] = cmd
	return cmd, nil
}

func (s *PostgresService) OperateControl(ctx context.Context, tenantID ids.ID, selectID ids.ID) (*ControlCommand, error) {
	s.mu.Lock()
	cmd, ok := s.controls[selectID]
	s.mu.Unlock()

	if !ok || cmd.TenantID != tenantID {
		return nil, errs.NotFound("selected control not found or expired")
	}

	if time.Now().UnixMilli()-cmd.SelectedAt > 60000 {
		cmd.Status = "CANCELLED"
		cmd.Reason = "Select-Before-Operate window (60s) timed out"
		return cmd, errs.InvalidArgument("control selection expired (timeout > 60s)")
	}

	// Apply value to tag
	_, _, err := s.UpdateTagValue(ctx, tenantID, cmd.TagPath, cmd.DesiredValue, QualityGood)
	if err != nil {
		cmd.Status = "CANCELLED"
		cmd.Reason = err.Error()
		return cmd, err
	}

	cmd.Status = "OPERATED"
	cmd.OperatedAt = time.Now().UnixMilli()
	return cmd, nil
}
