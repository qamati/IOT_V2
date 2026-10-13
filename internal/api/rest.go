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
	"github.com/qamati/IOT_V2/internal/dashboard"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/edge"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/integration"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/notification"
	"github.com/qamati/IOT_V2/internal/ota"
	"github.com/qamati/IOT_V2/internal/realtime"
	"github.com/qamati/IOT_V2/internal/rpc"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/internal/scada"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// API encapsulates the REST API router and handlers.
type API struct {
	router        chi.Router
	identity      *identity.Service
	devManager    *core.DeviceManager
	telemetry     telemetry.Store
	attributes    telemetry.AttributeStore
	tracker       devstate.Tracker
	alarms        alarm.Service
	notifications notification.Service
	realtime      realtime.Service
	dashboards    dashboard.Service
	rpcSvc        rpc.Service
	otaSvc        ota.Service
	integrations  integration.Service
	edgeHub       edge.Service
	scadaSvc      scada.Service
	ruleEngine    *rule.Engine
	bus           bus.Bus
}

func NewAPI(idSvc *identity.Service, devMgr *core.DeviceManager, tsStore telemetry.Store, b bus.Bus) *API {
	attrs := telemetry.NewMemoryAttributeStore()
	tracker := devstate.NewMemoryTracker(60 * time.Second)
	alarmSvc := alarm.NewMemoryService()
	notifSvc := notification.NewMemoryService(notification.Config{})

	a := &API{
		router:        chi.NewRouter(),
		identity:      idSvc,
		devManager:    devMgr,
		telemetry:     tsStore,
		attributes:    attrs,
		tracker:       tracker,
		alarms:        alarmSvc,
		notifications: notifSvc,
		dashboards:    dashboard.NewMemoryService(),
		rpcSvc:        rpc.NewMemoryService(),
		otaSvc:        ota.NewService(nil, nil, nil),
		integrations:  integration.NewMemoryService(devMgr, tsStore, attrs, tracker, nil),
		edgeHub:       edge.NewMemoryHub(tsStore, alarmSvc),
		scadaSvc:      scada.NewMemoryService(),
		ruleEngine:    rule.NewEngine(alarmSvc, nil),
		bus:           b,
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

func (a *API) WithNotifications(n notification.Service) *API {
	a.notifications = n
	return a
}

func (a *API) WithRealtime(rt realtime.Service) *API {
	a.realtime = rt
	return a
}

func (a *API) WithDashboards(d dashboard.Service) *API {
	a.dashboards = d
	return a
}

func (a *API) WithRPC(r rpc.Service) *API {
	a.rpcSvc = r
	return a
}

func (a *API) WithOTA(o ota.Service) *API {
	a.otaSvc = o
	return a
}

func (a *API) WithIntegrations(i integration.Service) *API {
	a.integrations = i
	return a
}

func (a *API) WithEdge(e edge.Service) *API {
	a.edgeHub = e
	return a
}

func (a *API) WithScada(s scada.Service) *API {
	a.scadaSvc = s
	return a
}

func (a *API) WithRuleEngine(re *rule.Engine) *API {
	a.ruleEngine = re
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

		// Realtime WebSocket Gateway: GET /api/v1/ws
		r.Get("/ws", a.handleWebSocket)

		// Public Integration Webhook Endpoints
		r.Post("/integrations/webhook/{routingKey}", a.handleIntegrationWebhook)
		r.Post("/integrations/{routingKey}/webhook", a.handleIntegrationWebhook)

		// Public Edge Node Store-and-Forward & Synchronization Endpoints
		r.Post("/edge/sync", a.handleEdgeSync)
		r.Post("/edge/heartbeat", a.handleEdgeHeartbeat)
		r.Get("/edge/configuration", a.handleGetEdgeConfiguration)

		// Device wire-compatible HTTP endpoints:
		// 1. Telemetry upload: POST /api/v1/{token}/telemetry
		r.Post("/{token}/telemetry", a.handleDeviceTelemetryUpload)
		// 2. Client attributes upload: POST /api/v1/{token}/attributes
		r.Post("/{token}/attributes", a.handleDeviceAttributesUpload)
		// 3. Attributes query: GET /api/v1/{token}/attributes
		r.Get("/{token}/attributes", a.handleDeviceAttributesGet)
		// 4. Device RPC polling: GET /api/v1/{token}/rpc
		r.Get("/{token}/rpc", a.handleDeviceRpcPoll)
		// 5. Device RPC response: POST /api/v1/{token}/rpc/{requestId}
		r.Post("/{token}/rpc/{requestId}", a.handleDeviceRpcResponse)
		// 6. Device Firmware chunk download: GET /api/v1/{token}/firmware
		r.Get("/{token}/firmware", a.handleDeviceFirmwareChunkDownload)

		// Protected management endpoints
		r.Group(func(protected chi.Router) {
			protected.Use(a.authMiddleware)

			// Device CRUD & Activity / State
			protected.Post("/devices", a.handleCreateDevice)
			protected.Get("/devices", a.handleListDevices)
			protected.Get("/devices/{id}", a.handleGetDevice)
			protected.Get("/devices/{id}/credentials", a.handleGetDeviceCredentials)
			protected.Get("/devices/{id}/activity", a.handleGetDeviceActivity)
			protected.Get("/devices/{id}/state", a.handleGetDeviceState)
			protected.Post("/devices/{id}/state/override", a.handleOverrideDeviceState)
			protected.Get("/devstate/summary", a.handleGetDevStateSummary)
			protected.Delete("/devices/{id}", a.handleDeleteDevice)
			protected.Post("/devices/bulk/import", a.handleBulkImportDevices)
			protected.Get("/devices/bulk/export", a.handleBulkExportDevices)

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

			// Stage 5: Alarm Rules
			protected.Get("/alarmRules", a.handleListAlarmRules)
			protected.Post("/alarmRules", a.handleSaveAlarmRule)
			protected.Get("/alarmRules/{id}", a.handleGetAlarmRule)
			protected.Delete("/alarmRules/{id}", a.handleDeleteAlarmRule)
			protected.Post("/alarmRules/test", a.handleTestAlarmRule)

			// Stage 5: Notification Targets, Templates, Rules & Inbox
			protected.Get("/notification/targets", a.handleListNotificationTargets)
			protected.Post("/notification/targets", a.handleSaveNotificationTarget)
			protected.Delete("/notification/targets/{id}", a.handleDeleteNotificationTarget)

			protected.Get("/notification/templates", a.handleListNotificationTemplates)
			protected.Post("/notification/templates", a.handleSaveNotificationTemplate)
			protected.Delete("/notification/templates/{id}", a.handleDeleteNotificationTemplate)

			protected.Get("/notification/rules", a.handleListNotificationRules)
			protected.Post("/notification/rules", a.handleSaveNotificationRule)
			protected.Delete("/notification/rules/{id}", a.handleDeleteNotificationRule)

			protected.Get("/notification/inbox", a.handleListInbox)
			protected.Post("/notification/inbox/{id}/read", a.handleMarkInboxRead)
			protected.Post("/notification/inbox/read-all", a.handleMarkAllInboxRead)
			protected.Get("/notification/inbox/unread-count", a.handleGetInboxUnreadCount)
			protected.Post("/notification/send", a.handleSendNotification)

			// Dashboard management
			protected.Post("/dashboards", a.handleCreateDashboard)
			protected.Get("/dashboards", a.handleListDashboards)
			protected.Get("/dashboards/{id}", a.handleGetDashboard)
			protected.Put("/dashboards/{id}", a.handleUpdateDashboard)
			protected.Delete("/dashboards/{id}", a.handleDeleteDashboard)

			// Downlink RPC
			protected.Post("/plugins/rpc/oneway/DEVICE/{id}", a.handleSendOneWayRpc)
			protected.Post("/plugins/rpc/twoway/DEVICE/{id}", a.handleSendTwoWayRpc)
			protected.Get("/plugins/rpc/DEVICE/{id}", a.handleGetDeviceRpcHistory)
			protected.Get("/rpc/persistent/{id}", a.handleGetPersistentRpc)
			protected.Delete("/rpc/persistent/{id}", a.handleDeletePersistentRpc)

			// Stage 7: Firmware & Software OTA Management
			protected.Get("/otaPackages", a.handleListOtaPackages)
			protected.Post("/otaPackages", a.handleCreateOtaPackage)
			protected.Get("/otaPackages/{id}", a.handleGetOtaPackage)
			protected.Delete("/otaPackages/{id}", a.handleDeleteOtaPackage)
			protected.Post("/otaPackages/{id}/upload", a.handleUploadOtaBinary)
			protected.Get("/otaPackages/{id}/download", a.handleDownloadOtaBinary)
			protected.Get("/otaPackages/{id}/fleet", a.handleListFleetOtaStates)
			protected.Post("/devices/{id}/ota/assign", a.handleAssignDeviceOta)
			protected.Get("/devices/{id}/ota/state", a.handleGetDeviceOtaState)

			// Stage 8: Converters & Integrations
			protected.Get("/converters", a.handleListConverters)
			protected.Post("/converters", a.handleCreateConverter)
			protected.Get("/converters/{id}", a.handleGetConverter)
			protected.Delete("/converters/{id}", a.handleDeleteConverter)
			protected.Post("/converters/test", a.handleTestConverter)

			protected.Get("/integrations", a.handleListIntegrations)
			protected.Post("/integrations", a.handleCreateIntegration)
			protected.Get("/integrations/{id}", a.handleGetIntegration)
			protected.Delete("/integrations/{id}", a.handleDeleteIntegration)
			protected.Get("/integrations/{id}/status", a.handleGetIntegrationStatus)
			protected.Get("/integrations/{id}/debug", a.handleGetIntegrationDebug)

			// Stage 9: Edge Fleet Management
			protected.Get("/edge", a.handleListEdgeNodes)
			protected.Post("/edge", a.handleCreateEdgeNode)
			protected.Get("/edge/{id}", a.handleGetEdgeNode)
			protected.Delete("/edge/{id}", a.handleDeleteEdgeNode)
			protected.Post("/edge/{id}/assignments", a.handleAssignEdgeDevices)
			protected.Delete("/edge/{id}/assignments/{deviceId}", a.handleUnassignEdgeDevice)
			protected.Get("/edge/{id}/assignments", a.handleGetEdgeAssignments)
			protected.Post("/edge/{id}/commands", a.handleExecuteEdgeCommand)
			protected.Get("/edge/{id}/journal", a.handleGetEdgeJournal)

			// Stage 10: SCADA Tags, Process Alarms, SBO Control & ThingsBoard SVG Symbols
			protected.Get("/scada/tags", a.handleListScadaTags)
			protected.Post("/scada/tags", a.handleRegisterScadaTag)
			protected.Post("/scada/tags/update", a.handleUpdateScadaTag)
			protected.Get("/scada/alarms", a.handleListScadaAlarms)
			protected.Get("/scada/alarms/kpis", a.handleGetScadaAlarmKPIs)
			protected.Post("/scada/alarms/{id}/ack", a.handleAckScadaAlarm)
			protected.Post("/scada/alarms/{id}/shelve", a.handleShelveScadaAlarm)
			protected.Post("/scada/alarms/{id}/unshelve", a.handleUnshelveScadaAlarm)
			protected.Post("/scada/control/select", a.handleSelectScadaControl)
			protected.Post("/scada/control/operate", a.handleOperateScadaControl)
			protected.Get("/scada/control/log", a.handleListScadaControlLogs)
			protected.Get("/scada/symbols", a.handleListScadaSymbols)
			protected.Get("/scada/symbols/{id}", a.handleGetScadaSymbol)
			protected.Post("/scada/symbols", a.handleSaveScadaSymbol)
			protected.Delete("/scada/symbols/{id}", a.handleDeleteScadaSymbol)

			// Stage 4: Rule Chains & DAG Node Catalog
			protected.Get("/ruleChains", a.handleListRuleChains)
			protected.Post("/ruleChains", a.handleCreateRuleChain)
			protected.Get("/ruleChains/{id}", a.handleGetRuleChain)
			protected.Delete("/ruleChains/{id}", a.handleDeleteRuleChain)
			protected.Post("/ruleChains/{id}/root", a.handleSetRootRuleChain)
			protected.Get("/ruleChains/{id}/metadata", a.handleGetRuleChainMetadata)
			protected.Post("/ruleChains/{id}/metadata", a.handleSaveRuleChainMetadata)
			protected.Get("/ruleChains/descriptors", a.handleListRuleDescriptors)
			protected.Post("/ruleChains/test", a.handleTestRuleChain)
			protected.Post("/ruleChains/{id}/test", a.handleTestRuleChainByID)
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

// GET /api/v1/devices/{id}/state
func (a *API) handleGetDeviceState(w http.ResponseWriter, r *http.Request) {
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

	st, err := a.tracker.GetState(r.Context(), deviceID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

// POST /api/v1/devices/{id}/state/override
func (a *API) handleOverrideDeviceState(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid request body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	st, err := a.tracker.SetOverride(r.Context(), deviceID, req.Active)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

// GET /api/v1/devstate/summary
func (a *API) handleGetDevStateSummary(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	summary, err := a.tracker.GetSummary(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
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

// 17. GET /api/v1/{token}/rpc (Device checks for pending downlink RPCs)
func (a *API) handleDeviceRpcPoll(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	creds, err := a.devManager.AuthenticateToken(r.Context(), token)
	if err != nil {
		errs.WriteHTTP(w, errs.Unauthorized("invalid device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	cmd, err := a.rpcSvc.GetPendingDeviceRpc(r.Context(), creds.DeviceID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to poll rpc", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if cmd == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cmd)
}

// 18. POST /api/v1/{token}/rpc/{requestId} (Device returns execution result)
func (a *API) handleDeviceRpcResponse(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	creds, err := a.devManager.AuthenticateToken(r.Context(), token)
	if err != nil {
		errs.WriteHTTP(w, errs.Unauthorized("invalid device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	reqID, err := ids.Parse(chi.URLParam(r, "requestId"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid request id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("failed to read body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.rpcSvc.DeliverRpcResponse(r.Context(), creds.DeviceID, reqID, body); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// 19. POST /api/v1/dashboards
type createDashboardReq struct {
	Title         string          `json:"title"`
	Configuration json.RawMessage `json:"configuration"`
}

func (a *API) handleCreateDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var req createDashboardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid json body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	d, err := a.dashboards.CreateDashboard(r.Context(), tenantID, req.Title, req.Configuration)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(d)
}

// 20. GET /api/v1/dashboards
func (a *API) handleListDashboards(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	list, total, err := a.dashboards.ListDashboards(r.Context(), tenantID, page, pageSize)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to list dashboards", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":     list,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// 21. GET /api/v1/dashboards/{id}
func (a *API) handleGetDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	dashID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid dashboard id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	d, err := a.dashboards.GetDashboard(r.Context(), tenantID, dashID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}

// 22. PUT /api/v1/dashboards/{id}
func (a *API) handleUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	dashID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid dashboard id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req createDashboardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid json body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	d, err := a.dashboards.UpdateDashboard(r.Context(), tenantID, dashID, req.Title, req.Configuration)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}

// 23. DELETE /api/v1/dashboards/{id}
func (a *API) handleDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	dashID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid dashboard id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.dashboards.DeleteDashboard(r.Context(), tenantID, dashID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "DELETED"})
}

// 24. POST /api/v1/plugins/rpc/oneway/DEVICE/{id}
type sendRpcReq struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (a *API) handleSendOneWayRpc(w http.ResponseWriter, r *http.Request) {
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

	var req sendRpcReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid json body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	cmd, err := a.rpcSvc.SendOneWayRpc(r.Context(), tenantID, deviceID, req.Method, req.Params)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cmd)
}

// 25. POST /api/v1/plugins/rpc/twoway/DEVICE/{id}
func (a *API) handleSendTwoWayRpc(w http.ResponseWriter, r *http.Request) {
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

	var req sendRpcReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid json body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	timeoutMs, _ := strconv.Atoi(r.URL.Query().Get("timeout"))
	timeout := 5 * time.Second
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	cmd, err := a.rpcSvc.SendTwoWayRpc(r.Context(), tenantID, deviceID, req.Method, req.Params, timeout)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cmd)
}

// 26. GET /api/v1/plugins/rpc/DEVICE/{id}
func (a *API) handleGetDeviceRpcHistory(w http.ResponseWriter, r *http.Request) {
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

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	cmds, total, err := a.rpcSvc.ListDeviceRpcHistory(r.Context(), tenantID, deviceID, page, pageSize)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to fetch rpc history", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":     cmds,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GET /api/v1/ws
func (a *API) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if a.realtime != nil {
		a.realtime.HandleWebSocket(w, r)
		return
	}
	http.Error(w, "realtime gateway not configured", http.StatusServiceUnavailable)
}

// GET /api/v1/rpc/persistent/{id}
func (a *API) handleGetPersistentRpc(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	cmdID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid command id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	cmd, err := a.rpcSvc.GetRpcCommand(r.Context(), tenantID, cmdID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cmd)
}

// DELETE /api/v1/rpc/persistent/{id}
func (a *API) handleDeletePersistentRpc(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	cmdID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid command id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.rpcSvc.DeleteRpcCommand(r.Context(), tenantID, cmdID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

