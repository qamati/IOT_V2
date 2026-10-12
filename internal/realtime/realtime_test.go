package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestRealtimeWebSocketLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	userStore := identity.NewMemoryUserStore()
	jwtSecret := []byte("test-realtime-secret-key-32bytes!")
	idSvc := identity.NewService(userStore, jwtSecret, 24*time.Hour)

	tenantID := ids.MustNew()
	user := &model.User{
		ID:        ids.MustNew(),
		TenantID:  &tenantID,
		Email:     "realtime@iotp.local",
		Authority: model.AuthorityTenantAdmin,
	}
	_ = idSvc.Register(context.Background(), user, "password123")
	token, _, err := idSvc.Login(context.Background(), "realtime@iotp.local", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	rtSvc := NewService(idSvc, logger)
	defer rtSvc.Close()

	server := httptest.NewServer(http.HandlerFunc(rtSvc.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=" + token

	// 1. Connect via WebSocket
	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v (resp: %v)", err, resp)
	}
	defer conn.Close()

	// 2. Read CONNECTED frame
	var frame ServerFrame
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("failed to read CONNECTED frame: %v", err)
	}
	if frame.Type != FrameTypeConnected {
		t.Fatalf("expected CONNECTED frame, got: %s", frame.Type)
	}

	// 3. Ping -> Pong
	_ = conn.WriteJSON(ClientFrame{Type: FrameTypePing})
	var pongFrame ServerFrame
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&pongFrame); err != nil {
		t.Fatalf("failed to read PONG frame: %v", err)
	}
	if pongFrame.Type != FrameTypePong {
		t.Fatalf("expected PONG frame, got: %s", pongFrame.Type)
	}

	// 4. Subscribe to Telemetry
	devID := ids.MustNew()
	_ = conn.WriteJSON(ClientFrame{
		Type:     FrameTypeSubscribe,
		CmdID:    1,
		Kind:     KindTelemetry,
		EntityID: devID.String(),
		Keys:     []string{"temperature", "humidity"},
	})

	var subAck ServerFrame
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&subAck); err != nil {
		t.Fatalf("failed to read subscribe ack: %v", err)
	}
	if subAck.CmdID != 1 || subAck.Kind != KindTelemetry {
		t.Fatalf("expected subscribe ack for cmdId 1, got: %v", subAck)
	}

	// 5. Broadcast Telemetry and verify delivery
	rtSvc.BroadcastTelemetry(tenantID, devID, time.Now().UnixMilli(), map[string]any{
		"temperature": 27.5,
		"humidity":    48.2,
		"pressure":    101.3, // Should be filtered out because only temperature & humidity were requested
	})

	var updateFrame ServerFrame
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&updateFrame); err != nil {
		t.Fatalf("failed to receive telemetry update frame: %v", err)
	}
	if updateFrame.Kind != KindTelemetry || updateFrame.EntityID != devID.String() {
		t.Fatalf("unexpected update frame content: %v", updateFrame)
	}

	dataMap, ok := updateFrame.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data in update frame, got: %T", updateFrame.Data)
	}
	if dataMap["temperature"] != 27.5 || dataMap["humidity"] != 48.2 {
		t.Fatalf("unexpected telemetry values in update frame: %v", dataMap)
	}
	if _, exists := dataMap["pressure"]; exists {
		t.Fatalf("pressure should have been filtered out: %v", dataMap)
	}

	// 6. Subscribe to Alarms
	_ = conn.WriteJSON(ClientFrame{
		Type:  FrameTypeSubscribe,
		CmdID: 2,
		Kind:  KindAlarms,
	})
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_ = conn.ReadJSON(&subAck)

	// Broadcast Alarm and verify delivery
	testAlarm := &model.Alarm{
		ID:           ids.MustNew(),
		TenantID:     tenantID,
		OriginatorID: devID,
		Type:         "High Temp",
		Severity:     model.SeverityCritical,
		Status:       model.AlarmActiveUnack,
		StartTs:      time.Now().UnixMilli(),
		Details:      json.RawMessage(`{"threshold":40}`),
	}
	rtSvc.BroadcastAlarm(testAlarm)

	var alarmFrame ServerFrame
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&alarmFrame); err != nil {
		t.Fatalf("failed to receive alarm update frame: %v", err)
	}
	if alarmFrame.Kind != KindAlarms {
		t.Fatalf("expected alarm update frame, got: %v", alarmFrame)
	}
}
