package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/api"
	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/bus/mem"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestApp(t *testing.T) (*api.API, *identity.Service, *core.DeviceManager, telemetry.Store, *mem.MemoryBus, ids.ID, ids.ID) {
	t.Helper()

	userStore := identity.NewMemoryUserStore()
	idSvc := identity.NewService(userStore, []byte("super-secure-test-jwt-secret-32b"), 1*time.Hour)

	devStore := core.NewMemoryDeviceStore()
	devMgr := core.NewDeviceManager(devStore, []byte("pepper-123"))

	tsStore := telemetry.NewMemoryStore()
	b := mem.New(4)

	// Direct-save worker
	worker := telemetry.NewDirectSaveWorker(b, tsStore, nil)
	require.NoError(t, worker.Start(context.Background()))
	t.Cleanup(func() {
		_ = worker.Stop()
		_ = b.Close()
	})

	apiSvc := api.NewAPI(idSvc, devMgr, tsStore, b)

	tenantA := ids.MustNew()
	tenantB := ids.MustNew()

	// Seed Tenant A Admin
	adminA := &model.User{
		ID:          ids.MustNew(),
		CreatedTime: time.Now().UnixMilli(),
		TenantID:    &tenantA,
		Authority:   model.AuthorityTenantAdmin,
		Email:       "admin@tenant-a.com",
		Version:     1,
	}
	require.NoError(t, idSvc.Register(context.Background(), adminA, "passwordA"))

	// Seed Tenant B Admin
	adminB := &model.User{
		ID:          ids.MustNew(),
		CreatedTime: time.Now().UnixMilli(),
		TenantID:    &tenantB,
		Authority:   model.AuthorityTenantAdmin,
		Email:       "admin@tenant-b.com",
		Version:     1,
	}
	require.NoError(t, idSvc.Register(context.Background(), adminB, "passwordB"))

	return apiSvc, idSvc, devMgr, tsStore, b, tenantA, tenantB
}

func TestEndToEndWalkingSkeleton(t *testing.T) {
	apiSvc, _, _, _, _, _, _ := setupTestApp(t)

	// 1. Login as Tenant A Admin
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "admin@tenant-a.com",
		"password": "passwordA",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &loginResp))
	tokenA := loginResp["token"].(string)
	assert.NotEmpty(t, tokenA)

	// 2. Create Device under Tenant A
	createBody, _ := json.Marshal(map[string]string{
		"name":  "TempSensor-01",
		"label": "Warehouse Sensor",
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var devResp struct {
		Device      model.Device      `json:"device"`
		Credentials model.DeviceCredentials `json:"credentials"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &devResp))
	deviceID := devResp.Device.ID.String()
	deviceToken := devResp.Credentials.CredentialsValue
	assert.NotEmpty(t, deviceID)
	assert.NotEmpty(t, deviceToken)

	// 3. Upload device telemetry via wire-compatible HTTP endpoint
	telemetryBody := []byte(`{"temperature": 23.4, "humidity": 55}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/"+deviceToken+"/telemetry", bytes.NewReader(telemetryBody))
	apiSvc.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Allow bus worker moment to process
	time.Sleep(50 * time.Millisecond)

	// 4. Query Latest Telemetry with Tenant A token
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/DEVICE/"+deviceID+"/values/latest", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var latestResp map[string][]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &latestResp))
	require.Contains(t, latestResp, "temperature")
	assert.InDelta(t, 23.4, latestResp["temperature"][0]["value"].(float64), 0.001)

	// 5. Query Timeseries Telemetry
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/DEVICE/"+deviceID+"/values/timeseries?keys=temperature", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var tsResp map[string][]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tsResp))
	require.Contains(t, tsResp, "temperature")
	assert.Len(t, tsResp["temperature"], 1)

	// 5b. Query Device State & Summary
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+deviceID+"/state", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var stResp model.DeviceState
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stResp))
	assert.True(t, stResp.Active)
	assert.Greater(t, stResp.LastActivityTime, int64(0))

	// Query State Summary
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/devstate/summary", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var sumResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sumResp))
	assert.Equal(t, float64(1), sumResp["total"])
	assert.Equal(t, float64(1), sumResp["active"])

	// Override Device State to Inactive
	overrideBody, _ := json.Marshal(map[string]bool{"active": false})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+deviceID+"/state/override", bytes.NewReader(overrideBody))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+deviceID+"/state", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stResp))
	assert.False(t, stResp.Active)

	// 6. Cross-Tenant Isolation: Login as Tenant B
	loginBBody, _ := json.Marshal(map[string]string{
		"email":    "admin@tenant-b.com",
		"password": "passwordB",
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBBody))
	apiSvc.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginBResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &loginBResp))
	tokenB := loginBResp["token"].(string)

	// Tenant B attempting to access Tenant A's device gets 404 Not Found
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+deviceID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	apiSvc.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Tenant B attempting to query Tenant A's telemetry gets 404 Not Found
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/DEVICE/"+deviceID+"/values/latest", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	apiSvc.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// 7. Invalid device token rejected
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/bad-device-token/telemetry", bytes.NewReader(telemetryBody))
	apiSvc.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
