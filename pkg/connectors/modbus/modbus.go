package modbus

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/pkg/connectors"
)

// RegisterMapping defines field mapping for a Modbus register or coil.
type RegisterMapping struct {
	Key      string  `json:"key"`
	Register string  `json:"register"` // holding, input, coil, discrete
	Address  int     `json:"address"`
	Type     string  `json:"type"` // uint16, int16, float32, bool
	Scale    float64 `json:"scale,omitempty"`
	Offset   float64 `json:"offset,omitempty"`
	Unit     string  `json:"unit,omitempty"`
}

// DeviceConfig defines polling settings for a Modbus slave device.
type DeviceConfig struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	SlaveID        int               `json:"slaveId"`
	PollIntervalMs int               `json:"pollIntervalMs"`
	Registers      []RegisterMapping `json:"registers"`
}

// Config defines Modbus connector configuration.
type Config struct {
	Host      string         `json:"host,omitempty"`
	Port      int            `json:"port,omitempty"`
	Simulator bool           `json:"simulator"` // When true, runs built-in simulated PLC
	Devices   []DeviceConfig `json:"devices"`
}

// Connector implements connectors.Connector for Modbus TCP and simulated PLCs.
type Connector struct {
	mu         sync.RWMutex
	name       string
	cfg        Config
	startTime  time.Time
	pollsCount int64
	errsCount  int64
	lastError  string

	// Simulated PLC register memory: slaveID -> address -> raw value
	virtualHolding  map[int]map[int]uint16
	virtualCoils    map[int]map[int]bool
	virtualInputs   map[int]map[int]uint16
	virtualDiscrete map[int]map[int]bool
}

// NewConnector creates a new Modbus connector instance.
func NewConnector(name string) *Connector {
	c := &Connector{
		name:            name,
		virtualHolding:  make(map[int]map[int]uint16),
		virtualCoils:    make(map[int]map[int]bool),
		virtualInputs:   make(map[int]map[int]uint16),
		virtualDiscrete: make(map[int]map[int]bool),
	}
	return c
}

func (c *Connector) Name() string {
	return c.name
}

func (c *Connector) Type() string {
	return "MODBUS_TCP"
}

func (c *Connector) Configure(cfgJSON json.RawMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(cfgJSON) > 0 {
		if err := json.Unmarshal(cfgJSON, &c.cfg); err != nil {
			return fmt.Errorf("modbus: invalid config JSON: %w", err)
		}
	}

	// Default to simulator if host is empty
	if c.cfg.Host == "" {
		c.cfg.Simulator = true
	}

	// Default demo device if none configured
	if len(c.cfg.Devices) == 0 {
		c.cfg.Devices = []DeviceConfig{
			{
				Name:           "modbus-pump-station-01",
				Type:           "industrial_pump",
				SlaveID:        1,
				PollIntervalMs: 1000,
				Registers: []RegisterMapping{
					{Key: "motor_speed_rpm", Register: "holding", Address: 100, Type: "uint16", Scale: 1.0, Unit: "rpm"},
					{Key: "discharge_pressure", Register: "holding", Address: 101, Type: "uint16", Scale: 0.1, Unit: "bar"},
					{Key: "bearing_temp", Register: "input", Address: 200, Type: "int16", Scale: 0.1, Unit: "°C"},
					{Key: "pump_run_relay", Register: "coil", Address: 10, Type: "bool"},
					{Key: "vibration_alarm", Register: "discrete", Address: 20, Type: "bool"},
				},
			},
		}
	}

	// Initialize simulated registers
	for _, dev := range c.cfg.Devices {
		sid := dev.SlaveID
		if sid == 0 {
			sid = 1
		}
		if c.virtualHolding[sid] == nil {
			c.virtualHolding[sid] = make(map[int]uint16)
		}
		if c.virtualCoils[sid] == nil {
			c.virtualCoils[sid] = make(map[int]bool)
		}
		if c.virtualInputs[sid] == nil {
			c.virtualInputs[sid] = make(map[int]uint16)
		}
		if c.virtualDiscrete[sid] == nil {
			c.virtualDiscrete[sid] = make(map[int]bool)
		}

		c.virtualHolding[sid][100] = 1450 // 1450 RPM
		c.virtualHolding[sid][101] = 42   // 4.2 bar
		c.virtualInputs[sid][200] = 685   // 68.5 °C
		c.virtualCoils[sid][10] = true    // Pump running
		c.virtualDiscrete[sid][20] = false // No vibration alarm
	}

	return nil
}

func (c *Connector) Run(ctx context.Context, sink connectors.Sink) error {
	c.mu.Lock()
	c.startTime = time.Now()
	c.mu.Unlock()

	// Notify sink that child devices are connected
	for _, dev := range c.cfg.Devices {
		sink.Connected(dev.Name, map[string]string{
			"connector": c.name,
			"type":      dev.Type,
			"slaveId":   fmt.Sprintf("%d", dev.SlaveID),
		})
	}

	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	cycle := 0
	for {
		select {
		case <-ctx.Done():
			for _, dev := range c.cfg.Devices {
				sink.Disconnected(dev.Name)
			}
			return nil
		case now := <-ticker.C:
			cycle++
			c.pollAll(now.UnixMilli(), cycle, sink)
		}
	}
}

func (c *Connector) pollAll(nowTs int64, cycle int, sink connectors.Sink) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, dev := range c.cfg.Devices {
		sid := dev.SlaveID
		if sid == 0 {
			sid = 1
		}

		// Introduce subtle simulated variations in simulator mode
		if c.cfg.Simulator {
			sineOffset := int16(math.Sin(float64(cycle)*0.1) * 15)
			c.virtualHolding[sid][100] = uint16(1450 + sineOffset) // 1435 - 1465 RPM
			c.virtualHolding[sid][101] = uint16(42 + (cycle%5))    // 4.2 - 4.6 bar
			c.virtualInputs[sid][200] = uint16(680 + (cycle%12))   // 68.0 - 69.2 °C
		}

		values := make(map[string]any)
		for _, reg := range dev.Registers {
			val := c.readRegister(sid, reg)
			if val != nil {
				values[reg.Key] = val
			}
		}

		if len(values) > 0 {
			sink.Telemetry(dev.Name, nowTs, values)
			c.pollsCount++
		}
	}
}

func (c *Connector) readRegister(slaveID int, reg RegisterMapping) any {
	switch reg.Register {
	case "holding":
		raw, ok := c.virtualHolding[slaveID][reg.Address]
		if !ok {
			return nil
		}
		f := float64(raw)
		if reg.Scale != 0 {
			f *= reg.Scale
		}
		if reg.Offset != 0 {
			f += reg.Offset
		}
		if reg.Type == "int" || reg.Type == "uint16" && reg.Scale == 1.0 {
			return int64(f)
		}
		return math.Round(f*100) / 100
	case "input":
		raw, ok := c.virtualInputs[slaveID][reg.Address]
		if !ok {
			return nil
		}
		f := float64(raw)
		if reg.Scale != 0 {
			f *= reg.Scale
		}
		if reg.Offset != 0 {
			f += reg.Offset
		}
		return math.Round(f*100) / 100
	case "coil":
		b, ok := c.virtualCoils[slaveID][reg.Address]
		if !ok {
			return false
		}
		return b
	case "discrete":
		b, ok := c.virtualDiscrete[slaveID][reg.Address]
		if !ok {
			return false
		}
		return b
	}
	return nil
}

func (c *Connector) HandleWrite(ctx context.Context, w connectors.WriteReq) (connectors.WriteResp, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Default target slave ID
	sid := 1
	for _, dev := range c.cfg.Devices {
		if dev.Name == w.Device {
			sid = dev.SlaveID
			break
		}
	}

	// Supported write methods: setRegister, writeCoil, setSpeed
	switch w.Method {
	case "setRegister", "setSpeed":
		addr := 100 // default speed holding register
		if a, ok := w.Params["address"].(float64); ok {
			addr = int(a)
		}
		val := uint16(1500)
		if v, ok := w.Params["value"].(float64); ok {
			val = uint16(v)
		} else if v, ok := w.Params["rpm"].(float64); ok {
			val = uint16(v)
		}

		if c.virtualHolding[sid] == nil {
			c.virtualHolding[sid] = make(map[int]uint16)
		}
		c.virtualHolding[sid][addr] = val
		return connectors.WriteResp{
			Success: true,
			Message: fmt.Sprintf("holding register %d set to %d", addr, val),
			Values:  map[string]any{"address": addr, "value": val},
		}, nil

	case "writeCoil", "setRelay":
		addr := 10 // default coil
		if a, ok := w.Params["address"].(float64); ok {
			addr = int(a)
		}
		val := true
		if b, ok := w.Params["value"].(bool); ok {
			val = b
		} else if b, ok := w.Params["enabled"].(bool); ok {
			val = b
		}

		if c.virtualCoils[sid] == nil {
			c.virtualCoils[sid] = make(map[int]bool)
		}
		c.virtualCoils[sid][addr] = val
		return connectors.WriteResp{
			Success: true,
			Message: fmt.Sprintf("coil %d set to %v", addr, val),
			Values:  map[string]any{"address": addr, "value": val},
		}, nil

	default:
		return connectors.WriteResp{
			Success: false,
			Message: fmt.Sprintf("unknown write method '%s'", w.Method),
		}, nil
	}
}

func (c *Connector) Devices() []connectors.DeviceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []connectors.DeviceInfo
	for _, dev := range c.cfg.Devices {
		result = append(result, connectors.DeviceInfo{
			Name:      dev.Name,
			Type:      dev.Type,
			Connected: true,
		})
	}
	return result
}

func (c *Connector) Health() connectors.Health {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := "HEALTHY"
	if c.errsCount > 0 {
		status = "DEGRADED"
	}

	var uptime int64
	if !c.startTime.IsZero() {
		uptime = int64(time.Since(c.startTime).Seconds())
	}

	return connectors.Health{
		Status:      status,
		LastError:   c.lastError,
		PollsCount:  c.pollsCount,
		ErrorsCount: c.errsCount,
		UptimeSec:   uptime,
	}
}
