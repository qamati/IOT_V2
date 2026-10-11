package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const (
	tenantContextKey contextKey = "iotp.db.tenant_id"
)

// Config holds Postgres / TimescaleDB connection pool configuration.
type Config struct {
	Host            string        `koanf:"host"`
	Port            int           `koanf:"port"`
	Database        string        `koanf:"database"`
	User            string        `koanf:"user"`
	Password        string        `koanf:"password"`
	SSLMode         string        `koanf:"ssl_mode"`
	MaxConns        int32         `koanf:"max_conns"`
	MinConns        int32         `koanf:"min_conns"`
	MaxConnLifetime time.Duration `koanf:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `koanf:"max_conn_idle_time"`
}

// ConnString generates a PostgreSQL connection string.
func (c Config) ConnString() string {
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 5432
	}
	if c.Database == "" {
		c.Database = "iotp"
	}
	if c.User == "" {
		c.User = "postgres"
	}
	if c.SSLMode == "" {
		c.SSLMode = "disable"
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode)
}

// NewPool initializes a pgxpool connection pool.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	connStr := cfg.ConnString()
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("db: failed to parse connection string: %w", err)
	}

	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	} else {
		poolConfig.MaxConns = 25
	}
	if cfg.MinConns > 0 {
		poolConfig.MinConns = cfg.MinConns
	} else {
		poolConfig.MinConns = 2
	}
	if cfg.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	} else {
		poolConfig.MaxConnLifetime = 30 * time.Minute
	}
	if cfg.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	} else {
		poolConfig.MaxConnIdleTime = 5 * time.Minute
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("db: failed to create connection pool: %w", err)
	}

	return pool, nil
}

// WithTenant sets the tenant ID on context for database queries.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenantID)
}

// TenantID retrieves the tenant ID from context.
func TenantID(ctx context.Context) string {
	if val, ok := ctx.Value(tenantContextKey).(string); ok {
		return val
	}
	return ""
}

// InTx executes the callback function inside a database transaction.
// If a tenant_id is set on the context, it sets the session GUC `app.tenant_id`
// so that Row Level Security (RLS) policies are automatically enforced.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tenantID := TenantID(ctx)
	if tenantID != "" {
		// Set transaction-local GUC for Postgres RLS: SET LOCAL app.tenant_id = '...'
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
			return fmt.Errorf("db: failed to set tenant session GUC: %w", err)
		}
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: failed to commit transaction: %w", err)
	}

	return nil
}

// InTenantTx is a convenience helper that binds tenantID into context and executes within a transaction.
func InTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return InTx(WithTenant(ctx, tenantID), pool, fn)
}
