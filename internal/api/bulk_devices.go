package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// handleBulkImportDevices handles POST /api/v1/devices/bulk/import.
func (a *API) handleBulkImportDevices(w http.ResponseWriter, r *http.Request) {
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

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || len(bodyBytes) == 0 {
		errs.WriteHTTP(w, errs.InvalidArgument("empty bulk import payload"), middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	var importedDevices []model.Device
	var failedErrors []string

	// Check if JSON format
	trimmed := strings.TrimSpace(string(bodyBytes))
	if strings.HasPrefix(trimmed, "[") {
		var rows []struct {
			Name        string `json:"name"`
			Label       string `json:"label"`
			AccessToken string `json:"access_token"`
			Credentials string `json:"credentials_value"`
		}
		if err := json.Unmarshal(bodyBytes, &rows); err == nil {
			for _, row := range rows {
				token := row.AccessToken
				if token == "" {
					token = row.Credentials
				}
				dev, _, err := a.devManager.CreateDevice(r.Context(), tenantID, row.Name, row.Label, token)
				if err != nil {
					failedErrors = append(failedErrors, fmt.Sprintf("device '%s': %v", row.Name, err))
					continue
				}
				importedDevices = append(importedDevices, *dev)
			}
		} else {
			errs.WriteHTTP(w, errs.InvalidArgument("invalid JSON payload: "+err.Error()), middleware.GetReqID(r.Context()), r.URL.Path)
			return
		}
	} else {
		// Parse CSV
		reader := csv.NewReader(bytes.NewReader(bodyBytes))
		records, err := reader.ReadAll()
		if err != nil {
			errs.WriteHTTP(w, errs.InvalidArgument("invalid CSV format: "+err.Error()), middleware.GetReqID(r.Context()), r.URL.Path)
			return
		}

		if len(records) > 0 {
			headerMap := make(map[string]int)
			startIndex := 0

			// Header detection
			firstRow := records[0]
			if strings.EqualFold(firstRow[0], "name") || strings.Contains(strings.ToLower(strings.Join(firstRow, ",")), "name") {
				for idx, col := range firstRow {
					headerMap[strings.ToLower(strings.TrimSpace(col))] = idx
				}
				startIndex = 1
			} else {
				headerMap["name"] = 0
				headerMap["label"] = 1
				headerMap["credentials"] = 2
			}

			for i := startIndex; i < len(records); i++ {
				row := records[i]
				if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
					continue
				}

				nameIdx := headerMap["name"]
				if nameIdx >= len(row) {
					continue
				}
				name := strings.TrimSpace(row[nameIdx])
				if name == "" {
					continue
				}

				label := ""
				if labelIdx, ok := headerMap["label"]; ok && labelIdx < len(row) {
					label = strings.TrimSpace(row[labelIdx])
				}

				token := ""
				if credIdx, ok := headerMap["access_token"]; ok && credIdx < len(row) {
					token = strings.TrimSpace(row[credIdx])
				} else if credIdx, ok := headerMap["credentials_value"]; ok && credIdx < len(row) {
					token = strings.TrimSpace(row[credIdx])
				} else if credIdx, ok := headerMap["credentials"]; ok && credIdx < len(row) {
					token = strings.TrimSpace(row[credIdx])
				} else if credIdx, ok := headerMap["token"]; ok && credIdx < len(row) {
					token = strings.TrimSpace(row[credIdx])
				}

				dev, _, err := a.devManager.CreateDevice(r.Context(), tenantID, name, label, token)
				if err != nil {
					failedErrors = append(failedErrors, fmt.Sprintf("row %d ('%s'): %v", i+1, name, err))
					continue
				}

				importedDevices = append(importedDevices, *dev)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"imported": len(importedDevices),
		"failed":   len(failedErrors),
		"errors":   failedErrors,
		"devices":  importedDevices,
	})
}

// handleBulkExportDevices handles GET /api/v1/devices/bulk/export.
func (a *API) handleBulkExportDevices(w http.ResponseWriter, r *http.Request) {
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

	devices, _, err := a.devManager.ListDevices(r.Context(), tenantID, 0, 1000)
	if err != nil {
		errs.WriteHTTP(w, err, middleware.GetReqID(r.Context()), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="devices_export.csv"`)

	csvWriter := csv.NewWriter(w)
	_ = csvWriter.Write([]string{"id", "name", "label", "profile_id", "created_time"})

	for _, dev := range devices {
		_ = csvWriter.Write([]string{
			dev.ID.String(),
			dev.Name,
			dev.Label,
			dev.DeviceProfileID.String(),
			fmt.Sprintf("%d", dev.CreatedTime),
		})
	}
	csvWriter.Flush()
}
