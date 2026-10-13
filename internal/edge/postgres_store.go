package edge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// PostgresStore implements durable Edge persistence in PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore initializes a PostgreSQL edge store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) withTenant(ctx context.Context, tenantID ids.ID) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("edge: failed to begin tx: %w", err)
	}

	if tenantID != ids.Nil {
		_, err = tx.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID.String())
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("edge: failed to set tenant RLS context: %w", err)
		}
	}
	return tx, nil
}

func (s *PostgresStore) CreateEdge(ctx context.Context, e *EdgeNode) error {
	if e.ID == ids.Nil {
		e.ID = ids.MustNew()
	}
	if e.EdgeKey == "" {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		e.EdgeKey = fmt.Sprintf("EDGE_%s", hex.EncodeToString(b))
	}
	if e.EdgeSecret == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		e.EdgeSecret = "sec_edge_" + hex.EncodeToString(b)
	}
	if e.Status == "" {
		e.Status = StatusOffline
	}
	if e.FirmwareVersion == "" {
		e.FirmwareVersion = "v1.0.0-edge"
	}
	if e.HardwareModel == "" {
		e.HardwareModel = "Advantech UNO-2271G"
	}
	if e.AssignedDevices == nil {
		e.AssignedDevices = []ids.ID{}
	}
	if e.AssignedProfiles == nil {
		e.AssignedProfiles = []ids.ID{}
	}
	e.Version = 1

	tx, err := s.withTenant(ctx, e.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	devsJSON, _ := json.Marshal(e.AssignedDevices)
	profsJSON, _ := json.Marshal(e.AssignedProfiles)

	query := `
		INSERT INTO edge (
			id, tenant_id, name, edge_key, edge_secret, status,
			firmware_version, hardware_model, last_sync_ts, backlog_count,
			assigned_devices, assigned_profiles, downlink_seq, uplink_seq, version,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW())
	`

	_, err = tx.Exec(ctx, query,
		e.ID.String(), e.TenantID.String(), e.Name, e.EdgeKey, e.EdgeSecret, e.Status,
		e.FirmwareVersion, e.HardwareModel, e.LastSyncTs, e.BacklogCount,
		devsJSON, profsJSON, e.DownlinkSeq, e.UplinkSeq, e.Version,
	)
	if err != nil {
		return fmt.Errorf("edge: failed to insert edge node: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetEdge(ctx context.Context, tenantID, edgeID ids.ID) (*EdgeNode, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, tenant_id, name, edge_key, edge_secret, status,
		       firmware_version, hardware_model, last_sync_ts, backlog_count,
		       assigned_devices, assigned_profiles, downlink_seq, uplink_seq, version
		FROM edge
		WHERE id = $1 AND ($2 = '00000000-0000-0000-0000-000000000000'::UUID OR tenant_id = $2)
	`

	var (
		idStr, tIDStr string
		e             EdgeNode
		devsJSON      []byte
		profsJSON     []byte
	)

	err = tx.QueryRow(ctx, query, edgeID.String(), tenantID.String()).Scan(
		&idStr, &tIDStr, &e.Name, &e.EdgeKey, &e.EdgeSecret, &e.Status,
		&e.FirmwareVersion, &e.HardwareModel, &e.LastSyncTs, &e.BacklogCount,
		&devsJSON, &profsJSON, &e.DownlinkSeq, &e.UplinkSeq, &e.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("edge node not found")
		}
		return nil, fmt.Errorf("edge: query failed: %w", err)
	}

	e.ID = ids.MustParse(idStr)
	e.TenantID = ids.MustParse(tIDStr)
	_ = json.Unmarshal(devsJSON, &e.AssignedDevices)
	_ = json.Unmarshal(profsJSON, &e.AssignedProfiles)

	return &e, nil
}

func (s *PostgresStore) GetEdgeByKey(ctx context.Context, edgeKey string) (*EdgeNode, error) {
	query := `
		SELECT id, tenant_id, name, edge_key, edge_secret, status,
		       firmware_version, hardware_model, last_sync_ts, backlog_count,
		       assigned_devices, assigned_profiles, downlink_seq, uplink_seq, version
		FROM edge
		WHERE edge_key = $1
	`

	var (
		idStr, tIDStr string
		e             EdgeNode
		devsJSON      []byte
		profsJSON     []byte
	)

	err := s.pool.QueryRow(ctx, query, edgeKey).Scan(
		&idStr, &tIDStr, &e.Name, &e.EdgeKey, &e.EdgeSecret, &e.Status,
		&e.FirmwareVersion, &e.HardwareModel, &e.LastSyncTs, &e.BacklogCount,
		&devsJSON, &profsJSON, &e.DownlinkSeq, &e.UplinkSeq, &e.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("edge node not found for key")
		}
		return nil, fmt.Errorf("edge: query by key failed: %w", err)
	}

	e.ID = ids.MustParse(idStr)
	e.TenantID = ids.MustParse(tIDStr)
	_ = json.Unmarshal(devsJSON, &e.AssignedDevices)
	_ = json.Unmarshal(profsJSON, &e.AssignedProfiles)

	return &e, nil
}

func (s *PostgresStore) ListEdges(ctx context.Context, tenantID ids.ID) ([]*EdgeNode, error) {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, tenant_id, name, edge_key, edge_secret, status,
		       firmware_version, hardware_model, last_sync_ts, backlog_count,
		       assigned_devices, assigned_profiles, downlink_seq, uplink_seq, version
		FROM edge
		WHERE ($1 = '00000000-0000-0000-0000-000000000000'::UUID OR tenant_id = $1)
		ORDER BY created_at DESC
	`

	rows, err := tx.Query(ctx, query, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("edge: list query failed: %w", err)
	}
	defer rows.Close()

	var result []*EdgeNode
	for rows.Next() {
		var (
			idStr, tIDStr string
			e             EdgeNode
			devsJSON      []byte
			profsJSON     []byte
		)
		if err := rows.Scan(
			&idStr, &tIDStr, &e.Name, &e.EdgeKey, &e.EdgeSecret, &e.Status,
			&e.FirmwareVersion, &e.HardwareModel, &e.LastSyncTs, &e.BacklogCount,
			&devsJSON, &profsJSON, &e.DownlinkSeq, &e.UplinkSeq, &e.Version,
		); err != nil {
			return nil, err
		}

		e.ID = ids.MustParse(idStr)
		e.TenantID = ids.MustParse(tIDStr)
		_ = json.Unmarshal(devsJSON, &e.AssignedDevices)
		_ = json.Unmarshal(profsJSON, &e.AssignedProfiles)
		result = append(result, &e)
	}

	return result, nil
}

func (s *PostgresStore) UpdateEdge(ctx context.Context, e *EdgeNode) error {
	tx, err := s.withTenant(ctx, e.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	devsJSON, _ := json.Marshal(e.AssignedDevices)
	profsJSON, _ := json.Marshal(e.AssignedProfiles)

	query := `
		UPDATE edge
		SET name = $1, status = $2, firmware_version = $3, hardware_model = $4,
		    assigned_devices = $5, assigned_profiles = $6, version = version + 1, updated_at = NOW()
		WHERE id = $7 AND ($8 = '00000000-0000-0000-0000-000000000000'::UUID OR tenant_id = $8)
	`

	cmd, err := tx.Exec(ctx, query,
		e.Name, e.Status, e.FirmwareVersion, e.HardwareModel,
		devsJSON, profsJSON, e.ID.String(), e.TenantID.String(),
	)
	if err != nil {
		return fmt.Errorf("edge: update failed: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.NotFound("edge node not found")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) DeleteEdge(ctx context.Context, tenantID, edgeID ids.ID) error {
	tx, err := s.withTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx, "DELETE FROM edge WHERE id = $1 AND ($2 = '00000000-0000-0000-0000-000000000000'::UUID OR tenant_id = $2)", edgeID.String(), tenantID.String())
	if err != nil {
		return fmt.Errorf("edge: delete failed: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.NotFound("edge node not found")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) AssignDevices(ctx context.Context, edgeID ids.ID, deviceIDs []ids.ID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, devID := range deviceIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO edge_device_assignment (edge_id, device_id, assigned_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (edge_id, device_id) DO NOTHING
		`, edgeID.String(), devID.String())
		if err != nil {
			return fmt.Errorf("edge: assign device failed: %w", err)
		}
	}

	// Refresh assigned_devices JSON array on edge row
	rows, err := tx.Query(ctx, "SELECT device_id FROM edge_device_assignment WHERE edge_id = $1", edgeID.String())
	if err != nil {
		return err
	}
	defer rows.Close()

	var allDevs []ids.ID
	for rows.Next() {
		var dStr string
		if err := rows.Scan(&dStr); err == nil {
			allDevs = append(allDevs, ids.MustParse(dStr))
		}
	}
	devsJSON, _ := json.Marshal(allDevs)

	_, err = tx.Exec(ctx, "UPDATE edge SET assigned_devices = $1, updated_at = NOW() WHERE id = $2", devsJSON, edgeID.String())
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) UnassignDevice(ctx context.Context, edgeID ids.ID, deviceID ids.ID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM edge_device_assignment WHERE edge_id = $1 AND device_id = $2", edgeID.String(), deviceID.String())
	if err != nil {
		return fmt.Errorf("edge: unassign failed: %w", err)
	}

	// Refresh assigned_devices array
	rows, err := tx.Query(ctx, "SELECT device_id FROM edge_device_assignment WHERE edge_id = $1", edgeID.String())
	if err != nil {
		return err
	}
	defer rows.Close()

	var allDevs []ids.ID
	for rows.Next() {
		var dStr string
		if err := rows.Scan(&dStr); err == nil {
			allDevs = append(allDevs, ids.MustParse(dStr))
		}
	}
	devsJSON, _ := json.Marshal(allDevs)

	_, err = tx.Exec(ctx, "UPDATE edge SET assigned_devices = $1, updated_at = NOW() WHERE id = $2", devsJSON, edgeID.String())
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetAssignedDevices(ctx context.Context, edgeID ids.ID) ([]ids.ID, error) {
	rows, err := s.pool.Query(ctx, "SELECT device_id FROM edge_device_assignment WHERE edge_id = $1 ORDER BY assigned_at ASC", edgeID.String())
	if err != nil {
		return nil, fmt.Errorf("edge: get assignments failed: %w", err)
	}
	defer rows.Close()

	var result []ids.ID
	for rows.Next() {
		var dStr string
		if err := rows.Scan(&dStr); err == nil {
			result = append(result, ids.MustParse(dStr))
		}
	}
	return result, nil
}

func (s *PostgresStore) RecordSyncJournal(ctx context.Context, entry *SyncJournalEntry) error {
	if entry.ID == ids.Nil {
		entry.ID = ids.MustNew()
	}
	if entry.Status == "" {
		entry.Status = "PROCESSED"
	}
	if entry.SyncedAt == 0 {
		entry.SyncedAt = time.Now().UnixMilli()
	}

	query := `
		INSERT INTO edge_sync_journal (
			id, edge_id, sequence, lane_l0_count, lane_l1_count, lane_l2_count,
			processed_items, status, synced_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TO_TIMESTAMP($9 / 1000.0))
		ON CONFLICT (edge_id, sequence) DO UPDATE
		SET status = EXCLUDED.status, processed_items = EXCLUDED.processed_items
	`

	_, err := s.pool.Exec(ctx, query,
		entry.ID.String(), entry.EdgeID.String(), entry.Sequence,
		entry.LaneL0Count, entry.LaneL1Count, entry.LaneL2Count,
		entry.ProcessedItems, entry.Status, entry.SyncedAt,
	)
	if err != nil {
		return fmt.Errorf("edge: record journal failed: %w", err)
	}

	return nil
}

func (s *PostgresStore) ListSyncJournal(ctx context.Context, edgeID ids.ID, limit int) ([]*SyncJournalEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, edge_id, sequence, lane_l0_count, lane_l1_count, lane_l2_count,
		       processed_items, status, (EXTRACT(EPOCH FROM synced_at) * 1000)::BIGINT
		FROM edge_sync_journal
		WHERE edge_id = $1
		ORDER BY sequence DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, edgeID.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("edge: query journal failed: %w", err)
	}
	defer rows.Close()

	var result []*SyncJournalEntry
	for rows.Next() {
		var (
			idStr, eIDStr string
			j             SyncJournalEntry
		)
		if err := rows.Scan(
			&idStr, &eIDStr, &j.Sequence, &j.LaneL0Count, &j.LaneL1Count, &j.LaneL2Count,
			&j.ProcessedItems, &j.Status, &j.SyncedAt,
		); err != nil {
			return nil, err
		}
		j.ID = ids.MustParse(idStr)
		j.EdgeID = ids.MustParse(eIDStr)
		result = append(result, &j)
	}

	return result, nil
}

func (s *PostgresStore) UpdateSyncState(ctx context.Context, edgeID ids.ID, status string, upSeq int64, lastSyncTs int64, backlog int64) error {
	query := `
		UPDATE edge
		SET status = $1, uplink_seq = $2, last_sync_ts = $3, backlog_count = $4, updated_at = NOW()
		WHERE id = $5
	`
	_, err := s.pool.Exec(ctx, query, status, upSeq, lastSyncTs, backlog, edgeID.String())
	if err != nil {
		return fmt.Errorf("edge: update sync state failed: %w", err)
	}
	return nil
}
