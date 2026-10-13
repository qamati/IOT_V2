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
	"github.com/qamati/IOT_V2/internal/edge"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/integration"
	"github.com/qamati/IOT_V2/internal/scada"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// ==========================================
// Stage 8: Integrations & Converters Handlers
// ==========================================

func (a *API) handleIntegrationWebhook(w http.ResponseWriter, r *http.Request) {
	routingKey := chi.URLParam(r, "routingKey")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("failed to read payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	event, err := a.integrations.ProcessWebhook(r.Context(), routingKey, body, headers)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ACCEPTED",
		"event":  event,
	})
}

func (a *API) handleListConverters(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	convs, err := a.integrations.ListConverters(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if convs == nil {
		convs = make([]*integration.Converter, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(convs)
}

func (a *API) handleCreateConverter(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var c integration.Converter
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	c.TenantID = tenantID

	if err := a.integrations.CreateConverter(r.Context(), &c); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(c)
}

func (a *API) handleGetConverter(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	converterID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid converter id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	c, err := a.integrations.GetConverter(r.Context(), tenantID, converterID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func (a *API) handleDeleteConverter(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	converterID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid converter id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.integrations.DeleteConverter(r.Context(), tenantID, converterID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleTestConverter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Converter integration.Converter `json:"converter"`
		Payload   string                `json:"payload"`
		Metadata  map[string]string     `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	event, err := a.integrations.TestConverter(r.Context(), &req.Converter, []byte(req.Payload), req.Metadata)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"event":   event,
	})
}

func (a *API) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	integs, err := a.integrations.ListIntegrations(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if integs == nil {
		integs = make([]*integration.Integration, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(integs)
}

func (a *API) handleCreateIntegration(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var i integration.Integration
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	i.TenantID = tenantID

	if err := a.integrations.CreateIntegration(r.Context(), &i); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(i)
}

func (a *API) handleGetIntegration(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	integID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid integration id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	i, err := a.integrations.GetIntegration(r.Context(), tenantID, integID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(i)
}

func (a *API) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	integID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid integration id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.integrations.DeleteIntegration(r.Context(), tenantID, integID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleGetIntegrationStatus(w http.ResponseWriter, r *http.Request) {
	integID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid integration id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	st, err := a.integrations.GetStatus(r.Context(), integID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (a *API) handleGetIntegrationDebug(w http.ResponseWriter, r *http.Request) {
	integID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid integration id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	logs, err := a.integrations.GetDebugEvents(r.Context(), integID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}

// ==========================================
// Stage 9: Edge Fleet & Store-and-Forward Sync
// ==========================================

func (a *API) handleListEdgeNodes(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	nodes, err := a.edgeHub.ListEdges(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if nodes == nil {
		nodes = make([]*edge.EdgeNode, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodes)
}

func (a *API) handleCreateEdgeNode(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var node edge.EdgeNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	node.TenantID = tenantID

	if err := a.edgeHub.CreateEdge(r.Context(), &node); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(node)
}

func (a *API) handleGetEdgeNode(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	edgeID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid edge id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	node, err := a.edgeHub.GetEdge(r.Context(), tenantID, edgeID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(node)
}

func (a *API) handleDeleteEdgeNode(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	edgeID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid edge id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.edgeHub.DeleteEdge(r.Context(), tenantID, edgeID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleEdgeSync(w http.ResponseWriter, r *http.Request) {
	var batch edge.SyncBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	ack, err := a.edgeHub.ProcessSync(r.Context(), &batch)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ack)
}

func (a *API) handleAssignEdgeDevices(w http.ResponseWriter, r *http.Request) {
	edgeID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid edge id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req edge.DeviceAssignmentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.edgeHub.AssignDevices(r.Context(), edgeID, req.DeviceIDs); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "assigned_count": len(req.DeviceIDs)})
}

func (a *API) handleUnassignEdgeDevice(w http.ResponseWriter, r *http.Request) {
	edgeID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid edge id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	deviceID, err := ids.Parse(chi.URLParam(r, "deviceId"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.edgeHub.UnassignDevice(r.Context(), edgeID, deviceID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleGetEdgeAssignments(w http.ResponseWriter, r *http.Request) {
	edgeID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid edge id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	devs, err := a.edgeHub.GetAssignedDevices(r.Context(), edgeID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if devs == nil {
		devs = []ids.ID{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(devs)
}

func (a *API) handleExecuteEdgeCommand(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	edgeID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid edge id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req edge.RemoteCommandReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	resp, err := a.edgeHub.ExecuteRemoteCommand(r.Context(), tenantID, edgeID, &req)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *API) handleGetEdgeJournal(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	edgeID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid edge id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	limit := 20
	journal, err := a.edgeHub.GetSyncJournal(r.Context(), tenantID, edgeID, limit)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if journal == nil {
		journal = []*edge.SyncJournalEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(journal)
}

func (a *API) handleEdgeHeartbeat(w http.ResponseWriter, r *http.Request) {
	edgeKey := r.Header.Get("X-Edge-Key")
	if edgeKey == "" {
		edgeKey = r.URL.Query().Get("key")
	}
	if edgeKey == "" {
		errs.WriteHTTP(w, errs.InvalidArgument("missing X-Edge-Key header or key parameter"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.edgeHub.Heartbeat(r.Context(), edgeKey); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "OK", "server_time": time.Now().UnixMilli()})
}

func (a *API) handleGetEdgeConfiguration(w http.ResponseWriter, r *http.Request) {
	edgeKey := r.Header.Get("X-Edge-Key")
	if edgeKey == "" {
		edgeKey = r.URL.Query().Get("key")
	}
	if edgeKey == "" {
		errs.WriteHTTP(w, errs.InvalidArgument("missing X-Edge-Key header or key parameter"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	cfg, err := a.edgeHub.GetConfiguration(r.Context(), edgeKey)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

// ==========================================
// Stage 10: SCADA Tags, Process Alarms & SBO Control
// ==========================================

func (a *API) handleListScadaTags(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	tags, err := a.scadaSvc.ListTags(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if tags == nil {
		tags = make([]*scada.Tag, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tags)
}

func (a *API) handleRegisterScadaTag(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var tag scada.Tag
	if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tag.TenantID = tenantID

	if err := a.scadaSvc.RegisterTag(r.Context(), &tag); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(tag)
}

func (a *API) handleUpdateScadaTag(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var req struct {
		Path    string `json:"path"`
		Value   any    `json:"value"`
		Quality string `json:"quality"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	tag, emitted, err := a.scadaSvc.UpdateTagValue(r.Context(), tenantID, req.Path, req.Value, req.Quality)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tag":     tag,
		"emitted": emitted,
	})
}

func (a *API) handleListScadaAlarms(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	alarms, err := a.scadaSvc.ListAlarms(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if alarms == nil {
		alarms = make([]*scada.ISA182Alarm, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alarms)
}

func (a *API) handleAckScadaAlarm(w http.ResponseWriter, r *http.Request) {
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

	user := claims.Subject
	if user == "" {
		user = "operator"
	}

	alarm, err := a.scadaSvc.AcknowledgeAlarm(r.Context(), tenantID, alarmID, user)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alarm)
}

func (a *API) handleShelveScadaAlarm(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		DurationMs int64 `json:"duration_ms"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	alarm, err := a.scadaSvc.ShelveAlarm(r.Context(), tenantID, alarmID, req.DurationMs)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alarm)
}

func (a *API) handleSelectScadaControl(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var req struct {
		TagPath      string `json:"tag_path"`
		DesiredValue any    `json:"desired_value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	user := claims.Subject
	if user == "" {
		user = "operator"
	}

	cmd, err := a.scadaSvc.SelectControl(r.Context(), tenantID, req.TagPath, req.DesiredValue, user)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cmd)
}

func (a *API) handleOperateScadaControl(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var req struct {
		SelectID string `json:"select_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	selectUUID, err := ids.Parse(req.SelectID)
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid select id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	cmd, err := a.scadaSvc.OperateControl(r.Context(), tenantID, selectUUID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cmd)
}

func (a *API) handleGetScadaAlarmKPIs(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	kpis, err := a.scadaSvc.GetAlarmKPIs(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(kpis)
}

func (a *API) handleUnshelveScadaAlarm(w http.ResponseWriter, r *http.Request) {
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

	alarm, err := a.scadaSvc.UnshelveAlarm(r.Context(), tenantID, alarmID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alarm)
}

func (a *API) handleListScadaControlLogs(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	logs, err := a.scadaSvc.ListControlLogs(r.Context(), tenantID, limit)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if logs == nil {
		logs = make([]*scada.ControlLogEntry, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}

func (a *API) handleListScadaSymbols(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	category := r.URL.Query().Get("category")
	symbols, err := a.scadaSvc.ListSymbols(r.Context(), tenantID, category)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	if symbols == nil {
		symbols = make([]*scada.Symbol, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(symbols)
}

func (a *API) handleGetScadaSymbol(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	symbolID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid symbol id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	sym, err := a.scadaSvc.GetSymbol(r.Context(), tenantID, symbolID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sym)
}

func (a *API) handleSaveScadaSymbol(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var sym scada.Symbol
	if err := json.NewDecoder(r.Body).Decode(&sym); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("malformed JSON body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	sym.TenantID = &tenantID
	if err := a.scadaSvc.SaveSymbol(r.Context(), &sym); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sym)
}

func (a *API) handleDeleteScadaSymbol(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	symbolID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid symbol id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.scadaSvc.DeleteSymbol(r.Context(), tenantID, symbolID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}
