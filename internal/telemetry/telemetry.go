package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// TelemetryDataPoint is an alias for model.TelemetryDataPoint.
type TelemetryDataPoint = model.TelemetryDataPoint

// ParsePayload parses ThingsBoard-compatible telemetry JSON payloads into slice of data points.
func ParsePayload(payload []byte, defaultTs int64) ([]*model.TelemetryDataPoint, error) {
	if defaultTs == 0 {
		defaultTs = time.Now().UnixMilli()
	}

	// Try batch array first: [{"ts": 123, "values": {"k": "v"}}]
	var batch []struct {
		Ts     int64          `json:"ts"`
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(payload, &batch); err == nil && len(batch) > 0 {
		var points []*model.TelemetryDataPoint
		for _, item := range batch {
			ts := item.Ts
			if ts == 0 {
				ts = defaultTs
			}
			for k, v := range item.Values {
				points = append(points, createPoint(k, ts, v))
			}
		}
		return points, nil
	}

	// Try object with "ts" and "values": {"ts": 123, "values": {"k": "v"}}
	var singleWithTs struct {
		Ts     int64          `json:"ts"`
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(payload, &singleWithTs); err == nil && singleWithTs.Values != nil {
		ts := singleWithTs.Ts
		if ts == 0 {
			ts = defaultTs
		}
		var points []*model.TelemetryDataPoint
		for k, v := range singleWithTs.Values {
			points = append(points, createPoint(k, ts, v))
		}
		return points, nil
	}

	// Plain key-value map: {"temperature": 22.5, "humidity": 40}
	var rawMap map[string]any
	if err := json.Unmarshal(payload, &rawMap); err == nil {
		var points []*model.TelemetryDataPoint
		for k, v := range rawMap {
			points = append(points, createPoint(k, defaultTs, v))
		}
		return points, nil
	}

	return nil, fmt.Errorf("telemetry: unrecognized JSON payload format")
}

func createPoint(key string, ts int64, val any) *model.TelemetryDataPoint {
	p := &model.TelemetryDataPoint{
		Key:       key,
		Timestamp: ts,
	}

	switch v := val.(type) {
	case bool:
		p.BoolV = &v
	case float64:
		// Check if integer or double
		if v == float64(int64(v)) {
			intVal := int64(v)
			p.LongV = &intVal
		}
		p.DblV = &v
	case string:
		p.StrV = &v
	default:
		b, _ := json.Marshal(v)
		p.JsonV = b
	}
	return p
}

// Store defines the interface for persisting and querying telemetry.
type Store interface {
	Save(ctx context.Context, entityID ids.ID, points []*model.TelemetryDataPoint) error
	GetLatest(ctx context.Context, entityID ids.ID, keys []string) (map[string]*model.TelemetryDataPoint, error)
	GetTimeseries(ctx context.Context, entityID ids.ID, keys []string, startTs, endTs int64, limit int) (map[string][]*model.TelemetryDataPoint, error)
}

// MemoryStore provides in-memory time-series storage.
type MemoryStore struct {
	mu     sync.RWMutex
	series map[ids.ID][]*model.TelemetryDataPoint
	latest map[ids.ID]map[string]*model.TelemetryDataPoint
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		series: make(map[ids.ID][]*model.TelemetryDataPoint),
		latest: make(map[ids.ID]map[string]*model.TelemetryDataPoint),
	}
}

func (m *MemoryStore) Save(ctx context.Context, entityID ids.ID, points []*model.TelemetryDataPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.latest[entityID]; !ok {
		m.latest[entityID] = make(map[string]*model.TelemetryDataPoint)
	}

	for _, pt := range points {
		m.series[entityID] = append(m.series[entityID], pt)
		existing, ok := m.latest[entityID][pt.Key]
		if !ok || pt.Timestamp >= existing.Timestamp {
			m.latest[entityID][pt.Key] = pt
		}
	}
	return nil
}

func (m *MemoryStore) GetLatest(ctx context.Context, entityID ids.ID, keys []string) (map[string]*model.TelemetryDataPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*model.TelemetryDataPoint)
	entityLatest, ok := m.latest[entityID]
	if !ok {
		return result, nil
	}

	if len(keys) == 0 {
		for k, v := range entityLatest {
			result[k] = v
		}
		return result, nil
	}

	for _, k := range keys {
		if v, exists := entityLatest[k]; exists {
			result[k] = v
		}
	}
	return result, nil
}

func (m *MemoryStore) GetTimeseries(ctx context.Context, entityID ids.ID, keys []string, startTs, endTs int64, limit int) (map[string][]*model.TelemetryDataPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*model.TelemetryDataPoint)
	pts, ok := m.series[entityID]
	if !ok {
		return result, nil
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	if limit <= 0 {
		limit = 1000
	}

	for _, p := range pts {
		if len(keys) > 0 && !keySet[p.Key] {
			continue
		}
		if startTs > 0 && p.Timestamp < startTs {
			continue
		}
		if endTs > 0 && p.Timestamp > endTs {
			continue
		}
		if len(result[p.Key]) < limit {
			result[p.Key] = append(result[p.Key], p)
		}
	}
	return result, nil
}

// PostgresStore persists telemetry into ts_kv and ts_kv_latest with key_dictionary lookups.
type PostgresStore struct {
	pool    *pgxpool.Pool
	mu      sync.RWMutex
	keyDict map[string]int // key -> key_id cache
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{
		pool:    pool,
		keyDict: make(map[string]int),
	}
}

func (p *PostgresStore) getOrCreateKeyID(ctx context.Context, key string) (int, error) {
	p.mu.RLock()
	id, ok := p.keyDict[key]
	p.mu.RUnlock()
	if ok {
		return id, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check
	if id, ok := p.keyDict[key]; ok {
		return id, nil
	}

	var keyID int
	err := p.pool.QueryRow(ctx, `
		INSERT INTO key_dictionary (key) VALUES ($1)
		ON CONFLICT (key) DO UPDATE SET key = EXCLUDED.key
		RETURNING key_id
	`, key).Scan(&keyID)
	if err != nil {
		return 0, err
	}

	p.keyDict[key] = keyID
	return keyID, nil
}

func (p *PostgresStore) Save(ctx context.Context, entityID ids.ID, points []*model.TelemetryDataPoint) error {
	if len(points) == 0 {
		return nil
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, pt := range points {
		keyID, err := p.getOrCreateKeyID(ctx, pt.Key)
		if err != nil {
			return err
		}

		// Insert into ts_kv
		_, err = tx.Exec(ctx, `
			INSERT INTO ts_kv (entity_id, key_id, ts, bool_v, str_v, long_v, dbl_v, json_v)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (entity_id, key_id, ts) DO UPDATE SET
				bool_v = EXCLUDED.bool_v,
				str_v = EXCLUDED.str_v,
				long_v = EXCLUDED.long_v,
				dbl_v = EXCLUDED.dbl_v,
				json_v = EXCLUDED.json_v
		`, entityID, keyID, pt.Timestamp, pt.BoolV, pt.StrV, pt.LongV, pt.DblV, pt.JsonV)
		if err != nil {
			return err
		}

		// Upsert into ts_kv_latest
		_, err = tx.Exec(ctx, `
			INSERT INTO ts_kv_latest (entity_id, key_id, ts, bool_v, str_v, long_v, dbl_v, json_v)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (entity_id, key_id) DO UPDATE SET
				ts = EXCLUDED.ts,
				bool_v = EXCLUDED.bool_v,
				str_v = EXCLUDED.str_v,
				long_v = EXCLUDED.long_v,
				dbl_v = EXCLUDED.dbl_v,
				json_v = EXCLUDED.json_v
			WHERE EXCLUDED.ts >= ts_kv_latest.ts
		`, entityID, keyID, pt.Timestamp, pt.BoolV, pt.StrV, pt.LongV, pt.DblV, pt.JsonV)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (p *PostgresStore) GetLatest(ctx context.Context, entityID ids.ID, keys []string) (map[string]*model.TelemetryDataPoint, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT k.key, l.ts, l.bool_v, l.str_v, l.long_v, l.dbl_v, l.json_v
		FROM ts_kv_latest l
		JOIN key_dictionary k ON k.key_id = l.key_id
		WHERE l.entity_id = $1
	`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	result := make(map[string]*model.TelemetryDataPoint)
	for rows.Next() {
		var pt model.TelemetryDataPoint
		if err := rows.Scan(&pt.Key, &pt.Timestamp, &pt.BoolV, &pt.StrV, &pt.LongV, &pt.DblV, &pt.JsonV); err != nil {
			return nil, err
		}
		if len(keys) == 0 || keySet[pt.Key] {
			result[pt.Key] = &pt
		}
	}
	return result, nil
}

func (p *PostgresStore) GetTimeseries(ctx context.Context, entityID ids.ID, keys []string, startTs, endTs int64, limit int) (map[string][]*model.TelemetryDataPoint, error) {
	if limit <= 0 {
		limit = 1000
	}
	if endTs == 0 {
		endTs = time.Now().UnixMilli()
	}

	rows, err := p.pool.Query(ctx, `
		SELECT k.key, t.ts, t.bool_v, t.str_v, t.long_v, t.dbl_v, t.json_v
		FROM ts_kv t
		JOIN key_dictionary k ON k.key_id = t.key_id
		WHERE t.entity_id = $1 AND t.ts >= $2 AND t.ts <= $3
		ORDER BY t.ts ASC
		LIMIT $4
	`, entityID, startTs, endTs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	result := make(map[string][]*model.TelemetryDataPoint)
	for rows.Next() {
		var pt model.TelemetryDataPoint
		if err := rows.Scan(&pt.Key, &pt.Timestamp, &pt.BoolV, &pt.StrV, &pt.LongV, &pt.DblV, &pt.JsonV); err != nil {
			return nil, err
		}
		if len(keys) == 0 || keySet[pt.Key] {
			result[pt.Key] = append(result[pt.Key], &pt)
		}
	}
	return result, nil
}
