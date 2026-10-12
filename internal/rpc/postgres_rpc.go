package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Store defines persistent storage operations for RPC commands.
type Store interface {
	CreateCommand(ctx context.Context, cmd *model.RpcCommand) error
	GetCommand(ctx context.Context, tenantID, cmdID ids.ID) (*model.RpcCommand, error)
	UpdateCommand(ctx context.Context, tenantID, cmdID ids.ID, status model.RpcStatus, response json.RawMessage) error
	ListDeviceRpcHistory(ctx context.Context, tenantID, deviceID ids.ID, page, pageSize int) ([]model.RpcCommand, int, error)
	ListQueuedRpc(ctx context.Context, deviceID ids.ID) ([]model.RpcCommand, error)
	DeleteCommand(ctx context.Context, tenantID, cmdID ids.ID) error
	SweepExpired(ctx context.Context) (int, error)
}

// PostgresStore implements Store using TimescaleDB/PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL RPC store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) withTenant(ctx context.Context, tenantID ids.ID) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("rpc: failed to begin tx: %w", err)
	}

	_, err = tx.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID.String())
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("rpc: failed to set tenant RLS context: %w", err)
	}
	return tx, nil
}

func (s *PostgresStore) CreateCommand(ctx context.Context, cmd *model.RpcCommand) error {
	tx, err := s.withTenant(ctx, cmd.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	paramsJSON := cmd.Params
	if len(paramsJSON) == 0 {
		paramsJSON = []byte("{}")
	}

	query := `
		INSERT INTO rpc (
			id, created_time, tenant_id, device_id, method, params, response,
			status, two_way, expiration_time, additional_info, retries, max_retries, request_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14
		)
	`

	addInfoJSON := cmd.AdditionalInfo
	if len(addInfoJSON) == 0 {
		addInfoJSON = []byte("{}")
	}

	var respJSON []byte
	if len(cmd.Response) > 0 {
		respJSON = cmd.Response
	}

	_, err = tx.Exec(ctx, query,
		cmd.ID,
		cmd.CreatedTime,
		cmd.TenantID,
		cmd.DeviceID,
		cmd.Method,
		paramsJSON,
		respJSON,
		string(cmd.Status),
		cmd.TwoWay,
		cmd.ExpirationTime,
		addInfoJSON,
		cmd.Retries,
		cmd.MaxRetries,
		cmd.RequestID,
	)
	if err != nil {
		return fmt.Errorf("rpc: failed to insert command: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetCommand(ctx context.Context, tenantID, cmdID ids.ID) (*model.RpcCommand, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, created_time, tenant_id, device_id, method, params, response,
		       status, two_way, expiration_time, additional_info, retries, max_retries,
		       response_time, request_id
		FROM rpc
		WHERE tenant_id = $1 AND id = $2
	`

	var cmd model.RpcCommand
	var statusStr string
	var respTime *int64
	var reqID *string

	err = tx.QueryRow(ctx, query, tenantID, cmdID).Scan(
		&cmd.ID,
		&cmd.CreatedTime,
		&cmd.TenantID,
		&cmd.DeviceID,
		&cmd.Method,
		&cmd.Params,
		&cmd.Response,
		&statusStr,
		&cmd.TwoWay,
		&cmd.ExpirationTime,
		&cmd.AdditionalInfo,
		&cmd.Retries,
		&cmd.MaxRetries,
		&respTime,
		&reqID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("rpc command not found")
		}
		return nil, fmt.Errorf("rpc: failed to query command: %w", err)
	}

	cmd.Status = model.RpcStatus(statusStr)
	if respTime != nil {
		cmd.ResponseTime = *respTime
	}
	if reqID != nil {
		cmd.RequestID = *reqID
	}

	return &cmd, tx.Commit(ctx)
}

func (s *PostgresStore) UpdateCommand(ctx context.Context, tenantID, cmdID ids.ID, status model.RpcStatus, response json.RawMessage) error {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UnixMilli()
	query := `
		UPDATE rpc
		SET status = $1, response = COALESCE($2, response), response_time = $3
		WHERE tenant_id = $4 AND id = $5
	`

	_, err = tx.Exec(ctx, query, string(status), response, now, tenantID, cmdID)
	if err != nil {
		return fmt.Errorf("rpc: failed to update command: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) ListDeviceRpcHistory(ctx context.Context, tenantID, deviceID ids.ID, page, pageSize int) ([]model.RpcCommand, int, error) {
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
	countQuery := `SELECT COUNT(*) FROM rpc WHERE tenant_id = $1 AND device_id = $2`
	if err := tx.QueryRow(ctx, countQuery, tenantID, deviceID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("rpc: failed to count commands: %w", err)
	}

	query := `
		SELECT id, created_time, tenant_id, device_id, method, params, response,
		       status, two_way, expiration_time, additional_info, retries, max_retries,
		       response_time, request_id
		FROM rpc
		WHERE tenant_id = $1 AND device_id = $2
		ORDER BY created_time DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := tx.Query(ctx, query, tenantID, deviceID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("rpc: failed to query device rpc history: %w", err)
	}
	defer rows.Close()

	commands := make([]model.RpcCommand, 0)
	for rows.Next() {
		var cmd model.RpcCommand
		var statusStr string
		var respTime *int64
		var reqID *string

		if err := rows.Scan(
			&cmd.ID,
			&cmd.CreatedTime,
			&cmd.TenantID,
			&cmd.DeviceID,
			&cmd.Method,
			&cmd.Params,
			&cmd.Response,
			&statusStr,
			&cmd.TwoWay,
			&cmd.ExpirationTime,
			&cmd.AdditionalInfo,
			&cmd.Retries,
			&cmd.MaxRetries,
			&respTime,
			&reqID,
		); err != nil {
			return nil, 0, fmt.Errorf("rpc: scan error: %w", err)
		}

		cmd.Status = model.RpcStatus(statusStr)
		if respTime != nil {
			cmd.ResponseTime = *respTime
		}
		if reqID != nil {
			cmd.RequestID = *reqID
		}
		commands = append(commands, cmd)
	}

	return commands, total, tx.Commit(ctx)
}

func (s *PostgresStore) ListQueuedRpc(ctx context.Context, deviceID ids.ID) ([]model.RpcCommand, error) {
	query := `
		SELECT id, created_time, tenant_id, device_id, method, params, response,
		       status, two_way, expiration_time, additional_info, retries, max_retries,
		       response_time, request_id
		FROM rpc
		WHERE device_id = $1 AND status = 'QUEUED' AND (expiration_time = 0 OR expiration_time > $2)
		ORDER BY created_time ASC
		LIMIT 10
	`

	now := time.Now().UnixMilli()
	rows, err := s.pool.Query(ctx, query, deviceID, now)
	if err != nil {
		return nil, fmt.Errorf("rpc: failed to list queued commands: %w", err)
	}
	defer rows.Close()

	commands := make([]model.RpcCommand, 0)
	for rows.Next() {
		var cmd model.RpcCommand
		var statusStr string
		var respTime *int64
		var reqID *string

		if err := rows.Scan(
			&cmd.ID,
			&cmd.CreatedTime,
			&cmd.TenantID,
			&cmd.DeviceID,
			&cmd.Method,
			&cmd.Params,
			&cmd.Response,
			&statusStr,
			&cmd.TwoWay,
			&cmd.ExpirationTime,
			&cmd.AdditionalInfo,
			&cmd.Retries,
			&cmd.MaxRetries,
			&respTime,
			&reqID,
		); err != nil {
			return nil, fmt.Errorf("rpc: scan error: %w", err)
		}

		cmd.Status = model.RpcStatus(statusStr)
		if respTime != nil {
			cmd.ResponseTime = *respTime
		}
		if reqID != nil {
			cmd.RequestID = *reqID
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

func (s *PostgresStore) DeleteCommand(ctx context.Context, tenantID, cmdID ids.ID) error {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM rpc WHERE tenant_id = $1 AND id = $2", tenantID, cmdID)
	if err != nil {
		return fmt.Errorf("rpc: failed to delete command: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) SweepExpired(ctx context.Context) (int, error) {
	now := time.Now().UnixMilli()
	query := `
		UPDATE rpc
		SET status = 'EXPIRED'
		WHERE status IN ('QUEUED', 'SENT') AND expiration_time > 0 AND expiration_time <= $1
	`
	tag, err := s.pool.Exec(ctx, query, now)
	if err != nil {
		return 0, fmt.Errorf("rpc: failed to sweep expired: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
