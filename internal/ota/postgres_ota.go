package ota

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Store defines persistent storage operations for OTA packages and states.
type Store interface {
	CreatePackage(ctx context.Context, pkg *model.OtaPackage) error
	GetPackage(ctx context.Context, tenantID, pkgID ids.ID) (*model.OtaPackage, error)
	ListPackages(ctx context.Context, tenantID ids.ID, pkgType string, page, pageSize int) ([]model.OtaPackage, int, error)
	DeletePackage(ctx context.Context, tenantID, pkgID ids.ID) error
	SavePackageData(ctx context.Context, tenantID, pkgID ids.ID, data []byte) error
	GetPackageData(ctx context.Context, tenantID, pkgID ids.ID) ([]byte, error)
	AssignPackage(ctx context.Context, assign *model.OtaAssignment) error
	GetAssignment(ctx context.Context, tenantID ids.ID, scope string, targetID ids.ID, kind string) (*model.OtaAssignment, error)
	ListAssignments(ctx context.Context, tenantID ids.ID) ([]model.OtaAssignment, error)
	UpdateDeviceOtaState(ctx context.Context, state *model.DeviceOtaState) error
	GetDeviceOtaState(ctx context.Context, deviceID ids.ID, kind string) (*model.DeviceOtaState, error)
	ListFleetOtaStates(ctx context.Context, packageID *ids.ID) ([]model.DeviceOtaState, error)
}

// PostgresStore implements Store using TimescaleDB/PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL OTA store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) withTenant(ctx context.Context, tenantID ids.ID) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ota: failed to begin tx: %w", err)
	}

	_, err = tx.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID.String())
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("ota: failed to set tenant RLS context: %w", err)
	}
	return tx, nil
}

func (s *PostgresStore) CreatePackage(ctx context.Context, pkg *model.OtaPackage) error {
	tx, err := s.withTenant(ctx, pkg.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO ota_package (
			id, created_time, tenant_id, type, title, version, tag, url,
			file_name, content_type, checksum_algorithm, checksum, data_size,
			data, additional_info
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15
		)
	`

	addInfo := pkg.AdditionalInfo
	if len(addInfo) == 0 {
		addInfo = []byte("{}")
	}

	_, err = tx.Exec(ctx, query,
		pkg.ID,
		pkg.CreatedTime,
		pkg.TenantID,
		string(pkg.Type),
		pkg.Title,
		pkg.Version,
		pkg.Tag,
		pkg.URL,
		pkg.FileName,
		pkg.ContentType,
		pkg.ChecksumAlgorithm,
		pkg.Checksum,
		pkg.DataSize,
		pkg.Data,
		addInfo,
	)
	if err != nil {
		return fmt.Errorf("ota: failed to insert package: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetPackage(ctx context.Context, tenantID, pkgID ids.ID) (*model.OtaPackage, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, created_time, tenant_id, type, title, version, coalesce(tag, ''),
		       coalesce(url, ''), coalesce(file_name, ''), coalesce(content_type, ''),
		       checksum_algorithm, checksum, data_size, additional_info
		FROM ota_package
		WHERE tenant_id = $1 AND id = $2
	`

	var pkg model.OtaPackage
	var typeStr string

	err = tx.QueryRow(ctx, query, tenantID, pkgID).Scan(
		&pkg.ID,
		&pkg.CreatedTime,
		&pkg.TenantID,
		&typeStr,
		&pkg.Title,
		&pkg.Version,
		&pkg.Tag,
		&pkg.URL,
		&pkg.FileName,
		&pkg.ContentType,
		&pkg.ChecksumAlgorithm,
		&pkg.Checksum,
		&pkg.DataSize,
		&pkg.AdditionalInfo,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("ota package not found")
		}
		return nil, fmt.Errorf("ota: failed to query package: %w", err)
	}

	pkg.Type = model.OtaPackageType(typeStr)
	return &pkg, tx.Commit(ctx)
}

func (s *PostgresStore) ListPackages(ctx context.Context, tenantID ids.ID, pkgType string, page, pageSize int) ([]model.OtaPackage, int, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	if pkgType != "" {
		_ = tx.QueryRow(ctx, "SELECT COUNT(*) FROM ota_package WHERE tenant_id = $1 AND type = $2", tenantID, pkgType).Scan(&total)
	} else {
		_ = tx.QueryRow(ctx, "SELECT COUNT(*) FROM ota_package WHERE tenant_id = $1", tenantID).Scan(&total)
	}

	var rows pgx.Rows
	if pkgType != "" {
		rows, err = tx.Query(ctx, `
			SELECT id, created_time, tenant_id, type, title, version, coalesce(tag, ''),
			       coalesce(url, ''), coalesce(file_name, ''), coalesce(content_type, ''),
			       checksum_algorithm, checksum, data_size, additional_info
			FROM ota_package
			WHERE tenant_id = $1 AND type = $2
			ORDER BY created_time DESC
			LIMIT $3 OFFSET $4
		`, tenantID, pkgType, pageSize, offset)
	} else {
		rows, err = tx.Query(ctx, `
			SELECT id, created_time, tenant_id, type, title, version, coalesce(tag, ''),
			       coalesce(url, ''), coalesce(file_name, ''), coalesce(content_type, ''),
			       checksum_algorithm, checksum, data_size, additional_info
			FROM ota_package
			WHERE tenant_id = $1
			ORDER BY created_time DESC
			LIMIT $2 OFFSET $3
		`, tenantID, pageSize, offset)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("ota: failed to list packages: %w", err)
	}
	defer rows.Close()

	packages := make([]model.OtaPackage, 0)
	for rows.Next() {
		var pkg model.OtaPackage
		var typeStr string

		if err := rows.Scan(
			&pkg.ID,
			&pkg.CreatedTime,
			&pkg.TenantID,
			&typeStr,
			&pkg.Title,
			&pkg.Version,
			&pkg.Tag,
			&pkg.URL,
			&pkg.FileName,
			&pkg.ContentType,
			&pkg.ChecksumAlgorithm,
			&pkg.Checksum,
			&pkg.DataSize,
			&pkg.AdditionalInfo,
		); err != nil {
			return nil, 0, fmt.Errorf("ota: scan error: %w", err)
		}

		pkg.Type = model.OtaPackageType(typeStr)
		packages = append(packages, pkg)
	}

	return packages, total, tx.Commit(ctx)
}

func (s *PostgresStore) DeletePackage(ctx context.Context, tenantID, pkgID ids.ID) error {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM ota_package WHERE tenant_id = $1 AND id = $2", tenantID, pkgID)
	if err != nil {
		return fmt.Errorf("ota: failed to delete package: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) SavePackageData(ctx context.Context, tenantID, pkgID ids.ID, data []byte) error {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "UPDATE ota_package SET data = $1, data_size = $2 WHERE tenant_id = $3 AND id = $4",
		data, int64(len(data)), tenantID, pkgID)
	if err != nil {
		return fmt.Errorf("ota: failed to save package binary: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetPackageData(ctx context.Context, tenantID, pkgID ids.ID) ([]byte, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var data []byte
	err = tx.QueryRow(ctx, "SELECT data FROM ota_package WHERE tenant_id = $1 AND id = $2", tenantID, pkgID).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("ota package data not found")
		}
		return nil, fmt.Errorf("ota: failed to query package binary: %w", err)
	}

	return data, tx.Commit(ctx)
}

func (s *PostgresStore) AssignPackage(ctx context.Context, assign *model.OtaAssignment) error {
	tx, err := s.withTenant(ctx, assign.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO ota_assignment (id, created_time, tenant_id, kind, package_id, scope, target_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (scope, target_id, kind)
		DO UPDATE SET package_id = EXCLUDED.package_id, created_time = EXCLUDED.created_time
	`

	_, err = tx.Exec(ctx, query,
		assign.ID,
		assign.CreatedTime,
		assign.TenantID,
		assign.Kind,
		assign.PackageID,
		assign.Scope,
		assign.TargetID,
	)
	if err != nil {
		return fmt.Errorf("ota: failed to save assignment: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetAssignment(ctx context.Context, tenantID ids.ID, scope string, targetID ids.ID, kind string) (*model.OtaAssignment, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, created_time, tenant_id, kind, package_id, scope, target_id
		FROM ota_assignment
		WHERE tenant_id = $1 AND scope = $2 AND target_id = $3 AND kind = $4
	`

	var a model.OtaAssignment
	err = tx.QueryRow(ctx, query, tenantID, scope, targetID, kind).Scan(
		&a.ID,
		&a.CreatedTime,
		&a.TenantID,
		&a.Kind,
		&a.PackageID,
		&a.Scope,
		&a.TargetID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("ota assignment not found")
		}
		return nil, fmt.Errorf("ota: failed to query assignment: %w", err)
	}

	return &a, tx.Commit(ctx)
}

func (s *PostgresStore) ListAssignments(ctx context.Context, tenantID ids.ID) ([]model.OtaAssignment, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, created_time, tenant_id, kind, package_id, scope, target_id
		FROM ota_assignment
		WHERE tenant_id = $1
		ORDER BY created_time DESC
	`

	rows, err := tx.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("ota: failed to list assignments: %w", err)
	}
	defer rows.Close()

	list := make([]model.OtaAssignment, 0)
	for rows.Next() {
		var a model.OtaAssignment
		if err := rows.Scan(
			&a.ID,
			&a.CreatedTime,
			&a.TenantID,
			&a.Kind,
			&a.PackageID,
			&a.Scope,
			&a.TargetID,
		); err != nil {
			return nil, fmt.Errorf("ota: scan assignment error: %w", err)
		}
		list = append(list, a)
	}

	return list, tx.Commit(ctx)
}

func (s *PostgresStore) UpdateDeviceOtaState(ctx context.Context, state *model.DeviceOtaState) error {
	query := `
		INSERT INTO device_ota_state (device_id, kind, package_id, state, progress, error, updated_ts, attempts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (device_id, kind)
		DO UPDATE SET
			package_id = EXCLUDED.package_id,
			state = EXCLUDED.state,
			progress = EXCLUDED.progress,
			error = EXCLUDED.error,
			updated_ts = EXCLUDED.updated_ts,
			attempts = device_ota_state.attempts + 1
	`

	_, err := s.pool.Exec(ctx, query,
		state.DeviceID,
		state.Kind,
		state.PackageID,
		state.State,
		state.Progress,
		state.Error,
		state.UpdatedTs,
		state.Attempts,
	)
	if err != nil {
		return fmt.Errorf("ota: failed to update device state: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetDeviceOtaState(ctx context.Context, deviceID ids.ID, kind string) (*model.DeviceOtaState, error) {
	query := `
		SELECT device_id, kind, package_id, state, progress, coalesce(error, ''), updated_ts, attempts
		FROM device_ota_state
		WHERE device_id = $1 AND kind = $2
	`

	var state model.DeviceOtaState
	err := s.pool.QueryRow(ctx, query, deviceID, kind).Scan(
		&state.DeviceID,
		&state.Kind,
		&state.PackageID,
		&state.State,
		&state.Progress,
		&state.Error,
		&state.UpdatedTs,
		&state.Attempts,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("device ota state not found")
		}
		return nil, fmt.Errorf("ota: failed to query device state: %w", err)
	}
	return &state, nil
}

func (s *PostgresStore) ListFleetOtaStates(ctx context.Context, packageID *ids.ID) ([]model.DeviceOtaState, error) {
	var rows pgx.Rows
	var err error

	if packageID != nil {
		rows, err = s.pool.Query(ctx, `
			SELECT device_id, kind, package_id, state, progress, coalesce(error, ''), updated_ts, attempts
			FROM device_ota_state
			WHERE package_id = $1
			ORDER BY updated_ts DESC
		`, *packageID)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT device_id, kind, package_id, state, progress, coalesce(error, ''), updated_ts, attempts
			FROM device_ota_state
			ORDER BY updated_ts DESC
			LIMIT 100
		`)
	}

	if err != nil {
		return nil, fmt.Errorf("ota: failed to list fleet states: %w", err)
	}
	defer rows.Close()

	list := make([]model.DeviceOtaState, 0)
	for rows.Next() {
		var state model.DeviceOtaState
		if err := rows.Scan(
			&state.DeviceID,
			&state.Kind,
			&state.PackageID,
			&state.State,
			&state.Progress,
			&state.Error,
			&state.UpdatedTs,
			&state.Attempts,
		); err != nil {
			return nil, fmt.Errorf("ota: scan error: %w", err)
		}
		list = append(list, state)
	}

	return list, nil
}
