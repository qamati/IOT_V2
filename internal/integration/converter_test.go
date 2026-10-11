package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestDeclarativeConverter_JSON(t *testing.T) {
	eng := NewEngine()

	defJSON, _ := json.Marshal(DeclarativeDefinition{
		Format:         "json",
		DeviceNamePath: "device_id",
		DeviceType:     "energy_meter",
		Mappings: []DeclarativeMapping{
			{Source: "power_kw", Target: "telemetry.power", Type: "float", Scale: 1000.0}, // scale to Watts
			{Source: "voltage", Target: "telemetry.voltage", Type: "float"},
			{Source: "status.online", Target: "attributes.is_online", Type: "bool"},
		},
	})

	conv := &Converter{
		ID:         ids.MustNew(),
		Name:       "Energy Meter Converter",
		Mode:       ModeDeclarative,
		Definition: defJSON,
	}

	payload := []byte(`{
		"device_id": "meter-substation-9",
		"power_kw": 4.5,
		"voltage": 230.1,
		"status": {
			"online": true
		}
	}`)

	event, err := eng.Convert(conv, payload, nil)
	if err != nil {
		t.Fatalf("unexpected convert error: %v", err)
	}

	if event.Device.Name != "meter-substation-9" {
		t.Errorf("expected device name meter-substation-9, got %s", event.Device.Name)
	}
	if len(event.Telemetry) == 0 {
		t.Fatalf("expected telemetry batch")
	}

	vals := event.Telemetry[0].Values
	if power, ok := vals["power"].(float64); !ok || power != 4500.0 {
		t.Errorf("expected power 4500.0, got %v", vals["power"])
	}
	if v, ok := vals["voltage"].(float64); !ok || v != 230.1 {
		t.Errorf("expected voltage 230.1, got %v", vals["voltage"])
	}
	if isOnline, ok := event.Attributes["is_online"].(bool); !ok || !isOnline {
		t.Errorf("expected attribute is_online true, got %v", event.Attributes["is_online"])
	}
}

func TestChirpStackConverter(t *testing.T) {
	eng := NewEngine()

	defJSON, _ := json.Marshal(DeclarativeDefinition{
		Format:         "chirpstack",
		DeviceNamePath: "deviceInfo.deviceName",
	})

	conv := &Converter{
		ID:         ids.MustNew(),
		Name:       "ChirpStack Converter",
		Mode:       ModeDeclarative,
		Definition: defJSON,
	}

	payload := []byte(`{
		"deviceInfo": {
			"deviceName": "soil-moisture-lora-1",
			"devEui": "0004a30b001c2345"
		},
		"fPort": 10,
		"rxInfo": [
			{"rssi": -78, "snr": 9.2}
		],
		"object": {
			"moisture_pct": 34.2,
			"temp_c": 19.8
		}
	}`)

	event, err := eng.Convert(conv, payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Device.Name != "soil-moisture-lora-1" {
		t.Errorf("expected device name soil-moisture-lora-1, got %s", event.Device.Name)
	}
	if len(event.Telemetry) == 0 {
		t.Fatalf("expected telemetry batch")
	}
	vals := event.Telemetry[0].Values
	if vals["moisture_pct"] != 34.2 || vals["temp_c"] != 19.8 {
		t.Errorf("unexpected telemetry values: %+v", vals)
	}
	if event.Attributes["rssi"] != float64(-78) {
		t.Errorf("expected rssi attribute -78, got %v", event.Attributes["rssi"])
	}
}

func TestIntegrationServiceWebhook(t *testing.T) {
	svc := NewMemoryService(nil, nil, nil, nil, nil)
	ctx := context.Background()

	convs, err := svc.ListConverters(ctx, ids.ID{})
	if err != nil || len(convs) == 0 {
		t.Fatalf("expected default seeded converters, got %v", err)
	}

	// Create test integration
	integ := &Integration{
		ID:                ids.MustNew(),
		TenantID:          ids.MustNew(),
		Name:              "Field Webhook Integration",
		Type:              TypeHTTPWebhook,
		Enabled:           true,
		UplinkConverterID: convs[0].ID,
		RoutingKey:        "test-webhook-key",
		AllowCreateDevice: true,
	}
	if err := svc.CreateIntegration(ctx, integ); err != nil {
		t.Fatalf("create integration failed: %v", err)
	}

	payload := []byte(`{"device_name": "auto-sensor-01", "data": {"temp": 28.4, "humidity": 65.1}}`)
	event, err := svc.ProcessWebhook(ctx, "test-webhook-key", payload, nil)
	if err != nil {
		t.Fatalf("process webhook failed: %v", err)
	}

	if event.Device.Name != "auto-sensor-01" {
		t.Errorf("expected auto-sensor-01, got %s", event.Device.Name)
	}

	// Verify debug logs were recorded
	logs, err := svc.GetDebugEvents(ctx, integ.ID)
	if err != nil || len(logs) != 1 {
		t.Fatalf("expected 1 debug event, got %v, err=%v", len(logs), err)
	}
	if !logs[0].Success {
		t.Errorf("expected debug event success true")
	}
}
