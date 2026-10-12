package rule

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

// PostgresStore implements Rule Chain persistence backed by TimescaleDB / PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL-backed RuleChain Store.
func NewPostgresStore(pool *pgxpool.Pool) Store {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) ListRuleChains(ctx context.Context, tenantID ids.ID, page, limit int) ([]*model.RuleChain, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var exists bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant WHERE id = $1)`, tenantID).Scan(&exists)
	if !exists {
		var fallbackID ids.ID
		if err := s.pool.QueryRow(ctx, `SELECT id FROM tenant LIMIT 1`).Scan(&fallbackID); err == nil {
			tenantID = fallbackID
		}
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM rule_chain WHERE tenant_id = $1;`
	if err := s.pool.QueryRow(ctx, countQuery, tenantID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres_rule_store: count query failed: %w", err)
	}

	query := `
		SELECT id, created_time, tenant_id, name, type, is_root, published_revision, additional_info, version
		FROM rule_chain
		WHERE tenant_id = $1
		ORDER BY is_root DESC, created_time ASC
		LIMIT $2 OFFSET $3;
	`
	rows, err := s.pool.Query(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres_rule_store: list query failed: %w", err)
	}
	defer rows.Close()

	chains := make([]*model.RuleChain, 0)
	for rows.Next() {
		var c model.RuleChain
		var addInfoBytes []byte
		if err := rows.Scan(
			&c.ID,
			&c.CreatedTime,
			&c.TenantID,
			&c.Name,
			&c.Type,
			&c.IsRoot,
			&c.PublishedRevision,
			&addInfoBytes,
			&c.Version,
		); err != nil {
			return nil, 0, fmt.Errorf("postgres_rule_store: scan failed: %w", err)
		}
		if len(addInfoBytes) > 0 {
			c.AdditionalInfo = json.RawMessage(addInfoBytes)
		} else {
			c.AdditionalInfo = json.RawMessage(`{}`)
		}
		chains = append(chains, &c)
	}

	return chains, total, nil
}

func (s *PostgresStore) GetRuleChain(ctx context.Context, tenantID, id ids.ID) (*model.RuleChain, error) {
	query := `
		SELECT id, created_time, tenant_id, name, type, is_root, published_revision, additional_info, version
		FROM rule_chain
		WHERE tenant_id = $1 AND id = $2;
	`
	var c model.RuleChain
	var addInfoBytes []byte
	err := s.pool.QueryRow(ctx, query, tenantID, id).Scan(
		&c.ID,
		&c.CreatedTime,
		&c.TenantID,
		&c.Name,
		&c.Type,
		&c.IsRoot,
		&c.PublishedRevision,
		&addInfoBytes,
		&c.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("Rule Chain not found")
		}
		return nil, fmt.Errorf("postgres_rule_store: get failed: %w", err)
	}
	if len(addInfoBytes) > 0 {
		c.AdditionalInfo = json.RawMessage(addInfoBytes)
	} else {
		c.AdditionalInfo = json.RawMessage(`{}`)
	}
	return &c, nil
}

func (s *PostgresStore) GetRootRuleChain(ctx context.Context, tenantID ids.ID) (*model.RuleChain, error) {
	var exists bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant WHERE id = $1)`, tenantID).Scan(&exists)
	if !exists {
		var fallbackID ids.ID
		if err := s.pool.QueryRow(ctx, `SELECT id FROM tenant LIMIT 1`).Scan(&fallbackID); err == nil {
			tenantID = fallbackID
		}
	}

	query := `
		SELECT id, created_time, tenant_id, name, type, is_root, published_revision, additional_info, version
		FROM rule_chain
		WHERE tenant_id = $1 AND is_root = true
		LIMIT 1;
	`
	var c model.RuleChain
	var addInfoBytes []byte
	err := s.pool.QueryRow(ctx, query, tenantID).Scan(
		&c.ID,
		&c.CreatedTime,
		&c.TenantID,
		&c.Name,
		&c.Type,
		&c.IsRoot,
		&c.PublishedRevision,
		&addInfoBytes,
		&c.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("Root Rule Chain not found")
		}
		return nil, fmt.Errorf("postgres_rule_store: get root failed: %w", err)
	}
	if len(addInfoBytes) > 0 {
		c.AdditionalInfo = json.RawMessage(addInfoBytes)
	} else {
		c.AdditionalInfo = json.RawMessage(`{}`)
	}
	return &c, nil
}

func (s *PostgresStore) SaveRuleChain(ctx context.Context, chain *model.RuleChain) error {
	if chain.ID == ids.Nil {
		chain.ID = ids.MustNew()
	}
	if chain.CreatedTime == 0 {
		chain.CreatedTime = time.Now().UnixMilli()
	}
	if chain.PublishedRevision == 0 {
		chain.PublishedRevision = 1
	}
	if chain.Type == "" {
		chain.Type = "CORE"
	}
	if chain.Version == 0 {
		chain.Version = 1
	}

	var exists bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant WHERE id = $1)`, chain.TenantID).Scan(&exists)
	if !exists {
		var fallbackID ids.ID
		if err := s.pool.QueryRow(ctx, `SELECT id FROM tenant LIMIT 1`).Scan(&fallbackID); err == nil {
			chain.TenantID = fallbackID
		}
	}

	addInfo := chain.AdditionalInfo
	if len(addInfo) == 0 {
		addInfo = json.RawMessage(`{}`)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO rule_chain (id, created_time, tenant_id, name, type, is_root, published_revision, additional_info, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			is_root = EXCLUDED.is_root,
			published_revision = EXCLUDED.published_revision,
			additional_info = EXCLUDED.additional_info,
			version = rule_chain.version + 1;
	`
	_, err = tx.Exec(ctx, query,
		chain.ID,
		chain.CreatedTime,
		chain.TenantID,
		chain.Name,
		chain.Type,
		chain.IsRoot,
		chain.PublishedRevision,
		[]byte(addInfo),
		chain.Version,
	)
	if err != nil {
		return fmt.Errorf("postgres_rule_store: save chain failed: %w", err)
	}

	// If this chain was designated as root, unmark any previous root chains for this tenant
	if chain.IsRoot {
		unsetQuery := `UPDATE rule_chain SET is_root = false WHERE tenant_id = $1 AND id != $2;`
		if _, err := tx.Exec(ctx, unsetQuery, chain.TenantID, chain.ID); err != nil {
			return fmt.Errorf("postgres_rule_store: unset root failed: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) SetRootRuleChain(ctx context.Context, tenantID, id ids.ID) error {
	var exists bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant WHERE id = $1)`, tenantID).Scan(&exists)
	if !exists {
		var fallbackID ids.ID
		if err := s.pool.QueryRow(ctx, `SELECT id FROM tenant LIMIT 1`).Scan(&fallbackID); err == nil {
			tenantID = fallbackID
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	unsetQuery := `UPDATE rule_chain SET is_root = false WHERE tenant_id = $1;`
	if _, err := tx.Exec(ctx, unsetQuery, tenantID); err != nil {
		return err
	}

	setQuery := `UPDATE rule_chain SET is_root = true WHERE tenant_id = $1 AND id = $2;`
	tag, err := tx.Exec(ctx, setQuery, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("Rule Chain not found")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) DeleteRuleChain(ctx context.Context, tenantID, id ids.ID) error {
	query := `DELETE FROM rule_chain WHERE tenant_id = $1 AND id = $2;`
	tag, err := s.pool.Exec(ctx, query, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres_rule_store: delete failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("Rule Chain not found")
	}
	return nil
}

func (s *PostgresStore) GetRevision(ctx context.Context, chainID ids.ID, revision int) (*model.RuleChainRevision, error) {
	query := `
		SELECT rule_chain_id, revision, status, definition, author_id, comment, created_time
		FROM rule_chain_revision
		WHERE rule_chain_id = $1 AND revision = $2;
	`
	var rev model.RuleChainRevision
	var defBytes []byte
	var authorID *ids.ID
	var comment *string

	err := s.pool.QueryRow(ctx, query, chainID, revision).Scan(
		&rev.RuleChainID,
		&rev.Revision,
		&rev.Status,
		&defBytes,
		&authorID,
		&comment,
		&rev.CreatedTime,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("Rule Chain revision not found")
		}
		return nil, fmt.Errorf("postgres_rule_store: get revision failed: %w", err)
	}

	if err := json.Unmarshal(defBytes, &rev.Definition); err != nil {
		return nil, fmt.Errorf("postgres_rule_store: unmarshal definition failed: %w", err)
	}
	rev.AuthorID = authorID
	if comment != nil {
		rev.Comment = *comment
	}
	return &rev, nil
}

func (s *PostgresStore) GetLatestRevision(ctx context.Context, chainID ids.ID) (*model.RuleChainRevision, error) {
	query := `
		SELECT rule_chain_id, revision, status, definition, author_id, comment, created_time
		FROM rule_chain_revision
		WHERE rule_chain_id = $1
		ORDER BY revision DESC
		LIMIT 1;
	`
	var rev model.RuleChainRevision
	var defBytes []byte
	var authorID *ids.ID
	var comment *string

	err := s.pool.QueryRow(ctx, query, chainID).Scan(
		&rev.RuleChainID,
		&rev.Revision,
		&rev.Status,
		&defBytes,
		&authorID,
		&comment,
		&rev.CreatedTime,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("No revisions found for Rule Chain")
		}
		return nil, fmt.Errorf("postgres_rule_store: get latest revision failed: %w", err)
	}

	if err := json.Unmarshal(defBytes, &rev.Definition); err != nil {
		return nil, fmt.Errorf("postgres_rule_store: unmarshal definition failed: %w", err)
	}
	rev.AuthorID = authorID
	if comment != nil {
		rev.Comment = *comment
	}
	return &rev, nil
}

func (s *PostgresStore) SaveRevision(ctx context.Context, rev *model.RuleChainRevision) error {
	if rev.CreatedTime == 0 {
		rev.CreatedTime = time.Now().UnixMilli()
	}
	if rev.Status == "" {
		rev.Status = "PUBLISHED"
	}

	defBytes, err := json.Marshal(rev.Definition)
	if err != nil {
		return fmt.Errorf("postgres_rule_store: marshal definition failed: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO rule_chain_revision (rule_chain_id, revision, status, definition, author_id, comment, created_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (rule_chain_id, revision) DO UPDATE SET
			status = EXCLUDED.status,
			definition = EXCLUDED.definition,
			author_id = EXCLUDED.author_id,
			comment = EXCLUDED.comment;
	`
	_, err = tx.Exec(ctx, query,
		rev.RuleChainID,
		rev.Revision,
		rev.Status,
		defBytes,
		rev.AuthorID,
		rev.Comment,
		rev.CreatedTime,
	)
	if err != nil {
		return fmt.Errorf("postgres_rule_store: save revision failed: %w", err)
	}

	// Synchronize published revision count on rule_chain
	updateChain := `
		UPDATE rule_chain
		SET published_revision = GREATEST(published_revision, $2)
		WHERE id = $1;
	`
	if _, err := tx.Exec(ctx, updateChain, rev.RuleChainID, rev.Revision); err != nil {
		return fmt.Errorf("postgres_rule_store: sync published revision failed: %w", err)
	}

	return tx.Commit(ctx)
}
