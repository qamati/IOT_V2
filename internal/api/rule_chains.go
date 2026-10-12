package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// handleListRuleChains lists all rule chains for the caller's tenant.
func (a *API) handleListRuleChains(w http.ResponseWriter, r *http.Request) {
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

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if limit < 1 {
		limit = 10
	}

	// Ensure tenant has at least the Root Rule Chain seeded
	_, _ = a.ruleEngine.EnsureRootRuleChain(r.Context(), tenantID)

	chains, total, err := a.ruleEngine.Store().ListRuleChains(r.Context(), tenantID, page, limit)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":     chains,
		"total":    total,
		"page":     page,
		"pageSize": limit,
	})
}

// handleCreateRuleChain creates or updates a rule chain's top-level metadata.
func (a *API) handleCreateRuleChain(w http.ResponseWriter, r *http.Request) {
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

	var chain model.RuleChain
	if err := json.NewDecoder(r.Body).Decode(&chain); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule chain JSON"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	isNew := chain.ID == ids.Nil
	if isNew {
		chain.ID = ids.MustNew()
		chain.CreatedTime = time.Now().UnixMilli()
		chain.PublishedRevision = 1
	}
	chain.TenantID = tenantID
	if chain.Name == "" {
		chain.Name = "Untitled Rule Chain"
	}

	if err := a.ruleEngine.Store().SaveRuleChain(r.Context(), &chain); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if isNew {
		// Initialize revision 1 with a default root graph layout
		rev := &model.RuleChainRevision{
			RuleChainID: chain.ID,
			Revision:    1,
			Status:      "PUBLISHED",
			Definition:  rule.NewDefaultRootChainDefinition(),
			Comment:     "Initial revision",
			CreatedTime: time.Now().UnixMilli(),
		}
		_ = a.ruleEngine.Store().SaveRevision(r.Context(), rev)
	}

	a.ruleEngine.InvalidateChain(chain.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(chain)
}

// handleGetRuleChain retrieves rule chain metadata by ID.
func (a *API) handleGetRuleChain(w http.ResponseWriter, r *http.Request) {
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

	chainID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule chain ID format"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	chain, err := a.ruleEngine.Store().GetRuleChain(r.Context(), tenantID, chainID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chain)
}

// handleDeleteRuleChain deletes a rule chain.
func (a *API) handleDeleteRuleChain(w http.ResponseWriter, r *http.Request) {
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

	chainID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule chain ID format"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.ruleEngine.Store().DeleteRuleChain(r.Context(), tenantID, chainID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	a.ruleEngine.InvalidateChain(chainID)
	w.WriteHeader(http.StatusNoContent)
}

// handleSetRootRuleChain designates a rule chain as the root chain for the tenant.
func (a *API) handleSetRootRuleChain(w http.ResponseWriter, r *http.Request) {
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

	chainID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule chain ID format"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.ruleEngine.Store().SetRootRuleChain(r.Context(), tenantID, chainID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "rootChainId": chainID})
}

// handleGetRuleChainMetadata returns the latest published revision definition.
func (a *API) handleGetRuleChainMetadata(w http.ResponseWriter, r *http.Request) {
	chainID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule chain ID format"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	rev, err := a.ruleEngine.Store().GetLatestRevision(r.Context(), chainID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rev)
}

type saveMetadataRequest struct {
	Definition model.ChainDefinition `json:"definition"`
	Comment    string                `json:"comment,omitempty"`
	Status     string                `json:"status,omitempty"`
}

// handleSaveRuleChainMetadata saves a new revision of the rule chain graph.
func (a *API) handleSaveRuleChainMetadata(w http.ResponseWriter, r *http.Request) {
	chainID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule chain ID format"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req saveMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid metadata request JSON"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	nextRev := 1
	if latest, err := a.ruleEngine.Store().GetLatestRevision(r.Context(), chainID); err == nil && latest != nil {
		nextRev = latest.Revision + 1
	}

	status := req.Status
	if status == "" {
		status = "PUBLISHED"
	}

	newRev := &model.RuleChainRevision{
		RuleChainID: chainID,
		Revision:    nextRev,
		Status:      status,
		Definition:  req.Definition,
		Comment:     req.Comment,
		CreatedTime: time.Now().UnixMilli(),
	}

	if err := a.ruleEngine.Store().SaveRevision(r.Context(), newRev); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	a.ruleEngine.InvalidateChain(chainID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(newRev)
}

// handleListRuleDescriptors returns all available node descriptors from the catalog.
func (a *API) handleListRuleDescriptors(w http.ResponseWriter, r *http.Request) {
	descriptors := a.ruleEngine.Catalog().ListDescriptors()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(descriptors)
}

type testRuleChainRequest struct {
	Definition *model.ChainDefinition `json:"definition,omitempty"`
	MsgType    string                 `json:"msgType,omitempty"`
	Data       any                    `json:"data,omitempty"`
	Metadata   map[string]string      `json:"metadata,omitempty"`
	Originator string                 `json:"originatorId,omitempty"`
}

// handleTestRuleChain executes a test message against a provided definition.
func (a *API) handleTestRuleChain(w http.ResponseWriter, r *http.Request) {
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

	var req testRuleChainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid test request body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if req.Definition == nil {
		errs.WriteHTTP(w, errs.InvalidArgument("definition is required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	dataBytes, _ := json.Marshal(req.Data)
	if len(dataBytes) == 0 {
		dataBytes = []byte(`{}`)
	}

	origID := ids.MustNew()
	if req.Originator != "" {
		if parsed, err := ids.Parse(req.Originator); err == nil {
			origID = parsed
		}
	}

	msgType := req.MsgType
	if msgType == "" {
		msgType = rule.TypePostTelemetry
	}

	meta := req.Metadata
	if meta == nil {
		meta = make(map[string]string)
	}

	msg := &rule.Message{
		ID:       ids.MustNew(),
		TenantID: tenantID,
		Ts:       time.Now().UnixMilli(),
		Type:     msgType,
		Originator: model.EntityRef{
			EntityType: "DEVICE",
			ID:         origID,
		},
		Metadata: meta,
		DataType: "JSON",
		Data:     dataBytes,
	}

	trace, _ := a.ruleEngine.ExecuteTestDefinition(r.Context(), tenantID, req.Definition, msg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(trace)
}

// handleTestRuleChainByID executes a test message against an existing chain ID.
func (a *API) handleTestRuleChainByID(w http.ResponseWriter, r *http.Request) {
	chainID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule chain ID format"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req testRuleChainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid test request body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	claims, _ := identity.UserFromContext(r.Context())
	var tenantID ids.ID
	if claims != nil && claims.TenantID != "" {
		tenantID, _ = ids.Parse(claims.TenantID)
	}

	dataBytes, _ := json.Marshal(req.Data)
	if len(dataBytes) == 0 {
		dataBytes = []byte(`{}`)
	}

	origID := ids.MustNew()
	if req.Originator != "" {
		if parsed, err := ids.Parse(req.Originator); err == nil {
			origID = parsed
		}
	}

	msgType := req.MsgType
	if msgType == "" {
		msgType = rule.TypePostTelemetry
	}

	meta := req.Metadata
	if meta == nil {
		meta = make(map[string]string)
	}

	msg := &rule.Message{
		ID:       ids.MustNew(),
		TenantID: tenantID,
		Ts:       time.Now().UnixMilli(),
		Type:     msgType,
		Originator: model.EntityRef{
			EntityType: "DEVICE",
			ID:         origID,
		},
		Metadata: meta,
		DataType: "JSON",
		Data:     dataBytes,
	}

	trace, _ := a.ruleEngine.ExecuteTrace(r.Context(), chainID, msg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(trace)
}
