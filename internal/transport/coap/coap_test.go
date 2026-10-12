package coap

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestCoAP_EncodeDecode(t *testing.T) {
	orig := &Message{
		Type:      TypeConfirmable,
		Code:      CodePOST,
		MessageID: 1042,
		Token:     []byte{0xAA, 0xBB},
		Paths:     []string{"api", "v1", "device_secret_token", "telemetry"},
		Payload:   []byte(`{"temperature":24.5,"humidity":55.0}`),
	}

	raw := EncodeMessage(orig)
	parsed, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("parse message failed: %v", err)
	}

	if parsed.Type != orig.Type || parsed.Code != orig.Code || parsed.MessageID != orig.MessageID {
		t.Errorf("header mismatch: %+v vs %+v", parsed, orig)
	}
	if len(parsed.Paths) != len(orig.Paths) {
		t.Fatalf("path length mismatch: got %v, expected %v", parsed.Paths, orig.Paths)
	}
	for i := range parsed.Paths {
		if parsed.Paths[i] != orig.Paths[i] {
			t.Errorf("path[%d] mismatch: got %s, expected %s", i, parsed.Paths[i], orig.Paths[i])
		}
	}
	if string(parsed.Payload) != string(orig.Payload) {
		t.Errorf("payload mismatch: got %s, expected %s", string(parsed.Payload), string(orig.Payload))
	}
}

func TestCoAP_ServerTelemetry(t *testing.T) {
	devStore := core.NewMemoryDeviceStore()
	devMgr := core.NewDeviceManager(devStore, []byte("pepper"))
	tsStore := telemetry.NewMemoryStore()
	attrStore := telemetry.NewMemoryAttributeStore()

	ctx := context.Background()
	tenantID := ids.MustNew()
	dev, creds, err := devMgr.CreateDevice(ctx, tenantID, "coap-test-node-1", "CoAP Node", "")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	token := creds.CredentialsValue

	server := NewServer("127.0.0.1:0", devMgr, tsStore, attrStore, nil, nil)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start coap server: %v", err)
	}
	defer server.Stop()

	serverAddr := server.conn.LocalAddr().String()

	// Connect UDP client
	clientConn, err := net.Dial("udp", serverAddr)
	if err != nil {
		t.Fatalf("failed to dial coap server: %v", err)
	}
	defer clientConn.Close()

	// Send POST /api/v1/{token}/telemetry
	req := &Message{
		Type:      TypeConfirmable,
		Code:      CodePOST,
		MessageID: 2001,
		Token:     []byte{0x01, 0x02},
		Paths:     []string{"api", "v1", token, "telemetry"},
		Payload:   []byte(`{"coap_temp": 28.7, "coap_batt": 92}`),
	}
	reqBytes := EncodeMessage(req)
	_, _ = clientConn.Write(reqBytes)

	// Read ACK
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	respBuf := make([]byte, 1024)
	n, err := clientConn.Read(respBuf)
	if err != nil {
		t.Fatalf("failed to read coap ack: %v", err)
	}

	ackMsg, err := ParseMessage(respBuf[:n])
	if err != nil {
		t.Fatalf("failed to parse ack: %v", err)
	}

	if ackMsg.Type != TypeAcknowledgement || ackMsg.Code != CodeChanged {
		t.Errorf("unexpected ack code: type=%d, code=%d", ackMsg.Type, ackMsg.Code)
	}

	// Verify telemetry was saved
	res, err := tsStore.GetLatest(ctx, dev.ID, []string{"coap_temp", "coap_batt"})
	if err != nil || len(res) < 2 {
		t.Fatalf("expected 2 telemetry points, got %v, err=%v", len(res), err)
	}
	if *res["coap_temp"].DblV != 28.7 {
		t.Errorf("expected coap_temp 28.7, got %v", *res["coap_temp"].DblV)
	}
}
