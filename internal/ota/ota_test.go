package ota_test

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/ota"
	"github.com/qamati/IOT_V2/pkg/ids"
)

type mockOtaStore struct {
	packages    map[string]*model.OtaPackage
	packageData map[string][]byte
	assignments map[string]*model.OtaAssignment
	states      map[string]*model.DeviceOtaState
}

func newMockStore() *mockOtaStore {
	return &mockOtaStore{
		packages:    make(map[string]*model.OtaPackage),
		packageData: make(map[string][]byte),
		assignments: make(map[string]*model.OtaAssignment),
		states:      make(map[string]*model.DeviceOtaState),
	}
}

func (m *mockOtaStore) CreatePackage(ctx context.Context, pkg *model.OtaPackage) error {
	m.packages[pkg.ID.String()] = pkg
	return nil
}

func (m *mockOtaStore) GetPackage(ctx context.Context, tenantID, pkgID ids.ID) (*model.OtaPackage, error) {
	pkg, ok := m.packages[pkgID.String()]
	if !ok {
		return nil, ota.ErrNotFound
	}
	return pkg, nil
}

func (m *mockOtaStore) ListPackages(ctx context.Context, tenantID ids.ID, pkgType string, page, pageSize int) ([]model.OtaPackage, int, error) {
	list := make([]model.OtaPackage, 0, len(m.packages))
	for _, p := range m.packages {
		list = append(list, *p)
	}
	return list, len(list), nil
}

func (m *mockOtaStore) DeletePackage(ctx context.Context, tenantID, pkgID ids.ID) error {
	delete(m.packages, pkgID.String())
	delete(m.packageData, pkgID.String())
	return nil
}

func (m *mockOtaStore) SavePackageData(ctx context.Context, tenantID, pkgID ids.ID, data []byte) error {
	m.packageData[pkgID.String()] = data
	return nil
}

func (m *mockOtaStore) GetPackageData(ctx context.Context, tenantID, pkgID ids.ID) ([]byte, error) {
	return m.packageData[pkgID.String()], nil
}

func (m *mockOtaStore) AssignPackage(ctx context.Context, assign *model.OtaAssignment) error {
	m.assignments[assign.ID.String()] = assign
	return nil
}

func (m *mockOtaStore) GetAssignment(ctx context.Context, tenantID ids.ID, scope string, targetID ids.ID, kind string) (*model.OtaAssignment, error) {
	for _, a := range m.assignments {
		if a.Scope == scope && a.TargetID == targetID && a.Kind == kind {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockOtaStore) ListAssignments(ctx context.Context, tenantID ids.ID) ([]model.OtaAssignment, error) {
	list := make([]model.OtaAssignment, 0, len(m.assignments))
	for _, a := range m.assignments {
		list = append(list, *a)
	}
	return list, nil
}

func (m *mockOtaStore) UpdateDeviceOtaState(ctx context.Context, state *model.DeviceOtaState) error {
	m.states[state.DeviceID.String()+state.Kind] = state
	return nil
}

func (m *mockOtaStore) GetDeviceOtaState(ctx context.Context, deviceID ids.ID, kind string) (*model.DeviceOtaState, error) {
	return m.states[deviceID.String()+kind], nil
}

func (m *mockOtaStore) ListFleetOtaStates(ctx context.Context, packageID *ids.ID) ([]model.DeviceOtaState, error) {
	list := make([]model.DeviceOtaState, 0, len(m.states))
	for _, s := range m.states {
		list = append(list, *s)
	}
	return list, nil
}

func TestOtaService_Lifecycle(t *testing.T) {
	store := newMockStore()
	svc := ota.NewService(store, nil, nil)

	tenantID := ids.MustNew()
	pkg, err := svc.CreatePackage(context.Background(), tenantID, model.OtaTypeFirmware, "SmartMeter FW", "v2.0.1", "prod", "")
	if err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	payload := []byte("fake-firmware-binary-bytes-data-stream-1234567890")
	updatedPkg, err := svc.UploadPackageBinary(context.Background(), tenantID, pkg.ID, "firmware.bin", "application/octet-stream", payload)
	if err != nil {
		t.Fatalf("failed to upload binary: %v", err)
	}

	if updatedPkg.Checksum == "" {
		t.Errorf("expected calculated checksum, got empty string")
	}

	// Test chunked download
	chunk, total, err := svc.GetPackageChunk(context.Background(), tenantID, pkg.ID, 0, 10)
	if err != nil {
		t.Fatalf("failed to get chunk: %v", err)
	}
	if len(chunk) != 10 {
		t.Errorf("expected chunk size 10, got %d", len(chunk))
	}
	if total != int64(len(payload)) {
		t.Errorf("expected total size %d, got %d", len(payload), total)
	}

	// Test assignment
	deviceID := ids.MustNew()
	assign, err := svc.AssignPackage(context.Background(), tenantID, "FIRMWARE", pkg.ID, "DEVICE", deviceID)
	if err != nil {
		t.Fatalf("failed to assign package: %v", err)
	}
	if assign.PackageID != pkg.ID {
		t.Errorf("expected assigned package %s, got %s", pkg.ID, assign.PackageID)
	}

	// Test device progress update
	err = svc.UpdateDeviceOtaProgress(context.Background(), deviceID, "FIRMWARE", "DOWNLOADING", 0.45, "")
	if err != nil {
		t.Fatalf("failed to update device state: %v", err)
	}

	state, err := svc.GetDeviceOtaState(context.Background(), deviceID, "FIRMWARE")
	if err != nil || state == nil {
		t.Fatalf("failed to get device state: %v", err)
	}
	if state.State != "DOWNLOADING" || state.Progress != 0.45 {
		t.Errorf("expected state DOWNLOADING / 0.45, got %s / %f", state.State, state.Progress)
	}
}
