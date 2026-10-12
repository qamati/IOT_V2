package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// PostgresService manages dashboards persisted in PostgreSQL.
type PostgresService struct {
	pool *pgxpool.Pool
}

// NewPostgresService creates a new PostgreSQL-backed Dashboard service.
func NewPostgresService(pool *pgxpool.Pool) Service {
	return &PostgresService{pool: pool}
}

func (s *PostgresService) CreateDashboard(ctx context.Context, tenantID ids.ID, title string, config json.RawMessage) (*model.Dashboard, error) {
	if title == "" {
		return nil, errs.InvalidArgument("dashboard title cannot be empty")
	}
	if len(config) == 0 {
		config = json.RawMessage(`{"widgets":[]}`)
	}

	d := &model.Dashboard{
		ID:            ids.MustNew(),
		CreatedTime:   time.Now().UnixMilli(),
		TenantID:      tenantID,
		Title:         title,
		Configuration: config,
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO dashboard (id, created_time, tenant_id, title, configuration, version)
		VALUES ($1, $2, $3, $4, $5, 1)
	`, d.ID, d.CreatedTime, d.TenantID, d.Title, d.Configuration)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (s *PostgresService) GetDashboard(ctx context.Context, tenantID, id ids.ID) (*model.Dashboard, error) {
	var d model.Dashboard
	var cfgBytes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, title, coalesce(image, ''), configuration
		FROM dashboard
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id).Scan(&d.ID, &d.CreatedTime, &d.TenantID, &d.Title, &d.Image, &cfgBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("dashboard not found")
		}
		return nil, err
	}
	d.Configuration = cfgBytes
	return &d, nil
}

func (s *PostgresService) UpdateDashboard(ctx context.Context, tenantID, id ids.ID, title string, config json.RawMessage) (*model.Dashboard, error) {
	if title == "" {
		return nil, errs.InvalidArgument("dashboard title cannot be empty")
	}

	var d model.Dashboard
	var cfgBytes []byte
	err := s.pool.QueryRow(ctx, `
		UPDATE dashboard
		SET title = $1, configuration = $2, version = version + 1
		WHERE tenant_id = $3 AND id = $4
		RETURNING id, created_time, tenant_id, title, coalesce(image, ''), configuration
	`, title, config, tenantID, id).Scan(&d.ID, &d.CreatedTime, &d.TenantID, &d.Title, &d.Image, &cfgBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("dashboard not found")
		}
		return nil, err
	}
	d.Configuration = cfgBytes
	return &d, nil
}

func (s *PostgresService) DeleteDashboard(ctx context.Context, tenantID, id ids.ID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM dashboard WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("dashboard not found")
	}
	return nil
}

func (s *PostgresService) ListDashboards(ctx context.Context, tenantID ids.ID, page, pageSize int) ([]model.Dashboard, int, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := page * pageSize

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM dashboard WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, created_time, tenant_id, title, coalesce(image, ''), configuration
		FROM dashboard
		WHERE tenant_id = $1
		ORDER BY created_time DESC
		LIMIT $2 OFFSET $3
	`, tenantID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []model.Dashboard
	for rows.Next() {
		var d model.Dashboard
		var cfgBytes []byte
		if err := rows.Scan(&d.ID, &d.CreatedTime, &d.TenantID, &d.Title, &d.Image, &cfgBytes); err != nil {
			return nil, 0, err
		}
		d.Configuration = cfgBytes
		result = append(result, d)
	}
	if result == nil {
		result = make([]model.Dashboard, 0)
	}
	return result, total, nil
}
