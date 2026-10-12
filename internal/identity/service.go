package identity

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

type UserStore interface {
	FindByEmail(ctx context.Context, email string) (*model.User, string, error) // returns user and passwordHash
	Save(ctx context.Context, user *model.User, passwordHash string) error
	FindByID(ctx context.Context, id ids.ID) (*model.User, error)
}

// MemoryUserStore provides an in-memory user store for fast testing and lite standalone mode.
type MemoryUserStore struct {
	mu     sync.RWMutex
	users  map[string]*model.User // email -> user
	hashes map[string]string      // email -> hash
}

func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users:  make(map[string]*model.User),
		hashes: make(map[string]string),
	}
}

func (m *MemoryUserStore) FindByEmail(ctx context.Context, email string) (*model.User, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[email]
	if !ok {
		return nil, "", errs.NotFound("user not found")
	}
	return u, m.hashes[email], nil
}

func (m *MemoryUserStore) Save(ctx context.Context, user *model.User, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.users[user.Email] = user
	m.hashes[user.Email] = passwordHash
	return nil
}

func (m *MemoryUserStore) FindByID(ctx context.Context, id ids.ID) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, errs.NotFound("user not found")
}

// PostgresUserStore implements UserStore using pgxpool.
type PostgresUserStore struct {
	pool *pgxpool.Pool
}

func NewPostgresUserStore(pool *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{pool: pool}
}

func (p *PostgresUserStore) FindByEmail(ctx context.Context, email string) (*model.User, string, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT u.id, u.created_time, u.tenant_id, u.authority, u.email, u.first_name, u.last_name, u.version,
		       c.password_hash
		FROM users u
		JOIN user_credentials c ON c.user_id = u.id
		WHERE u.email = $1 AND c.enabled = true
	`, email)

	var u model.User
	var hash *string
	err := row.Scan(&u.ID, &u.CreatedTime, &u.TenantID, &u.Authority, &u.Email, &u.FirstName, &u.LastName, &u.Version, &hash)
	if err != nil {
		return nil, "", errs.Wrap(errs.CodeNotFound, "user not found", err)
	}

	hashStr := ""
	if hash != nil {
		hashStr = *hash
	}
	return &u, hashStr, nil
}

func (p *PostgresUserStore) Save(ctx context.Context, user *model.User, passwordHash string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var actualID ids.ID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (id, created_time, tenant_id, authority, email, first_name, last_name, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (email) DO UPDATE SET
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			version = users.version + 1
		RETURNING id
	`, user.ID, user.CreatedTime, user.TenantID, user.Authority, user.Email, user.FirstName, user.LastName, user.Version).Scan(&actualID)
	if err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}
	user.ID = actualID

	_, err = tx.Exec(ctx, `
		INSERT INTO user_credentials (user_id, password_hash, enabled)
		VALUES ($1, $2, true)
		ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash, enabled = true
	`, actualID, passwordHash)
	if err != nil {
		return fmt.Errorf("failed to save user credentials: %w", err)
	}

	return tx.Commit(ctx)
}

func (p *PostgresUserStore) FindByID(ctx context.Context, id ids.ID) (*model.User, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, authority, email, first_name, last_name, version
		FROM users WHERE id = $1
	`, id)

	var u model.User
	err := row.Scan(&u.ID, &u.CreatedTime, &u.TenantID, &u.Authority, &u.Email, &u.FirstName, &u.LastName, &u.Version)
	if err != nil {
		return nil, errs.Wrap(errs.CodeNotFound, "user not found", err)
	}
	return &u, nil
}

// Service manages user authentication and authorization.
type Service struct {
	store     UserStore
	jwtSecret []byte
	tokenTTL  time.Duration
}

// NewService creates a new identity Service.
func NewService(store UserStore, jwtSecret []byte, tokenTTL time.Duration) *Service {
	if tokenTTL == 0 {
		tokenTTL = 24 * time.Hour
	}
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("default-dev-insecure-secret-32-bytes-long")
	}
	return &Service{
		store:     store,
		jwtSecret: jwtSecret,
		tokenTTL:  tokenTTL,
	}
}

// Login verifies user credentials and returns a signed JWT access token.
func (s *Service) Login(ctx context.Context, email, password string) (string, *model.User, error) {
	user, hash, err := s.store.FindByEmail(ctx, email)
	if err != nil {
		return "", nil, errs.Unauthorized("invalid email or password")
	}

	if !VerifyPassword(password, hash) {
		return "", nil, errs.Unauthorized("invalid email or password")
	}

	token, err := GenerateToken(user, s.jwtSecret, s.tokenTTL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, user, nil
}

// Register creates a new user with Argon2id hashed password.
func (s *Service) Register(ctx context.Context, user *model.User, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	return s.store.Save(ctx, user, hash)
}

// ValidateAccessToken parses and validates a token string.
func (s *Service) ValidateAccessToken(token string) (*Claims, error) {
	return ValidateToken(token, s.jwtSecret)
}
