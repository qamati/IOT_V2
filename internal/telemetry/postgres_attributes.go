package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

func scopeToInt(scope model.AttributeScope) smallintScope {
	switch scope {
	case model.ScopeClient:
		return 1
	case model.ScopeServer:
		return 2
	case model.ScopeShared:
		return 3
	default:
		return 1
	}
}

func intToScope(i int16) model.AttributeScope {
	switch i {
	case 1:
		return model.ScopeClient
	case 2:
		return model.ScopeServer
	case 3:
		return model.ScopeShared
	default:
		return model.ScopeClient
	}
}

type smallintScope = int16

// PostgresAttributeStore persists scoped attributes into the attribute_kv table.
type PostgresAttributeStore struct {
	pool    *pgxpool.Pool
	mu      sync.RWMutex
	keyDict map[string]int // key -> key_id cache
}

// NewPostgresAttributeStore creates a Postgres-backed AttributeStore.
func NewPostgresAttributeStore(pool *pgxpool.Pool) AttributeStore {
	return &PostgresAttributeStore{
		pool:    pool,
		keyDict: make(map[string]int),
	}
}

func (p *PostgresAttributeStore) getOrCreateKeyID(ctx context.Context, key string) (int, error) {
	p.mu.RLock()
	id, ok := p.keyDict[key]
	p.mu.RUnlock()
	if ok {
		return id, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

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

func (p *PostgresAttributeStore) SaveAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, attrs map[string]any) error {
	if len(attrs) == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	scopeCode := scopeToInt(scope)

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for k, v := range attrs {
		keyID, err := p.getOrCreateKeyID(ctx, k)
		if err != nil {
			return fmt.Errorf("attribute key_id lookup failed: %w", err)
		}

		var boolV *bool
		var strV *string
		var longV *int64
		var dblV *float64
		var jsonV []byte

		switch val := v.(type) {
		case bool:
			boolV = &val
		case float64:
			if val == float64(int64(val)) {
				intVal := int64(val)
				longV = &intVal
			}
			dblV = &val
		case int64:
			longV = &val
		case int:
			intVal := int64(val)
			longV = &intVal
		case string:
			strV = &val
		default:
			jsonV, _ = json.Marshal(val)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO attribute_kv (entity_id, scope, key_id, bool_v, str_v, long_v, dbl_v, json_v, last_update_ts, version)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)
			ON CONFLICT (entity_id, scope, key_id)
			DO UPDATE SET
				bool_v = EXCLUDED.bool_v,
				str_v = EXCLUDED.str_v,
				long_v = EXCLUDED.long_v,
				dbl_v = EXCLUDED.dbl_v,
				json_v = EXCLUDED.json_v,
				last_update_ts = EXCLUDED.last_update_ts,
				version = attribute_kv.version + 1
		`, entityID, scopeCode, keyID, boolV, strV, longV, dblV, jsonV, now)
		if err != nil {
			return fmt.Errorf("failed to upsert attribute %s: %w", k, err)
		}
	}

	return tx.Commit(ctx)
}

func (p *PostgresAttributeStore) GetAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, keys []string) ([]model.AttributeKv, error) {
	scopeCode := scopeToInt(scope)

	query := `
		SELECT k.key, a.scope, a.last_update_ts, a.bool_v, a.str_v, a.long_v, a.dbl_v, a.json_v
		FROM attribute_kv a
		JOIN key_dictionary k ON a.key_id = k.key_id
		WHERE a.entity_id = $1 AND a.scope = $2
	`
	var rows pgx.Rows
	var err error

	if len(keys) > 0 {
		query += ` AND k.key = ANY($3)`
		rows, err = p.pool.Query(ctx, query, entityID, scopeCode, keys)
	} else {
		rows, err = p.pool.Query(ctx, query, entityID, scopeCode)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.AttributeKv
	for rows.Next() {
		var (
			keyName      string
			sc           int16
			lastUpdate   int64
			boolV        *bool
			strV         *string
			longV        *int64
			dblV         *float64
			jsonV        []byte
		)
		if err := rows.Scan(&keyName, &sc, &lastUpdate, &boolV, &strV, &longV, &dblV, &jsonV); err != nil {
			return nil, err
		}

		var actualVal any
		if boolV != nil {
			actualVal = *boolV
		} else if strV != nil {
			actualVal = *strV
		} else if longV != nil {
			actualVal = *longV
		} else if dblV != nil {
			actualVal = *dblV
		} else if len(jsonV) > 0 {
			var decoded any
			_ = json.Unmarshal(jsonV, &decoded)
			actualVal = decoded
		}

		result = append(result, model.AttributeKv{
			EntityID:   entityID,
			Scope:      intToScope(sc),
			Key:        keyName,
			LastUpdate: lastUpdate,
			Val:        actualVal,
		})
	}
	if result == nil {
		result = make([]model.AttributeKv, 0)
	}
	return result, nil
}

func (p *PostgresAttributeStore) GetAllAttributes(ctx context.Context, entityID ids.ID) ([]model.AttributeKv, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT k.key, a.scope, a.last_update_ts, a.bool_v, a.str_v, a.long_v, a.dbl_v, a.json_v
		FROM attribute_kv a
		JOIN key_dictionary k ON a.key_id = k.key_id
		WHERE a.entity_id = $1
		ORDER BY a.scope, k.key
	`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.AttributeKv
	for rows.Next() {
		var (
			keyName    string
			sc         int16
			lastUpdate int64
			boolV      *bool
			strV       *string
			longV      *int64
			dblV       *float64
			jsonV      []byte
		)
		if err := rows.Scan(&keyName, &sc, &lastUpdate, &boolV, &strV, &longV, &dblV, &jsonV); err != nil {
			return nil, err
		}

		var actualVal any
		if boolV != nil {
			actualVal = *boolV
		} else if strV != nil {
			actualVal = *strV
		} else if longV != nil {
			actualVal = *longV
		} else if dblV != nil {
			actualVal = *dblV
		} else if len(jsonV) > 0 {
			var decoded any
			_ = json.Unmarshal(jsonV, &decoded)
			actualVal = decoded
		}

		result = append(result, model.AttributeKv{
			EntityID:   entityID,
			Scope:      intToScope(sc),
			Key:        keyName,
			LastUpdate: lastUpdate,
			Val:        actualVal,
		})
	}
	if result == nil {
		result = make([]model.AttributeKv, 0)
	}
	return result, nil
}

func (p *PostgresAttributeStore) DeleteAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	scopeCode := scopeToInt(scope)

	_, err := p.pool.Exec(ctx, `
		DELETE FROM attribute_kv
		WHERE entity_id = $1 AND scope = $2 AND key_id IN (
			SELECT key_id FROM key_dictionary WHERE key = ANY($3)
		)
	`, entityID, scopeCode, keys)
	return err
}
