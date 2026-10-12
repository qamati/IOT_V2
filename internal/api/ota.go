package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// handleCreateOtaPackage handles POST /api/v1/otaPackages.
func (a *API) handleCreateOtaPackage(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req struct {
		Type    string `json:"type"` // FIRMWARE or SOFTWARE
		Title   string `json:"title"`
		Version string `json:"version"`
		Tag     string `json:"tag"`
		URL     string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid request body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	pkgType := model.OtaPackageType(req.Type)
	if pkgType != model.OtaTypeSoftware {
		pkgType = model.OtaTypeFirmware
	}

	pkg, err := a.otaSvc.CreatePackage(r.Context(), tenantID, pkgType, req.Title, req.Version, req.Tag, req.URL)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pkg)
}

// handleListOtaPackages handles GET /api/v1/otaPackages.
func (a *API) handleListOtaPackages(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	pkgType := r.URL.Query().Get("type")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}

	packages, total, err := a.otaSvc.ListPackages(r.Context(), tenantID, pkgType, page, pageSize)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":     packages,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// handleGetOtaPackage handles GET /api/v1/otaPackages/{id}.
func (a *API) handleGetOtaPackage(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	pkgID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid package ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	pkg, err := a.otaSvc.GetPackage(r.Context(), tenantID, pkgID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pkg)
}

// handleDeleteOtaPackage handles DELETE /api/v1/otaPackages/{id}.
func (a *API) handleDeleteOtaPackage(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	pkgID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid package ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	if err := a.otaSvc.DeletePackage(r.Context(), tenantID, pkgID); err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleUploadOtaBinary handles POST /api/v1/otaPackages/{id}/upload.
func (a *API) handleUploadOtaBinary(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	pkgID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid package ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	// Limit upload size to 64MB in memory
	_ = r.ParseMultipartForm(64 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		// Try raw body if multipart wasn't provided
		bodyBytes, errRead := io.ReadAll(r.Body)
		if errRead != nil || len(bodyBytes) == 0 {
			errs.WriteHTTP(w, errs.InvalidArgument("missing binary file in request"), middleware.GetReqID(r.Context()), r.URL.Path)
			return
		}
		updated, errUpload := a.otaSvc.UploadPackageBinary(r.Context(), tenantID, pkgID, "firmware.bin", "application/octet-stream", bodyBytes)
		if errUpload != nil {
			errs.WriteHTTP(w, errUpload, middleware.GetReqID(r.Context()), r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(updated)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("failed to read uploaded file", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	updated, err := a.otaSvc.UploadPackageBinary(r.Context(), tenantID, pkgID, header.Filename, contentType, data)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

// handleDownloadOtaBinary handles GET /api/v1/otaPackages/{id}/download.
func (a *API) handleDownloadOtaBinary(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	pkgID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid package ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	data, err := a.otaSvc.GetPackageBinary(r.Context(), tenantID, pkgID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// handleAssignDeviceOta handles POST /api/v1/devices/{id}/ota/assign.
func (a *API) handleAssignDeviceOta(w http.ResponseWriter, r *http.Request) {
	claims, ok := identity.UserFromContext(r.Context())
	if !ok || claims.TenantID == "" {
		errs.WriteHTTP(w, errs.Forbidden("tenant context required"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}
	tenantID, err := ids.Parse(claims.TenantID)
	if err != nil {
		errs.WriteHTTP(w, errs.Internal("invalid tenant ID", err), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var req struct {
		Kind      string `json:"kind"` // FIRMWARE or SOFTWARE
		PackageID string `json:"packageId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid request body"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	packageID, err := ids.Parse(req.PackageID)
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid package ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	assign, err := a.otaSvc.AssignPackage(r.Context(), tenantID, req.Kind, packageID, "DEVICE", deviceID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assign)
}

// handleGetDeviceOtaState handles GET /api/v1/devices/{id}/ota/state.
func (a *API) handleGetDeviceOtaState(w http.ResponseWriter, r *http.Request) {
	deviceID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid device ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "FIRMWARE"
	}

	state, err := a.otaSvc.GetDeviceOtaState(r.Context(), deviceID, kind)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// handleListFleetOtaStates handles GET /api/v1/otaPackages/{id}/fleet.
func (a *API) handleListFleetOtaStates(w http.ResponseWriter, r *http.Request) {
	pkgID, err := ids.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errs.WriteHTTP(w, errs.InvalidArgument("invalid package ID"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	fleet, err := a.otaSvc.ListFleetOtaStates(r.Context(), &pkgID)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fleet)
}

// handleDeviceFirmwareChunkDownload handles GET /api/v1/{token}/firmware (Device downloads its assigned binary chunk).
func (a *API) handleDeviceFirmwareChunkDownload(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	creds, err := a.devManager.AuthenticateToken(r.Context(), token)
	if err != nil {
		errs.WriteHTTP(w, errs.Unauthorized("invalid device token"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "FIRMWARE"
	}

	state, err := a.otaSvc.GetDeviceOtaState(r.Context(), creds.DeviceID, kind)
	if err != nil || state == nil || state.PackageID == nil {
		errs.WriteHTTP(w, errs.NotFound("no active OTA package assigned to device"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	chunkIdx, _ := strconv.Atoi(r.URL.Query().Get("chunk"))
	chunkSize, _ := strconv.Atoi(r.URL.Query().Get("chunkSize"))
	if chunkSize <= 0 {
		chunkSize = 8192 // 8 KB
	}

	chunk, total, err := a.otaSvc.GetPackageChunk(r.Context(), creds.TenantID, *state.PackageID, chunkIdx, chunkSize)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	// Update device progress
	if total > 0 {
		progress := float32((chunkIdx+1)*chunkSize) / float32(total)
		if progress > 1.0 {
			progress = 1.0
		}
		stateStr := "DOWNLOADING"
		if progress >= 1.0 {
			stateStr = "DOWNLOADED"
		}
		_ = a.otaSvc.UpdateDeviceOtaProgress(r.Context(), creds.DeviceID, kind, stateStr, progress, "")
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Total-Size", strconv.FormatInt(total, 10))
	w.Header().Set("Content-Length", strconv.Itoa(len(chunk)))
	_, _ = w.Write(chunk)
}
