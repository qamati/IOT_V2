package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// --- Stage 5: Alarm Rules APIs ---

// GET /api/v1/alarmRules
func (a *API) handleListAlarmRules(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	rules, err := a.alarms.ListAlarmRules(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to list alarm rules", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rules)
}

// POST /api/v1/alarmRules
func (a *API) handleSaveAlarmRule(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var rule model.AlarmRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid alarm rule payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	rule.TenantID = tenantID

	saved, err := a.alarms.SaveAlarmRule(r.Context(), &rule)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to save alarm rule", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(saved)
}

// GET /api/v1/alarmRules/{id}
func (a *API) handleGetAlarmRule(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	ruleID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid alarm rule id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	rule, err := a.alarms.GetAlarmRule(r.Context(), tenantID, ruleID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rule)
}

// DELETE /api/v1/alarmRules/{id}
func (a *API) handleDeleteAlarmRule(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	ruleID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid alarm rule id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.alarms.DeleteAlarmRule(r.Context(), tenantID, ruleID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/alarmRules/test (Simulation trace on historical / mock data points)
func (a *API) handleTestAlarmRule(w http.ResponseWriter, r *http.Request) {
	var req model.AlarmRuleTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid test simulation payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	res, err := a.alarms.TestAlarmRule(r.Context(), &req)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("simulation failed", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// --- Stage 5: Notification Targets APIs ---

// GET /api/v1/notification/targets
func (a *API) handleListNotificationTargets(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	targets, err := a.notifications.ListTargets(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to list notification targets", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(targets)
}

// POST /api/v1/notification/targets
func (a *API) handleSaveNotificationTarget(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var target model.NotificationTarget
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid target payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	target.TenantID = tenantID

	saved, err := a.notifications.SaveTarget(r.Context(), &target)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to save notification target", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(saved)
}

// DELETE /api/v1/notification/targets/{id}
func (a *API) handleDeleteNotificationTarget(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	targetID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid target id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.notifications.DeleteTarget(r.Context(), tenantID, targetID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Stage 5: Notification Templates APIs ---

// GET /api/v1/notification/templates
func (a *API) handleListNotificationTemplates(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	templates, err := a.notifications.ListTemplates(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to list notification templates", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(templates)
}

// POST /api/v1/notification/templates
func (a *API) handleSaveNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var tpl model.NotificationTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid template payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tpl.TenantID = tenantID

	saved, err := a.notifications.SaveTemplate(r.Context(), &tpl)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to save notification template", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(saved)
}

// DELETE /api/v1/notification/templates/{id}
func (a *API) handleDeleteNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	templateID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid template id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.notifications.DeleteTemplate(r.Context(), tenantID, templateID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Stage 5: Notification Rules APIs ---

// GET /api/v1/notification/rules
func (a *API) handleListNotificationRules(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	rules, err := a.notifications.ListRules(r.Context(), tenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to list notification rules", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rules)
}

// POST /api/v1/notification/rules
func (a *API) handleSaveNotificationRule(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var rule model.NotificationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	rule.TenantID = tenantID

	saved, err := a.notifications.SaveRule(r.Context(), &rule)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to save notification rule", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(saved)
}

// DELETE /api/v1/notification/rules/{id}
func (a *API) handleDeleteNotificationRule(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	ruleID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid rule id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.notifications.DeleteRule(r.Context(), tenantID, ruleID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Stage 5: In-App User Inbox APIs ---

// GET /api/v1/notification/inbox
func (a *API) handleListInbox(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)
	userID := claims.UserID

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	notifications, total, err := a.notifications.ListInbox(r.Context(), tenantID, userID, limit, offset)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to fetch inbox", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	unread, _ := a.notifications.GetUnreadCount(r.Context(), tenantID, userID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":         notifications,
		"total":        total,
		"unread_count": unread,
	})
}

// POST /api/v1/notification/inbox/{id}/read
func (a *API) handleMarkInboxRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	notificationID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid notification id"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.notifications.MarkRead(r.Context(), tenantID, notificationID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/notification/inbox/read-all
func (a *API) handleMarkAllInboxRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)
	userID := claims.UserID

	if err := a.notifications.MarkAllRead(r.Context(), tenantID, userID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/notification/inbox/unread-count
func (a *API) handleGetInboxUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)
	userID := claims.UserID

	count, err := a.notifications.GetUnreadCount(r.Context(), tenantID, userID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to fetch unread count", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"unread_count": count})
}

// POST /api/v1/notification/send
func (a *API) handleSendNotification(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, _ := ids.Parse(claims.TenantID)

	var req model.NotificationSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid notification send payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	result, err := a.notifications.SendNotification(r.Context(), tenantID, &req)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to send notification", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
