package ota

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// ErrNotFound is returned when an OTA package or assignment is missing.
var ErrNotFound = errs.NotFound("ota package not found")

// AttributesPublisher sets shared attributes on device or profile when OTA is assigned.
type AttributesPublisher interface {
	SaveAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, values map[string]any) error
}

// Service manages OTA package lifecycle, chunked distribution, and device rollout state.
type Service interface {
	CreatePackage(ctx context.Context, tenantID ids.ID, pkgType model.OtaPackageType, title, version, tag, url string) (*model.OtaPackage, error)
	GetPackage(ctx context.Context, tenantID, pkgID ids.ID) (*model.OtaPackage, error)
	ListPackages(ctx context.Context, tenantID ids.ID, pkgType string, page, pageSize int) ([]model.OtaPackage, int, error)
	DeletePackage(ctx context.Context, tenantID, pkgID ids.ID) error
	UploadPackageBinary(ctx context.Context, tenantID, pkgID ids.ID, fileName, contentType string, data []byte) (*model.OtaPackage, error)
	GetPackageBinary(ctx context.Context, tenantID, pkgID ids.ID) ([]byte, error)
	GetPackageChunk(ctx context.Context, tenantID, pkgID ids.ID, chunkIndex, chunkSize int) ([]byte, int64, error)
	AssignPackage(ctx context.Context, tenantID ids.ID, kind string, packageID ids.ID, scope string, targetID ids.ID) (*model.OtaAssignment, error)
	GetDeviceOtaState(ctx context.Context, deviceID ids.ID, kind string) (*model.DeviceOtaState, error)
	UpdateDeviceOtaProgress(ctx context.Context, deviceID ids.ID, kind string, state string, progress float32, errStr string) error
	ListFleetOtaStates(ctx context.Context, packageID *ids.ID) ([]model.DeviceOtaState, error)
}

type defaultService struct {
	store      Store
	attrPub    AttributesPublisher
	logger     *slog.Logger
}

// NewService creates a new OTA Service instance.
func NewService(store Store, attrPub AttributesPublisher, logger *slog.Logger) Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &defaultService{
		store:   store,
		attrPub: attrPub,
		logger:  logger,
	}
}

func (s *defaultService) CreatePackage(ctx context.Context, tenantID ids.ID, pkgType model.OtaPackageType, title, version, tag, url string) (*model.OtaPackage, error) {
	if title == "" || version == "" {
		return nil, errs.InvalidArgument("title and version are required")
	}

	pkg := &model.OtaPackage{
		ID:                ids.MustNew(),
		CreatedTime:       time.Now().UnixMilli(),
		TenantID:          tenantID,
		Type:              pkgType,
		Title:             title,
		Version:           version,
		Tag:               tag,
		URL:               url,
		ChecksumAlgorithm: "SHA256",
		Checksum:          "",
		DataSize:          0,
		AdditionalInfo:    []byte("{}"),
	}

	if err := s.store.CreatePackage(ctx, pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

func (s *defaultService) GetPackage(ctx context.Context, tenantID, pkgID ids.ID) (*model.OtaPackage, error) {
	return s.store.GetPackage(ctx, tenantID, pkgID)
}

func (s *defaultService) ListPackages(ctx context.Context, tenantID ids.ID, pkgType string, page, pageSize int) ([]model.OtaPackage, int, error) {
	return s.store.ListPackages(ctx, tenantID, pkgType, page, pageSize)
}

func (s *defaultService) DeletePackage(ctx context.Context, tenantID, pkgID ids.ID) error {
	return s.store.DeletePackage(ctx, tenantID, pkgID)
}

func (s *defaultService) UploadPackageBinary(ctx context.Context, tenantID, pkgID ids.ID, fileName, contentType string, data []byte) (*model.OtaPackage, error) {
	pkg, err := s.store.GetPackage(ctx, tenantID, pkgID)
	if err != nil {
		return nil, err
	}

	// Calculate SHA256 checksum
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	if err := s.store.SavePackageData(ctx, tenantID, pkgID, data); err != nil {
		return nil, err
	}

	pkg.FileName = fileName
	pkg.ContentType = contentType
	pkg.Checksum = checksum
	pkg.ChecksumAlgorithm = "SHA256"
	pkg.DataSize = int64(len(data))

	return pkg, nil
}

func (s *defaultService) GetPackageBinary(ctx context.Context, tenantID, pkgID ids.ID) ([]byte, error) {
	return s.store.GetPackageData(ctx, tenantID, pkgID)
}

func (s *defaultService) GetPackageChunk(ctx context.Context, tenantID, pkgID ids.ID, chunkIndex, chunkSize int) ([]byte, int64, error) {
	data, err := s.store.GetPackageData(ctx, tenantID, pkgID)
	if err != nil {
		return nil, 0, err
	}

	totalSize := int64(len(data))
	if chunkSize <= 0 {
		chunkSize = 4096 // 4 KB default chunk
	}

	start := int64(chunkIndex * chunkSize)
	if start >= totalSize {
		return []byte{}, totalSize, nil
	}

	end := start + int64(chunkSize)
	if end > totalSize {
		end = totalSize
	}

	return data[start:end], totalSize, nil
}

func (s *defaultService) AssignPackage(ctx context.Context, tenantID ids.ID, kind string, packageID ids.ID, scope string, targetID ids.ID) (*model.OtaAssignment, error) {
	pkg, err := s.store.GetPackage(ctx, tenantID, packageID)
	if err != nil {
		return nil, fmt.Errorf("cannot assign non-existent package: %w", err)
	}

	assign := &model.OtaAssignment{
		ID:          ids.MustNew(),
		CreatedTime: time.Now().UnixMilli(),
		TenantID:    tenantID,
		Kind:        kind,
		PackageID:   packageID,
		Scope:       scope,
		TargetID:    targetID,
	}

	if err := s.store.AssignPackage(ctx, assign); err != nil {
		return nil, err
	}

	// Synchronize shared attributes on target device
	if s.attrPub != nil && scope == "DEVICE" {
		prefix := "fw_"
		if kind == "SOFTWARE" {
			prefix = "sw_"
		}

		attrs := map[string]any{
			prefix + "title":              pkg.Title,
			prefix + "version":            pkg.Version,
			prefix + "checksum":           pkg.Checksum,
			prefix + "checksum_algorithm": pkg.ChecksumAlgorithm,
		}

		if pkg.DataSize > 0 {
			attrs[prefix+"size"] = pkg.DataSize
		}
		if pkg.URL != "" {
			attrs[prefix+"url"] = pkg.URL
		}

		_ = s.attrPub.SaveAttributes(ctx, targetID, model.ScopeShared, attrs)
	}

	// Initialize device OTA state
	if scope == "DEVICE" {
		_ = s.store.UpdateDeviceOtaState(ctx, &model.DeviceOtaState{
			DeviceID:  targetID,
			Kind:      kind,
			PackageID: &packageID,
			State:     "QUEUED",
			Progress:  0.0,
			UpdatedTs: time.Now().UnixMilli(),
		})
	}

	return assign, nil
}

func (s *defaultService) GetDeviceOtaState(ctx context.Context, deviceID ids.ID, kind string) (*model.DeviceOtaState, error) {
	return s.store.GetDeviceOtaState(ctx, deviceID, kind)
}

func (s *defaultService) UpdateDeviceOtaProgress(ctx context.Context, deviceID ids.ID, kind string, state string, progress float32, errStr string) error {
	return s.store.UpdateDeviceOtaState(ctx, &model.DeviceOtaState{
		DeviceID:  deviceID,
		Kind:      kind,
		State:     state,
		Progress:  progress,
		Error:     errStr,
		UpdatedTs: time.Now().UnixMilli(),
	})
}

func (s *defaultService) ListFleetOtaStates(ctx context.Context, packageID *ids.ID) ([]model.DeviceOtaState, error) {
	return s.store.ListFleetOtaStates(ctx, packageID)
}

// ComputeChecksum helper
func ComputeChecksum(data []byte, algo string) string {
	switch algo {
	case "MD5":
		h := md5.Sum(data)
		return hex.EncodeToString(h[:])
	default:
		h := sha256.Sum256(data)
		return hex.EncodeToString(h[:])
	}
}
