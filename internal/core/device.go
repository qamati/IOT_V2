package core

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/db"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// DeviceStore abstracts device and device_credentials persistence.
type DeviceStore interface {
	CreateDevice(ctx context.Context, d *model.Device, creds *model.DeviceCredentials) error
	GetDevice(ctx context.Context, tenantID, deviceID ids.ID) (*model.Device, error)
	GetCredentials(ctx context.Context, tenantID, deviceID ids.ID) (*model.DeviceCredentials, error)
	ListDevices(ctx context.Context, tenantID ids.ID, offset, limit int) ([]*model.Device, int, error)
	DeleteDevice(ctx context.Context, tenantID, deviceID ids.ID) error
	FindCredentialsByLookupID(ctx context.Context, lookupID []byte) (*model.DeviceCredentials, error)
	GetOrCreateDefaultProfile(ctx context.Context, tenantID ids.ID) (*model.DeviceProfile, error)
}

// MemoryDeviceStore provides an in-memory device repository.
type MemoryDeviceStore struct {
	mu          sync.RWMutex
	devices     map[ids.ID]*model.Device
	credentials map[ids.ID]*model.DeviceCredentials // device_id -> creds
	byLookup    map[string]*model.DeviceCredentials  // hex(lookup_id) -> creds
	profiles    map[ids.ID]*model.DeviceProfile      // tenant_id -> default profile
}

func NewMemoryDeviceStore() *MemoryDeviceStore {
	return &MemoryDeviceStore{
		devices:     make(map[ids.ID]*model.Device),
		credentials: make(map[ids.ID]*model.DeviceCredentials),
		byLookup:    make(map[string]*model.DeviceCredentials),
		profiles:    make(map[ids.ID]*model.DeviceProfile),
	}
}

func (m *MemoryDeviceStore) GetOrCreateDefaultProfile(ctx context.Context, tenantID ids.ID) (*model.DeviceProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if prof, ok := m.profiles[tenantID]; ok {
		return prof, nil
	}

	prof := &model.DeviceProfile{
		ID:            ids.MustNew(),
		CreatedTime:   time.Now().UnixMilli(),
		TenantID:      tenantID,
		Name:          "default",
		TransportType: "DEFAULT",
		IsDefault:     true,
		ProfileData:   []byte(`{}`),
		Version:       1,
	}
	m.profiles[tenantID] = prof
	return prof, nil
}

func (m *MemoryDeviceStore) CreateDevice(ctx context.Context, d *model.Device, creds *model.DeviceCredentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check name uniqueness per tenant
	for _, existing := range m.devices {
		if existing.TenantID == d.TenantID && existing.Name == d.Name {
			return errs.Conflict("device with this name already exists in tenant")
		}
	}

	m.devices[d.ID] = d
	if creds != nil {
		m.credentials[d.ID] = creds
		m.byLookup[hex.EncodeToString(creds.LookupID)] = creds
	}
	return nil
}

func (m *MemoryDeviceStore) GetDevice(ctx context.Context, tenantID, deviceID ids.ID) (*model.Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.devices[deviceID]
	if !ok || d.TenantID != tenantID {
		return nil, errs.NotFound("device not found")
	}
	return d, nil
}

func (m *MemoryDeviceStore) GetCredentials(ctx context.Context, tenantID, deviceID ids.ID) (*model.DeviceCredentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.devices[deviceID]
	if !ok || d.TenantID != tenantID {
		return nil, errs.NotFound("device not found")
	}

	c, ok := m.credentials[deviceID]
	if !ok {
		return nil, errs.NotFound("credentials not found")
	}
	return c, nil
}

func (m *MemoryDeviceStore) ListDevices(ctx context.Context, tenantID ids.ID, offset, limit int) ([]*model.Device, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matching []*model.Device
	for _, d := range m.devices {
		if d.TenantID == tenantID {
			matching = append(matching, d)
		}
	}

	total := len(matching)
	if offset > total {
		return []*model.Device{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matching[offset:end], total, nil
}

func (m *MemoryDeviceStore) DeleteDevice(ctx context.Context, tenantID, deviceID ids.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.devices[deviceID]
	if !ok || d.TenantID != tenantID {
		return errs.NotFound("device not found")
	}

	delete(m.devices, deviceID)
	if c, ok := m.credentials[deviceID]; ok {
		delete(m.byLookup, hex.EncodeToString(c.LookupID))
		delete(m.credentials, deviceID)
	}
	return nil
}

func (m *MemoryDeviceStore) FindCredentialsByLookupID(ctx context.Context, lookupID []byte) (*model.DeviceCredentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.byLookup[hex.EncodeToString(lookupID)]
	if !ok {
		return nil, errs.NotFound("credentials not found")
	}
	return c, nil
}

// PostgresDeviceStore implements DeviceStore on top of PostgreSQL / TimescaleDB.
type PostgresDeviceStore struct {
	pool *pgxpool.Pool
}

func NewPostgresDeviceStore(pool *pgxpool.Pool) *PostgresDeviceStore {
	return &PostgresDeviceStore{pool: pool}
}

func (p *PostgresDeviceStore) GetOrCreateDefaultProfile(ctx context.Context, tenantID ids.ID) (*model.DeviceProfile, error) {
	var prof model.DeviceProfile
	row := p.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, name, transport_type, is_default, profile_data, version
		FROM device_profile
		WHERE tenant_id = $1 AND is_default = true
		LIMIT 1
	`, tenantID)

	err := row.Scan(&prof.ID, &prof.CreatedTime, &prof.TenantID, &prof.Name, &prof.TransportType, &prof.IsDefault, &prof.ProfileData, &prof.Version)
	if err == nil {
		return &prof, nil
	}

	// Create default device profile
	prof = model.DeviceProfile{
		ID:            ids.MustNew(),
		CreatedTime:   time.Now().UnixMilli(),
		TenantID:      tenantID,
		Name:          "default",
		TransportType: "DEFAULT",
		IsDefault:     true,
		ProfileData:   []byte(`{}`),
		Version:       1,
	}

	row = p.pool.QueryRow(ctx, `
		INSERT INTO device_profile (id, created_time, tenant_id, name, transport_type, is_default, profile_data, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, name) DO UPDATE SET is_default = EXCLUDED.is_default
		RETURNING id, created_time, transport_type, is_default, profile_data, version
	`, prof.ID, prof.CreatedTime, prof.TenantID, prof.Name, prof.TransportType, prof.IsDefault, prof.ProfileData, prof.Version)
	if err := row.Scan(&prof.ID, &prof.CreatedTime, &prof.TransportType, &prof.IsDefault, &prof.ProfileData, &prof.Version); err != nil {
		return nil, err
	}
	return &prof, nil
}

func (p *PostgresDeviceStore) CreateDevice(ctx context.Context, d *model.Device, creds *model.DeviceCredentials) error {
	return db.InTenantTx(ctx, p.pool, d.TenantID.String(), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO device (id, created_time, tenant_id, device_profile_id, name, label, is_gateway, tags, version)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, d.ID, d.CreatedTime, d.TenantID, d.DeviceProfileID, d.Name, d.Label, d.IsGateway, d.Tags, d.Version)
		if err != nil {
			if strings.Contains(err.Error(), "device_tenant_id_name_key") || strings.Contains(err.Error(), "23505") {
				return errs.Conflict(fmt.Sprintf("A device named '%s' already exists for this tenant. Please use a unique device name or update the existing device.", d.Name))
			}
			return fmt.Errorf("failed to insert device: %w", err)
		}

		if creds != nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO device_credentials (id, created_time, tenant_id, device_id, credentials_type, lookup_id, credentials_enc)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, creds.ID, creds.CreatedTime, creds.TenantID, creds.DeviceID, creds.CredentialsType, creds.LookupID, []byte(creds.CredentialsValue))
			if err != nil {
				if strings.Contains(err.Error(), "device_credentials_lookup_id_key") || strings.Contains(err.Error(), "23505") {
					return errs.Conflict("The specified access token is already in use by another device.")
				}
				return fmt.Errorf("failed to insert device credentials: %w", err)
			}
		}
		return nil
	})
}

func (p *PostgresDeviceStore) GetDevice(ctx context.Context, tenantID, deviceID ids.ID) (*model.Device, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, device_profile_id, name, label, is_gateway, tags, version
		FROM device
		WHERE id = $1 AND tenant_id = $2
	`, deviceID, tenantID)

	var d model.Device
	err := row.Scan(&d.ID, &d.CreatedTime, &d.TenantID, &d.DeviceProfileID, &d.Name, &d.Label, &d.IsGateway, &d.Tags, &d.Version)
	if err != nil {
		return nil, errs.NotFound("device not found")
	}
	return &d, nil
}

func (p *PostgresDeviceStore) GetCredentials(ctx context.Context, tenantID, deviceID ids.ID) (*model.DeviceCredentials, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, device_id, credentials_type, lookup_id, credentials_enc
		FROM device_credentials
		WHERE device_id = $1 AND tenant_id = $2
	`, deviceID, tenantID)

	var c model.DeviceCredentials
	var val []byte
	err := row.Scan(&c.ID, &c.CreatedTime, &c.TenantID, &c.DeviceID, &c.CredentialsType, &c.LookupID, &val)
	if err != nil {
		return nil, errs.NotFound("credentials not found")
	}
	c.CredentialsValue = string(val)
	return &c, nil
}

func (p *PostgresDeviceStore) ListDevices(ctx context.Context, tenantID ids.ID, offset, limit int) ([]*model.Device, int, error) {
	var total int
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM device WHERE tenant_id = $1`, tenantID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := p.pool.Query(ctx, `
		SELECT id, created_time, tenant_id, device_profile_id, name, label, is_gateway, tags, version
		FROM device
		WHERE tenant_id = $1
		ORDER BY created_time DESC
		OFFSET $2 LIMIT $3
	`, tenantID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.Device
	for rows.Next() {
		var d model.Device
		if err := rows.Scan(&d.ID, &d.CreatedTime, &d.TenantID, &d.DeviceProfileID, &d.Name, &d.Label, &d.IsGateway, &d.Tags, &d.Version); err != nil {
			return nil, 0, err
		}
		list = append(list, &d)
	}
	return list, total, nil
}

func (p *PostgresDeviceStore) DeleteDevice(ctx context.Context, tenantID, deviceID ids.ID) error {
	res, err := p.pool.Exec(ctx, `DELETE FROM device WHERE id = $1 AND tenant_id = $2`, deviceID, tenantID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errs.NotFound("device not found")
	}
	return nil
}

func (p *PostgresDeviceStore) FindCredentialsByLookupID(ctx context.Context, lookupID []byte) (*model.DeviceCredentials, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, created_time, tenant_id, device_id, credentials_type, lookup_id, credentials_enc
		FROM device_credentials
		WHERE lookup_id = $1
	`, lookupID)

	var c model.DeviceCredentials
	var val []byte
	err := row.Scan(&c.ID, &c.CreatedTime, &c.TenantID, &c.DeviceID, &c.CredentialsType, &c.LookupID, &val)
	if err != nil {
		return nil, errs.NotFound("credentials not found")
	}
	c.CredentialsValue = string(val)
	return &c, nil
}

// DeviceManager handles high-level device logic and credential generation.
type DeviceManager struct {
	store  DeviceStore
	pepper []byte
}

func NewDeviceManager(store DeviceStore, pepper []byte) *DeviceManager {
	if len(pepper) == 0 {
		pepper = []byte("iotp-default-token-pepper-secret")
	}
	return &DeviceManager{store: store, pepper: pepper}
}

// ComputeLookupID computes HMAC-SHA256(pepper, token) for O(1) constant-time credential lookup.
func (m *DeviceManager) ComputeLookupID(token string) []byte {
	h := hmac.New(sha256.New, m.pepper)
	h.Write([]byte(token))
	return h.Sum(nil)
}

// GenerateDeviceToken generates a cryptographically secure random token string.
func GenerateDeviceToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateDevice provisions a new device with credentials.
func (m *DeviceManager) CreateDevice(ctx context.Context, tenantID ids.ID, name, label string, customToken string) (*model.Device, *model.DeviceCredentials, error) {
	if name == "" {
		return nil, nil, errs.InvalidField("name", "device name cannot be empty")
	}

	prof, err := m.store.GetOrCreateDefaultProfile(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}

	token := customToken
	if token == "" {
		token = GenerateDeviceToken()
	}

	lookupID := m.ComputeLookupID(token)
	now := time.Now().UnixMilli()
	deviceID := ids.MustNew()

	device := &model.Device{
		ID:              deviceID,
		CreatedTime:     now,
		TenantID:        tenantID,
		DeviceProfileID: prof.ID,
		Name:            name,
		Label:           label,
		Tags:            []string{},
		Version:         1,
	}

	creds := &model.DeviceCredentials{
		ID:               ids.MustNew(),
		CreatedTime:      now,
		TenantID:         tenantID,
		DeviceID:         deviceID,
		CredentialsType:  model.CredentialsTypeAccessToken,
		LookupID:         lookupID,
		CredentialsValue: token,
	}

	if err := m.store.CreateDevice(ctx, device, creds); err != nil {
		return nil, nil, err
	}

	return device, creds, nil
}

func (m *DeviceManager) GetDevice(ctx context.Context, tenantID, deviceID ids.ID) (*model.Device, error) {
	return m.store.GetDevice(ctx, tenantID, deviceID)
}

func (m *DeviceManager) GetCredentials(ctx context.Context, tenantID, deviceID ids.ID) (*model.DeviceCredentials, error) {
	return m.store.GetCredentials(ctx, tenantID, deviceID)
}

func (m *DeviceManager) ListDevices(ctx context.Context, tenantID ids.ID, page, pageSize int) ([]*model.Device, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 0 {
		page = 0
	}
	offset := page * pageSize
	return m.store.ListDevices(ctx, tenantID, offset, pageSize)
}

func (m *DeviceManager) DeleteDevice(ctx context.Context, tenantID, deviceID ids.ID) error {
	return m.store.DeleteDevice(ctx, tenantID, deviceID)
}

func (m *DeviceManager) AuthenticateToken(ctx context.Context, token string) (*model.DeviceCredentials, error) {
	lookupID := m.ComputeLookupID(token)
	return m.store.FindCredentialsByLookupID(ctx, lookupID)
}
