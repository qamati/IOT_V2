package scada

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestSCADA_DeadbandFiltering(t *testing.T) {
	svc := NewMemoryService()
	ctx := context.Background()

	tenantID := ids.MustNew()
	tag := &Tag{
		ID:          ids.MustNew(),
		TenantID:    tenantID,
		Path:        "Turbine.Speed",
		DataType:    DataTypeAnalog,
		EngUnit:     "RPM",
		Deadband:    5.0, // deadband of 5 RPM
		ScaleFactor: 1.0,
		Value:       1800.0,
	}

	if err := svc.RegisterTag(ctx, tag); err != nil {
		t.Fatalf("failed to register tag: %v", err)
	}

	// 1. Small change inside deadband (delta = 2.0 < 5.0) -> emitted = false
	updated, emitted, err := svc.UpdateTagValue(ctx, tenantID, "Turbine.Speed", 1802.0, QualityGood)
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if emitted {
		t.Errorf("expected emitted=false for delta inside deadband")
	}
	if updated.Value.(float64) != 1800.0 {
		t.Errorf("expected value to remain 1800.0, got %v", updated.Value)
	}

	// 2. Significant change exceeding deadband (delta = 10.0 > 5.0) -> emitted = true
	updated2, emitted2, err := svc.UpdateTagValue(ctx, tenantID, "Turbine.Speed", 1810.0, QualityGood)
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if !emitted2 {
		t.Errorf("expected emitted=true for delta exceeding deadband")
	}
	if updated2.Value.(float64) != 1810.0 {
		t.Errorf("expected value to update to 1810.0, got %v", updated2.Value)
	}
}

func TestSCADA_ISA182AlarmAndControl(t *testing.T) {
	svc := NewMemoryService()
	ctx := context.Background()
	tenantID := ids.MustNew()

	// Exceed tank level threshold to trigger HIGH_HIGH alarm
	_, _, err := svc.UpdateTagValue(ctx, tenantID, "WaterTreatment.Tank101.Level", 95.5, QualityGood)
	if err != nil {
		t.Fatalf("update error: %v", err)
	}

	alarms, err := svc.ListAlarms(ctx, tenantID)
	if err != nil || len(alarms) == 0 {
		t.Fatalf("expected alarm triggered, got %v", err)
	}
	if alarms[0].State != AlarmStateUnackAlarm {
		t.Errorf("expected state UNACK_ALARM, got %s", alarms[0].State)
	}

	// Acknowledge alarm
	acked, err := svc.AcknowledgeAlarm(ctx, tenantID, alarms[0].ID, "operator-1")
	if err != nil || acked.State != AlarmStateAckedAlarm {
		t.Errorf("failed to ack alarm: %v", err)
	}

	// SBO Select-Before-Operate: Stop pump
	cmd, err := svc.SelectControl(ctx, tenantID, "WaterTreatment.Pump101.Status", 0, "lead-operator")
	if err != nil || cmd.Status != "SELECTED" {
		t.Fatalf("select control failed: %v", err)
	}

	operated, err := svc.OperateControl(ctx, tenantID, cmd.SelectID)
	if err != nil || operated.Status != "OPERATED" {
		t.Fatalf("operate control failed: %v", err)
	}

	// Verify tag value updated
	tag, err := svc.GetTagByPath(ctx, tenantID, "WaterTreatment.Pump101.Status")
	if err != nil || tag.Value != 0 {
		t.Errorf("expected tag value updated to 0, got %v", tag.Value)
	}
}
