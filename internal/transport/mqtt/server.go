package mqtt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
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
	logger     *slog.Logger

	mu      sync.RWMutex
	clients map[string]*model.DeviceCredentials // clientID -> creds
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
	h.mu.Unlock()

	h.logger.Info("device authenticated successfully via MQTT",
		"device_id", creds.DeviceID, "tenant_id", creds.TenantID, "client_id", cl.ID)
	return true
}

func (h *DeviceAuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	// Only permit topics under v1/devices/me/
	return strings.HasPrefix(topic, "v1/devices/me/")
}

func (h *DeviceAuthHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if pk.TopicName != "v1/devices/me/telemetry" {
		return pk, nil
	}

	h.mu.RLock()
	creds, ok := h.clients[cl.ID]
	h.mu.RUnlock()

	if !ok {
		return pk, errors.New("unauthorized device publish")
	}

	// Dispatch to the bus and confirm durability
	msg := bus.NewMessage(creds.TenantID.String(), "DEVICE", creds.DeviceID.String(), bus.TypeTelemetryUp, pk.Payload)
	if err := h.bus.Publish(context.Background(), "telemetry.up", msg); err != nil {
		h.logger.Error("failed to publish MQTT telemetry to bus", "device_id", creds.DeviceID, "error", err)
		return pk, fmt.Errorf("bus error: %w", err)
	}

	return pk, nil
}

func (h *DeviceAuthHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	h.mu.Lock()
	delete(h.clients, cl.ID)
	h.mu.Unlock()
}

// Server encapsulates the embedded mochi-mqtt broker.
type Server struct {
	server *mqtt.Server
	cfg    Config
	logger *slog.Logger
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
		clients:    make(map[string]*model.DeviceCredentials),
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
		logger: logger,
	}, nil
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
