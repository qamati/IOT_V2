package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// API encapsulates the REST API router and handlers.
type API struct {
	router     chi.Router
	identity   *identity.Service
	devManager *core.DeviceManager
	telemetry  telemetry.Store
	attributes telemetry.AttributeStore
	tracker    devstate.Tracker
	alarms     alarm.Service
	bus        bus.Bus
}

func NewAPI(idSvc *identity.Service, devMgr *core.DeviceManager, tsStore telemetry.Store, b bus.Bus) *API {
	a := &API{
		router:     chi.NewRouter(),
		identity:   idSvc,
		devManager: devMgr,
		telemetry:  tsStore,
		attributes: telemetry.NewMemoryAttributeStore(),
		tracker:    devstate.NewMemoryTracker(60 * time.Second),
		alarms:     alarm.NewMemoryService(),
		bus:        b,
	}

	a.setupMiddleware()
	a.setupRoutes()
	return a
}

func (a *API) WithAttributes(attrs telemetry.AttributeStore) *API {
	a.attributes = attrs
	return a
}

func (a *API) WithTracker(t devstate.Tracker) *API {
	a.tracker = t
	return a
}

func (a *API) WithAlarms(al alarm.Service) *API {
	a.alarms = al
	return a
}

func (a *API) Router() chi.Router {
	return a.router
}

func (a *API) setupMiddleware() {
	a.router.Use(middleware.RequestID)
	a.router.Use(middleware.RealIP)
	a.router.Use(middleware.Recoverer)
	a.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (a *API) setupRoutes() {
	a.router.Route("/api/v1", func(r chi.Router) {
		// Public Auth
		r.Post("/auth/login", a.handleLogin)

		// Device wire-compatible HTTP endpoints:
		// 1. Telemetry upload: POST /api/v1/{token}/telemetry
		r.Post("/{token}/telemetry", a.handleDeviceTelemetryUpload)
		// 2. Client attributes upload: POST /api/v1/{token}/attributes
		r.Post("/{token}/attributes", a.handleDeviceAttributesUpload)
		// 3. Attributes query: GET /api/v1/{token}/attributes
		r.Get("/{token}/attributes", a.handleDeviceAttributesGet)

		// Protected management endpoints
		r.Group(func(protected chi.Router) {
			protected.Use(a.authMiddleware)

			// Device CRUD & Activity
			protected.Post("/devices", a.handleCreateDevice)
			protected.Get("/devices", a.handleListDevices)
			protected.Get("/devices/{id}", a.handleGetDevice)
			protected.Get("/devices/{id}/credentials", a.handleGetDeviceCredentials)
			protected.Get("/devices/{id}/activity", a.handleGetDeviceActivity)
			protected.Delete("/devices/{id}", a.handleDeleteDevice)

			// Telemetry queries
			protected.Get("/telemetry/DEVICE/{id}/values/latest", a.handleGetLatestTelemetry)
			protected.Get("/telemetry/DEVICE/{id}/values/timeseries", a.handleGetTimeseriesTelemetry)

			// Attributes management
			protected.Get("/plugins/telemetry/DEVICE/{id}/values/attributes", a.handleGetAllAttributes)
			protected.Get("/plugins/telemetry/DEVICE/{id}/values/attributes/{scope}", a.handleGetScopedAttributes)
			protected.Post("/plugins/telemetry/DEVICE/{id}/values/attributes/{scope}", a.handleSaveScopedAttributes)
			protected.Delete("/plugins/telemetry/DEVICE/{id}/values/attributes/{scope}", a.handleDeleteScopedAttributes)

			// Alarms management
			protected.Get("/alarms", a.handleListAlarms)
			protected.Post("/alarms/{id}/ack", a.handleAckAlarm)
			protected.Post("/alarms/{id}/clear", a.handleClearAlarm)
		})
	})
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			errs.WriteHTTP(w, errs.Unauthorized("missing or invalid authorization header"), middleware.GetReqID(r.Context()), r.URL.Path)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := a.identity.ValidateAccessToken(token)
		if err != nil {
			errs.WriteHTTP(w, errs.Unauthorized("invalid access token"), middleware.GetReqID(r.Context()), r.URL.Path)
			return
		}

		ctx := identity.WithUser(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// 1. POST /api/v1/auth/login
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid request body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	token, user, err := a.identity.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token": token,
		"user":  user,
	})
}

// 2. POST /api/v1/devices
type createDeviceRequest struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Token string `json:"token"`
}

func (a *API) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID in claims", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req createDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid request body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	dev, creds, err := a.devManager.CreateDevice(r.Context(), tenantID, req.Name, req.Label, req.Token)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"device":      dev,
		"credentials": creds,
	})
}

// 3. GET /api/v1/devices
func (a *API) handleListDevices(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	devices, total, err := a.devManager.ListDevices(r.Context(), tenantID, page, pageSize)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":     devices,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// 4. GET /api/v1/devices/{id}
func (a *API) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	dev, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dev)
}

// 5. GET /api/v1/devices/{id}/credentials
func (a *API) handleGetDeviceCredentials(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	creds, err := a.devManager.GetCredentials(r.Context(), tenantID, deviceID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(creds)
}

// 6. DELETE /api/v1/devices/{id}
func (a *API) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.devManager.DeleteDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "DELETED"})
}

// 7. POST /api/v1/{token}/telemetry
func (a *API) handleDeviceTelemetryUpload(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		errs.WriteHTTP(w, errs.Unauthorized("missing device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	creds, err := a.devManager.AuthenticateToken(r.Context(), token)
	if err != nil {
		errs.WriteHTTP(w, errs.Unauthorized("invalid device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("failed to read body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	msg := bus.NewMessage(creds.TenantID.String(), "DEVICE", creds.DeviceID.String(), bus.TypeTelemetryUp, body)
	if err := a.bus.Publish(r.Context(), "telemetry.up", msg); err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to enqueue telemetry", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	a.tracker.RecordActivity(r.Context(), creds.DeviceID, time.Now().UnixMilli())
	w.WriteHeader(http.StatusOK)
}

// 7b. POST /api/v1/{token}/attributes (Device uploads client attributes)
func (a *API) handleDeviceAttributesUpload(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		errs.WriteHTTP(w, errs.Unauthorized("missing device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	creds, err := a.devManager.AuthenticateToken(r.Context(), token)
	if err != nil {
		errs.WriteHTTP(w, errs.Unauthorized("invalid device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var attrs map[string]any
	if err := json.NewDecoder(r.Body).Decode(&attrs); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid json body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.attributes.SaveAttributes(r.Context(), creds.DeviceID, model.ScopeClient, attrs); err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to save attributes", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	a.tracker.RecordActivity(r.Context(), creds.DeviceID, time.Now().UnixMilli())
	w.WriteHeader(http.StatusOK)
}

// 7c. GET /api/v1/{token}/attributes (Device requests client or shared attributes)
func (a *API) handleDeviceAttributesGet(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	creds, err := a.devManager.AuthenticateToken(r.Context(), token)
	if err != nil {
		errs.WriteHTTP(w, errs.Unauthorized("invalid device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var clientKeys, sharedKeys []string
	if k := r.URL.Query().Get("clientKeys"); k != "" {
		clientKeys = strings.Split(k, ",")
	}
	if k := r.URL.Query().Get("sharedKeys"); k != "" {
		sharedKeys = strings.Split(k, ",")
	}

	clientAttrs, _ := a.attributes.GetAttributes(r.Context(), creds.DeviceID, model.ScopeClient, clientKeys)
	sharedAttrs, _ := a.attributes.GetAttributes(r.Context(), creds.DeviceID, model.ScopeShared, sharedKeys)

	resp := map[string]map[string]any{
		"client": make(map[string]any),
		"shared": make(map[string]any),
	}
	for _, kv := range clientAttrs {
		resp["client"][kv.Key] = kv.Val
	}
	for _, kv := range sharedAttrs {
		resp["shared"][kv.Key] = kv.Val
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GET /api/v1/devices/{id}/activity
func (a *API) handleGetDeviceActivity(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if _, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	activity := a.tracker.GetActivity(r.Context(), deviceID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(activity)
}

// 8. GET /api/v1/telemetry/DEVICE/{id}/values/latest
func (a *API) handleGetLatestTelemetry(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	// Verify device belongs to caller's tenant
	if _, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var keys []string
	if k := r.URL.Query().Get("keys"); k != "" {
		keys = strings.Split(k, ",")
	}

	latest, err := a.telemetry.GetLatest(r.Context(), deviceID, keys)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to query latest telemetry", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	// Format response as { "key": [{ "ts": 1234, "value": 22.5 }] } matching ThingsBoard convention
	resp := make(map[string][]map[string]any)
	for k, pt := range latest {
		resp[k] = []map[string]any{
			{"ts": pt.Timestamp, "value": pt.Value()},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// 9. GET /api/v1/telemetry/DEVICE/{id}/values/timeseries
func (a *API) handleGetTimeseriesTelemetry(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if _, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var keys []string
	if k := r.URL.Query().Get("keys"); k != "" {
		keys = strings.Split(k, ",")
	}

	startTs, _ := strconv.ParseInt(r.URL.Query().Get("startTs"), 10, 64)
	endTs, _ := strconv.ParseInt(r.URL.Query().Get("endTs"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	timeseries, err := a.telemetry.GetTimeseries(r.Context(), deviceID, keys, startTs, endTs, limit)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to query timeseries", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	resp := make(map[string][]map[string]any)
	for k, pts := range timeseries {
		var list []map[string]any
		for _, pt := range pts {
			list = append(list, map[string]any{
				"ts":    pt.Timestamp,
				"value": pt.Value(),
			})
		}
		resp[k] = list
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// 10. GET /api/v1/plugins/telemetry/DEVICE/{id}/values/attributes
func (a *API) handleGetAllAttributes(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if _, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	attrs, err := a.attributes.GetAllAttributes(r.Context(), deviceID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to fetch attributes", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(attrs)
}

// 11. GET /api/v1/plugins/telemetry/DEVICE/{id}/values/attributes/{scope}
func (a *API) handleGetScopedAttributes(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if _, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	scope := model.AttributeScope(chi.URLParam(r, "scope"))
	var keys []string
	if k := r.URL.Query().Get("keys"); k != "" {
		keys = strings.Split(k, ",")
	}

	attrs, err := a.attributes.GetAttributes(r.Context(), deviceID, scope, keys)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to fetch scoped attributes", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(attrs)
}

// 12. POST /api/v1/plugins/telemetry/DEVICE/{id}/values/attributes/{scope}
func (a *API) handleSaveScopedAttributes(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if _, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	scope := model.AttributeScope(chi.URLParam(r, "scope"))
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid json body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.attributes.SaveAttributes(r.Context(), deviceID, scope, body); err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to save attributes", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// 13. DELETE /api/v1/plugins/telemetry/DEVICE/{id}/values/attributes/{scope}
func (a *API) handleDeleteScopedAttributes(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if _, err := a.devManager.GetDevice(r.Context(), tenantID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	scope := model.AttributeScope(chi.URLParam(r, "scope"))
	var keys []string
	if k := r.URL.Query().Get("keys"); k != "" {
		keys = strings.Split(k, ",")
	}

	if err := a.attributes.DeleteAttributes(r.Context(), deviceID, scope, keys); err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to delete attributes", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// 14. GET /api/v1/alarms
func (a *API) handleListAlarms(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	var origID *ids.ID
	if orig := r.URL.Query().Get("originatorId"); orig != "" {
		if parsed, err := ids.Parse(orig); err == nil {
			origID = &parsed
		}
	}

	alarms, total, err := a.alarms.ListAlarms(r.Context(), tenantID, origID, page, pageSize)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to query alarms", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":     alarms,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// 15. POST /api/v1/alarms/{id}/ack
func (a *API) handleAckAlarm(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	alarmID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid alarm id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	alarm, err := a.alarms.AckAlarm(r.Context(), tenantID, alarmID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alarm)
}

// 16. POST /api/v1/alarms/{id}/clear
func (a *API) handleClearAlarm(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	alarmID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid alarm id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	alarm, err := a.alarms.ClearAlarm(r.Context(), tenantID, alarmID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alarm)
}
