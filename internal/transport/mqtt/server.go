package mqtt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/ota"
	"github.com/qamati/IOT_V2/internal/rpc"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Config holds MQTT transport configuration.
type Config struct {
	Addr string `koanf:"addr"`
	Port int    `koanf:"port"`
}

// DeviceAuthHook authenticates MQTT clients via device access token and publishes telemetry to the event bus.
type DeviceAuthHook struct {
	mqtt.HookBase
	devManager *core.DeviceManager
	bus        bus.Bus
	tracker    devstate.Tracker
	rpcSvc     rpc.Service
	otaSvc     ota.Service
	server     *mqtt.Server
	logger     *slog.Logger

	mu      sync.RWMutex
	clients map[string]*model.DeviceCredentials // clientID -> creds
	devices map[string]string                   // deviceID string -> clientID
}

func (h *DeviceAuthHook) ID() string {
	return "iotp-device-auth-hook"
}

func (h *DeviceAuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
		mqtt.OnPublish,
		mqtt.OnDisconnect,
	}, []byte{b})
}

func (h *DeviceAuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	token := string(pk.Connect.Username)
	if token == "" {
		h.logger.Warn("rejected MQTT connection without token username", "client_id", cl.ID)
		return false
	}

	creds, err := h.devManager.AuthenticateToken(context.Background(), token)
	if err != nil {
		h.logger.Warn("rejected invalid MQTT device token", "client_id", cl.ID, "token", token)
		return false
	}

	h.mu.Lock()
	h.clients[cl.ID] = creds
	h.devices[creds.DeviceID.String()] = cl.ID
	h.mu.Unlock()

	if h.tracker != nil {
		h.tracker.RecordConnect(context.Background(), creds.DeviceID, time.Now().UnixMilli())
	}

	// Trigger delivery of any queued persistent RPC commands
	if h.rpcSvc != nil {
		go func(devID ids.ID) {
			time.Sleep(100 * time.Millisecond) // Let client finish subscribing
			_, _ = h.rpcSvc.DeliverQueuedRpc(context.Background(), devID)
		}(creds.DeviceID)
	}

	h.logger.Info("device authenticated successfully via MQTT",
		"device_id", creds.DeviceID, "tenant_id", creds.TenantID, "client_id", cl.ID)
	return true
}

func (h *DeviceAuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	// Permit standard device topics, OTA request topics, and gateway topics
	return strings.HasPrefix(topic, "v1/devices/me/") ||
		strings.HasPrefix(topic, "v2/fw/") ||
		strings.HasPrefix(topic, "v1/gateway/")
}

func (h *DeviceAuthHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	h.mu.RLock()
	creds, ok := h.clients[cl.ID]
	h.mu.RUnlock()

	if !ok {
		return pk, errors.New("unauthorized device publish")
	}

	if h.tracker != nil {
		h.tracker.RecordActivity(context.Background(), creds.DeviceID, time.Now().UnixMilli())
	}

	topic := pk.TopicName

	// 1. Standard Telemetry
	if topic == "v1/devices/me/telemetry" {
		msg := bus.NewMessage(creds.TenantID.String(), "DEVICE", creds.DeviceID.String(), bus.TypeTelemetryUp, pk.Payload)
		if err := h.bus.Publish(context.Background(), "telemetry.up", msg); err != nil {
			h.logger.Error("failed to publish MQTT telemetry to bus", "device_id", creds.DeviceID, "error", err)
			return pk, fmt.Errorf("bus error: %w", err)
		}
		return pk, nil
	}

	// 2. RPC Device Response (v1/devices/me/rpc/response/{id})
	if strings.HasPrefix(topic, "v1/devices/me/rpc/response/") && h.rpcSvc != nil {
		cmdIDStr := strings.TrimPrefix(topic, "v1/devices/me/rpc/response/")
		if cmdID, err := ids.Parse(cmdIDStr); err == nil {
			_ = h.rpcSvc.DeliverRpcResponse(context.Background(), creds.DeviceID, cmdID, pk.Payload)
			h.logger.Info("received MQTT RPC response from device", "device_id", creds.DeviceID, "command_id", cmdID)
		}
		return pk, nil
	}

	// 3. OTA Firmware Chunk Request (v2/fw/request/{requestId}/chunk/{chunkIndex})
	if strings.HasPrefix(topic, "v2/fw/request/") && h.otaSvc != nil {
		parts := strings.Split(strings.TrimPrefix(topic, "v2/fw/request/"), "/chunk/")
		if len(parts) == 2 {
			reqID := parts[0]
			chunkIdx, _ := strconv.Atoi(parts[1])

			// Get assigned firmware state for this device
			state, err := h.otaSvc.GetDeviceOtaState(context.Background(), creds.DeviceID, "FIRMWARE")
			if err == nil && state != nil && state.PackageID != nil {
				chunk, total, errChunk := h.otaSvc.GetPackageChunk(context.Background(), creds.TenantID, *state.PackageID, chunkIdx, 4096)
				if errChunk == nil && h.server != nil {
					respTopic := fmt.Sprintf("v2/fw/response/%s/chunk/%d", reqID, chunkIdx)
					_ = h.server.Publish(respTopic, chunk, false, 0)

					// Update progress
					if total > 0 {
						progress := float32((chunkIdx+1)*4096) / float32(total)
						if progress > 1.0 {
							progress = 1.0
						}
						_ = h.otaSvc.UpdateDeviceOtaProgress(context.Background(), creds.DeviceID, "FIRMWARE", "DOWNLOADING", progress, "")
					}
				}
			}
		}
		return pk, nil
	}

	// 4. Gateway Protocol: v1/gateway/telemetry
	if topic == "v1/gateway/telemetry" {
		var payload map[string][]map[string]any
		if err := json.Unmarshal(pk.Payload, &payload); err == nil {
			for childName, points := range payload {
				ptsBytes, _ := json.Marshal(points)
				msg := bus.NewMessage(creds.TenantID.String(), "DEVICE", childName, bus.TypeTelemetryUp, ptsBytes)
				_ = h.bus.Publish(context.Background(), "telemetry.up", msg)
			}
		}
		return pk, nil
	}

	return pk, nil
}

func (h *DeviceAuthHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	h.mu.Lock()
	creds, ok := h.clients[cl.ID]
	delete(h.clients, cl.ID)
	if ok {
		delete(h.devices, creds.DeviceID.String())
	}
	h.mu.Unlock()

	if ok && h.tracker != nil {
		h.tracker.RecordDisconnect(context.Background(), creds.DeviceID, time.Now().UnixMilli())
	}
}

// Server encapsulates the embedded mochi-mqtt broker.
type Server struct {
	server *mqtt.Server
	cfg    Config
	hook   *DeviceAuthHook
	logger *slog.Logger
}

func (s *Server) WithTracker(t devstate.Tracker) *Server {
	if s.hook != nil {
		s.hook.tracker = t
	}
	return s
}

func (s *Server) WithRpcService(svc rpc.Service) *Server {
	if s.hook != nil {
		s.hook.rpcSvc = svc
		svc.SetDispatcher(s)
	}
	return s
}

func (s *Server) WithOtaService(svc ota.Service) *Server {
	if s.hook != nil {
		s.hook.otaSvc = svc
	}
	return s
}

func NewServer(cfg Config, devManager *core.DeviceManager, b bus.Bus, logger *slog.Logger) (*Server, error) {
	if cfg.Port == 0 {
		cfg.Port = 1883
	}
	if logger == nil {
		logger = slog.Default()
	}

	s := mqtt.New(&mqtt.Options{
		InlineClient: true,
	})

	hook := &DeviceAuthHook{
		devManager: devManager,
		bus:        b,
		logger:     logger,
		server:     s,
		clients:    make(map[string]*model.DeviceCredentials),
		devices:    make(map[string]string),
	}

	if err := s.AddHook(hook, nil); err != nil {
		return nil, fmt.Errorf("mqtt: failed to add auth hook: %w", err)
	}

	addr := cfg.Addr
	if addr == "" {
		addr = fmt.Sprintf(":%d", cfg.Port)
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "t1",
		Address: addr,
	})

	if err := s.AddListener(tcp); err != nil {
		return nil, fmt.Errorf("mqtt: failed to add TCP listener: %w", err)
	}

	return &Server{
		server: s,
		cfg:    cfg,
		hook:   hook,
		logger: logger,
	}, nil
}

// DispatchRpcDownlink implements rpc.DownlinkDispatcher.
func (s *Server) DispatchRpcDownlink(ctx context.Context, deviceID ids.ID, commandID ids.ID, method string, params json.RawMessage) (bool, error) {
	s.hook.mu.RLock()
	_, isConnected := s.hook.devices[deviceID.String()]
	s.hook.mu.RUnlock()

	if !isConnected {
		return false, nil // Device is not connected; keep as QUEUED
	}

	topic := fmt.Sprintf("v1/devices/me/rpc/request/%s", commandID.String())
	payloadMap := map[string]any{
		"method": method,
		"params": params,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	err := s.server.Publish(topic, payloadBytes, false, 0)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Serve starts the MQTT broker.
func (s *Server) Serve() error {
	s.logger.Info("starting MQTT broker", "port", s.cfg.Port)
	return s.server.Serve()
}

// Close gracefully stops the broker.
func (s *Server) Close() error {
	return s.server.Close()
}
