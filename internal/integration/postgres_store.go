package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL integration store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) withTenant(ctx context.Context, tenantID ids.ID) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("integration: failed to begin tx: %w", err)
	}

	if tenantID != ids.Nil {
		_, err = tx.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID.String())
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("integration: failed to set tenant RLS context: %w", err)
		}
	}
	return tx, nil
}

// ==========================================
// Converters
// ==========================================

func (s *PostgresStore) CreateConverter(ctx context.Context, c *Converter) error {
	if c.ID == ids.Nil {
		c.ID = ids.MustNew()
	}
	if c.Version == 0 {
		c.Version = 1
	}

	tx, err := s.withTenant(ctx, c.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	defJSON := c.Definition
	if len(defJSON) == 0 {
		defJSON = []byte("{}")
	}

	testsJSON, err := json.Marshal(c.Tests)
	if err != nil || len(testsJSON) == 0 {
		testsJSON = []byte("[]")
	}

	var tenantIDParam any
	if c.TenantID != ids.Nil {
		tenantIDParam = c.TenantID
	}

	query := `
		INSERT INTO converter (
			id, tenant_id, name, direction, mode, definition, tests, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
		)
	`
	_, err = tx.Exec(ctx, query,
		c.ID, tenantIDParam, c.Name, c.Direction, c.Mode, defJSON, testsJSON, c.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to insert converter: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetConverter(ctx context.Context, tenantID, id ids.ID) (*Converter, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::UUID), name, direction, mode, definition, tests, version
		FROM converter
		WHERE id = $1 AND (tenant_id = $2 OR tenant_id IS NULL OR $2 = '00000000-0000-0000-0000-000000000000'::UUID)
	`
	row := tx.QueryRow(ctx, query, id, tenantID)

	var c Converter
	var tID ids.ID
	var testsBytes []byte
	err = row.Scan(&c.ID, &tID, &c.Name, &c.Direction, &c.Mode, &c.Definition, &testsBytes, &c.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("converter not found")
		}
		return nil, fmt.Errorf("failed to get converter: %w", err)
	}
	c.TenantID = tID
	if len(testsBytes) > 0 {
		_ = json.Unmarshal(testsBytes, &c.Tests)
	}

	_ = tx.Commit(ctx)
	return &c, nil
}

func (s *PostgresStore) ListConverters(ctx context.Context, tenantID ids.ID) ([]*Converter, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::UUID), name, direction, mode, definition, tests, version
		FROM converter
		WHERE (tenant_id = $1 OR tenant_id IS NULL OR $1 = '00000000-0000-0000-0000-000000000000'::UUID)
		ORDER BY created_at ASC
	`
	rows, err := tx.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list converters: %w", err)
	}
	defer rows.Close()

	var result []*Converter
	for rows.Next() {
		var c Converter
		var tID ids.ID
		var testsBytes []byte
		if err := rows.Scan(&c.ID, &tID, &c.Name, &c.Direction, &c.Mode, &c.Definition, &testsBytes, &c.Version); err != nil {
			return nil, fmt.Errorf("failed to scan converter: %w", err)
		}
		c.TenantID = tID
		if len(testsBytes) > 0 {
			_ = json.Unmarshal(testsBytes, &c.Tests)
		}
		result = append(result, &c)
	}

	_ = tx.Commit(ctx)
	return result, nil
}

func (s *PostgresStore) UpdateConverter(ctx context.Context, c *Converter) error {
	tx, err := s.withTenant(ctx, c.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	defJSON := c.Definition
	if len(defJSON) == 0 {
		defJSON = []byte("{}")
	}

	testsJSON, err := json.Marshal(c.Tests)
	if err != nil || len(testsJSON) == 0 {
		testsJSON = []byte("[]")
	}

	query := `
		UPDATE converter
		SET name = $1, direction = $2, mode = $3, definition = $4, tests = $5, version = version + 1, updated_at = NOW()
		WHERE id = $6 AND (tenant_id = $7 OR tenant_id IS NULL OR $7 = '00000000-0000-0000-0000-000000000000'::UUID)
	`
	cmdTag, err := tx.Exec(ctx, query, c.Name, c.Direction, c.Mode, defJSON, testsJSON, c.ID, c.TenantID)
	if err != nil {
		return fmt.Errorf("failed to update converter: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return errs.NotFound("converter not found")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) DeleteConverter(ctx context.Context, tenantID, id ids.ID) error {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		DELETE FROM converter
		WHERE id = $1 AND (tenant_id = $2 OR tenant_id IS NULL OR $2 = '00000000-0000-0000-0000-000000000000'::UUID)
	`
	cmdTag, err := tx.Exec(ctx, query, id, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete converter: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return errs.NotFound("converter not found")
	}

	return tx.Commit(ctx)
}

// ==========================================
// Integrations
// ==========================================

func (s *PostgresStore) CreateIntegration(ctx context.Context, i *Integration) error {
	if i.ID == ids.Nil {
		i.ID = ids.MustNew()
	}
	if i.Version == 0 {
		i.Version = 1
	}

	tx, err := s.withTenant(ctx, i.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	cfgJSON := i.Config
	if len(cfgJSON) == 0 {
		cfgJSON = []byte("{}")
	}

	secJSON := i.SecretsRef
	if len(secJSON) == 0 {
		secJSON = []byte("{}")
	}

	var upID, downID any
	if i.UplinkConverterID != ids.Nil {
		upID = i.UplinkConverterID
	}
	if i.DownlinkConverterID != ids.Nil {
		downID = i.DownlinkConverterID
	}

	query := `
		INSERT INTO integration (
			id, tenant_id, name, type, enabled, uplink_converter_id, downlink_converter_id,
			routing_key, config, secrets_ref, allow_create_device, default_device_type,
			remote, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, NOW(), NOW()
		)
	`
	_, err = tx.Exec(ctx, query,
		i.ID, i.TenantID, i.Name, i.Type, i.Enabled, upID, downID,
		i.RoutingKey, cfgJSON, secJSON, i.AllowCreateDevice, i.DefaultDeviceType,
		i.Remote, i.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to insert integration: %w", err)
	}

	// Initialize status
	initStatusQuery := `
		INSERT INTO integration_status (integration_id, state, last_error, last_ok_ts, counters, instance)
		VALUES ($1, 'RUNNING', '', $2, '{"rx_events":0,"tx_events":0,"errors":0,"auto_created":0}', 'standalone')
		ON CONFLICT (integration_id) DO NOTHING
	`
	_, _ = tx.Exec(ctx, initStatusQuery, i.ID, time.Now().UnixMilli())

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetIntegration(ctx context.Context, tenantID, id ids.ID) (*Integration, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, tenant_id, name, type, enabled,
		       COALESCE(uplink_converter_id, '00000000-0000-0000-0000-000000000000'::UUID),
		       COALESCE(downlink_converter_id, '00000000-0000-0000-0000-000000000000'::UUID),
		       routing_key, config, secrets_ref, allow_create_device, default_device_type,
		       remote, version
		FROM integration
		WHERE id = $1 AND (tenant_id = $2 OR $2 = '00000000-0000-0000-0000-000000000000'::UUID)
	`
	row := tx.QueryRow(ctx, query, id, tenantID)

	var i Integration
	var upID, downID ids.ID
	err = row.Scan(
		&i.ID, &i.TenantID, &i.Name, &i.Type, &i.Enabled,
		&upID, &downID,
		&i.RoutingKey, &i.Config, &i.SecretsRef, &i.AllowCreateDevice, &i.DefaultDeviceType,
		&i.Remote, &i.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("integration not found")
		}
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}
	i.UplinkConverterID = upID
	i.DownlinkConverterID = downID

	_ = tx.Commit(ctx)
	return &i, nil
}

func (s *PostgresStore) GetIntegrationByRoutingKey(ctx context.Context, routingKey string) (*Integration, error) {
	query := `
		SELECT id, tenant_id, name, type, enabled,
		       COALESCE(uplink_converter_id, '00000000-0000-0000-0000-000000000000'::UUID),
		       COALESCE(downlink_converter_id, '00000000-0000-0000-0000-000000000000'::UUID),
		       routing_key, config, secrets_ref, allow_create_device, default_device_type,
		       remote, version
		FROM integration
		WHERE routing_key = $1
	`
	row := s.pool.QueryRow(ctx, query, routingKey)

	var i Integration
	var upID, downID ids.ID
	err := row.Scan(
		&i.ID, &i.TenantID, &i.Name, &i.Type, &i.Enabled,
		&upID, &downID,
		&i.RoutingKey, &i.Config, &i.SecretsRef, &i.AllowCreateDevice, &i.DefaultDeviceType,
		&i.Remote, &i.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("integration not found for routing key")
		}
		return nil, fmt.Errorf("failed to get integration by routing key: %w", err)
	}
	i.UplinkConverterID = upID
	i.DownlinkConverterID = downID
	return &i, nil
}

func (s *PostgresStore) ListIntegrations(ctx context.Context, tenantID ids.ID) ([]*Integration, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, tenant_id, name, type, enabled,
		       COALESCE(uplink_converter_id, '00000000-0000-0000-0000-000000000000'::UUID),
		       COALESCE(downlink_converter_id, '00000000-0000-0000-0000-000000000000'::UUID),
		       routing_key, config, secrets_ref, allow_create_device, default_device_type,
		       remote, version
		FROM integration
		WHERE (tenant_id = $1 OR $1 = '00000000-0000-0000-0000-000000000000'::UUID)
		ORDER BY created_at ASC
	`
	rows, err := tx.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list integrations: %w", err)
	}
	defer rows.Close()

	var result []*Integration
	for rows.Next() {
		var i Integration
		var upID, downID ids.ID
		if err := rows.Scan(
			&i.ID, &i.TenantID, &i.Name, &i.Type, &i.Enabled,
			&upID, &downID,
			&i.RoutingKey, &i.Config, &i.SecretsRef, &i.AllowCreateDevice, &i.DefaultDeviceType,
			&i.Remote, &i.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan integration: %w", err)
		}
		i.UplinkConverterID = upID
		i.DownlinkConverterID = downID
		result = append(result, &i)
	}

	_ = tx.Commit(ctx)
	return result, nil
}

func (s *PostgresStore) UpdateIntegration(ctx context.Context, i *Integration) error {
	tx, err := s.withTenant(ctx, i.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	cfgJSON := i.Config
	if len(cfgJSON) == 0 {
		cfgJSON = []byte("{}")
	}

	secJSON := i.SecretsRef
	if len(secJSON) == 0 {
		secJSON = []byte("{}")
	}

	var upID, downID any
	if i.UplinkConverterID != ids.Nil {
		upID = i.UplinkConverterID
	}
	if i.DownlinkConverterID != ids.Nil {
		downID = i.DownlinkConverterID
	}

	query := `
		UPDATE integration
		SET name = $1, type = $2, enabled = $3, uplink_converter_id = $4, downlink_converter_id = $5,
		    routing_key = $6, config = $7, secrets_ref = $8, allow_create_device = $9, default_device_type = $10,
		    remote = $11, version = version + 1, updated_at = NOW()
		WHERE id = $12 AND (tenant_id = $13 OR $13 = '00000000-0000-0000-0000-000000000000'::UUID)
	`
	cmdTag, err := tx.Exec(ctx, query,
		i.Name, i.Type, i.Enabled, upID, downID,
		i.RoutingKey, cfgJSON, secJSON, i.AllowCreateDevice, i.DefaultDeviceType,
		i.Remote, i.ID, i.TenantID,
	)
	if err != nil {
		return fmt.Errorf("failed to update integration: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return errs.NotFound("integration not found")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) DeleteIntegration(ctx context.Context, tenantID, id ids.ID) error {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		DELETE FROM integration
		WHERE id = $1 AND (tenant_id = $2 OR $2 = '00000000-0000-0000-0000-000000000000'::UUID)
	`
	cmdTag, err := tx.Exec(ctx, query, id, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete integration: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return errs.NotFound("integration not found")
	}

	return tx.Commit(ctx)
}

// ==========================================
// Status & Checkpoints
// ==========================================

func (s *PostgresStore) UpsertStatus(ctx context.Context, st *IntegrationStatus) error {
	countersJSON, err := json.Marshal(st.Counters)
	if err != nil || len(countersJSON) == 0 {
		countersJSON = []byte("{}")
	}

	query := `
		INSERT INTO integration_status (integration_id, state, last_error, last_ok_ts, counters, instance)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (integration_id) DO UPDATE SET
			state = EXCLUDED.state,
			last_error = EXCLUDED.last_error,
			last_ok_ts = EXCLUDED.last_ok_ts,
			counters = EXCLUDED.counters,
			instance = EXCLUDED.instance
	`
	_, err = s.pool.Exec(ctx, query,
		st.IntegrationID, st.State, st.LastError, st.LastOkTs, countersJSON, st.Instance,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert integration status: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetStatus(ctx context.Context, integrationID ids.ID) (*IntegrationStatus, error) {
	query := `
		SELECT integration_id, state, last_error, last_ok_ts, counters, instance
		FROM integration_status
		WHERE integration_id = $1
	`
	row := s.pool.QueryRow(ctx, query, integrationID)

	var st IntegrationStatus
	var countersBytes []byte
	err := row.Scan(&st.IntegrationID, &st.State, &st.LastError, &st.LastOkTs, &countersBytes, &st.Instance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &IntegrationStatus{
				IntegrationID: integrationID,
				State:         "RUNNING",
				Counters:      map[string]int64{"rx_events": 0, "tx_events": 0, "errors": 0, "auto_created": 0},
				Instance:      "standalone",
			}, nil
		}
		return nil, fmt.Errorf("failed to get integration status: %w", err)
	}
	if len(countersBytes) > 0 {
		_ = json.Unmarshal(countersBytes, &st.Counters)
	}
	if st.Counters == nil {
		st.Counters = make(map[string]int64)
	}
	return &st, nil
}
