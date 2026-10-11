package integration

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/qamati/IOT_V2/pkg/errs"
)

// DeclarativeMapping defines a field-level transform.
type DeclarativeMapping struct {
	Source string  `json:"source"` // Dot-notated path in input JSON, e.g. "metrics.temperature" or "temp"
	Target string  `json:"target"` // Target in canonical event, e.g. "telemetry.temp" or "attributes.fw_version"
	Type   string  `json:"type"`   // float, int, string, bool
	Scale  float64 `json:"scale,omitempty"`
	Offset float64 `json:"offset,omitempty"`
}

// DeclarativeDefinition holds the configuration for ModeDeclarative converters.
type DeclarativeDefinition struct {
	Format         string               `json:"format"` // json, chirpstack, ttn, hex
	DeviceNamePath string               `json:"deviceNamePath"`
	DeviceNameTpl  string               `json:"deviceNameTpl,omitempty"` // e.g. "lora-${devEUI}"
	DeviceType     string               `json:"deviceType,omitempty"`
	TimestampPath  string               `json:"timestampPath,omitempty"`
	Mappings       []DeclarativeMapping `json:"mappings,omitempty"`
}

// Engine executes converter transformations.
type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

// Convert processes an incoming raw message and metadata into a canonical UplinkEvent.
func (e *Engine) Convert(conv *Converter, raw []byte, meta map[string]string) (*UplinkEvent, error) {
	if conv == nil {
		return nil, errs.InvalidArgument("nil converter provided")
	}

	switch conv.Mode {
	case ModeDeclarative, "":
		return e.convertDeclarative(conv, raw, meta)
	case ModeExpression:
		// Expression mode uses declarative fallback with expression mappings
		return e.convertDeclarative(conv, raw, meta)
	default:
		return nil, fmt.Errorf("unsupported converter mode: %s", conv.Mode)
	}
}

func (e *Engine) convertDeclarative(conv *Converter, raw []byte, meta map[string]string) (*UplinkEvent, error) {
	var def DeclarativeDefinition
	if len(conv.Definition) > 0 {
		if err := json.Unmarshal(conv.Definition, &def); err != nil {
			return nil, errs.InvalidArgument("invalid converter definition JSON: " + err.Error())
		}
	}

	if meta == nil {
		meta = make(map[string]string)
	}

	// 1. Parse JSON if format is json, chirpstack, ttn or default
	var parsed map[string]any
	if def.Format == "hex" {
		// Hex string conversion
		hexStr := strings.TrimSpace(string(raw))
		b, err := hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex payload: %w", err)
		}
		parsed = map[string]any{
			"hex":   hexStr,
			"bytes": b,
			"len":   len(b),
		}
	} else {
		// Try parsing as JSON map
		if err := json.Unmarshal(raw, &parsed); err != nil {
			// If not a JSON object, maybe a raw string or number
			parsed = map[string]any{"raw": string(raw)}
		}
	}

	// 2. Resolve Device Name
	deviceName := e.resolveDeviceName(def, parsed, meta)
	if deviceName == "" {
		deviceName = "unknown-device"
	}

	devType := def.DeviceType
	if devType == "" {
		devType = "generic"
	}

	// 3. Resolve Timestamp
	ts := time.Now().UnixMilli()
	if def.TimestampPath != "" {
		if val := extractPath(parsed, def.TimestampPath); val != nil {
			if tsNum, ok := toInt64(val); ok && tsNum > 0 {
				// Convert seconds to ms if epoch seconds
				if tsNum < 10000000000 {
					tsNum *= 1000
				}
				ts = tsNum
			}
		}
	}

	event := &UplinkEvent{
		Device: DeviceRef{
			Name: deviceName,
			Type: devType,
		},
		Telemetry:  []TsBatch{},
		Attributes: make(map[string]any),
		Server:     make(map[string]any),
		Meta:       make(map[string]string),
		Raw:        string(raw),
	}

	for k, v := range meta {
		event.Meta[k] = v
	}

	// 4. Handle specialized formats
	if def.Format == "chirpstack" {
		e.applyChirpstack(parsed, event)
	} else if def.Format == "ttn" {
		e.applyTTN(parsed, event)
	}

	// 5. Apply explicit mappings
	telemetryValues := make(map[string]any)
	for _, m := range def.Mappings {
		val := extractPath(parsed, m.Source)
		if val == nil {
			continue
		}

		transformed := transformValue(val, m.Type, m.Scale, m.Offset)
		if transformed == nil {
			continue
		}

		if strings.HasPrefix(m.Target, "telemetry.") {
			key := strings.TrimPrefix(m.Target, "telemetry.")
			telemetryValues[key] = transformed
		} else if strings.HasPrefix(m.Target, "attributes.") {
			key := strings.TrimPrefix(m.Target, "attributes.")
			event.Attributes[key] = transformed
		} else if strings.HasPrefix(m.Target, "server.") {
			key := strings.TrimPrefix(m.Target, "server.")
			event.Server[key] = transformed
		}
	}

	// If no explicit mappings were configured for JSON, auto-map top-level primitives to telemetry
	if len(def.Mappings) == 0 && def.Format != "chirpstack" && def.Format != "ttn" {
		for k, v := range parsed {
			if k == "deviceName" || k == "device_name" || k == "ts" || k == "timestamp" {
				continue
			}
			switch val := v.(type) {
			case float64, int, int64, string, bool:
				telemetryValues[k] = val
			}
		}
	}

	if len(telemetryValues) > 0 {
		event.Telemetry = append(event.Telemetry, TsBatch{
			Ts:     ts,
			Values: telemetryValues,
		})
	}

	return event, nil
}

func (e *Engine) resolveDeviceName(def DeclarativeDefinition, parsed map[string]any, meta map[string]string) string {
	if def.DeviceNamePath != "" {
		if val := extractPath(parsed, def.DeviceNamePath); val != nil {
			s := fmt.Sprint(val)
			if s != "" {
				return s
			}
		}
	}

	if def.DeviceNameTpl != "" {
		res := def.DeviceNameTpl
		for k, v := range meta {
			res = strings.ReplaceAll(res, "${meta."+k+"}", v)
		}
		for k, v := range parsed {
			res = strings.ReplaceAll(res, "${"+k+"}", fmt.Sprint(v))
		}
		return res
	}

	// Common standard fields
	for _, k := range []string{"deviceName", "device_name", "devEUI", "devEui", "deviceId", "device_id", "id"} {
		if val := extractPath(parsed, k); val != nil {
			s := fmt.Sprint(val)
			if s != "" {
				return s
			}
		}
	}

	if devName, ok := meta["device_name"]; ok && devName != "" {
		return devName
	}
	return ""
}

func (e *Engine) applyChirpstack(parsed map[string]any, event *UplinkEvent) {
	// ChirpStack: { "deviceInfo": { "deviceName": "...", "devEui": "..." }, "object": { "temp": 24.5 }, "rxInfo": [...] }
	if obj, ok := parsed["object"].(map[string]any); ok {
		ts := time.Now().UnixMilli()
		if len(event.Telemetry) > 0 {
			ts = event.Telemetry[0].Ts
		}
		vals := make(map[string]any)
		for k, v := range obj {
			vals[k] = v
		}
		event.Telemetry = []TsBatch{{Ts: ts, Values: vals}}
	} else if dataStr, ok := parsed["data"].(string); ok {
		// raw base64 data
		if b, err := base64.StdEncoding.DecodeString(dataStr); err == nil {
			event.Meta["raw_hex"] = hex.EncodeToString(b)
		}
	}

	if rxInfo, ok := parsed["rxInfo"].([]any); ok && len(rxInfo) > 0 {
		if rx0, ok := rxInfo[0].(map[string]any); ok {
			if rssi, ok := rx0["rssi"]; ok {
				event.Attributes["rssi"] = rssi
			}
			if snr, ok := rx0["snr"]; ok {
				event.Attributes["snr"] = snr
			}
		}
	}

	if fPort, ok := parsed["fPort"]; ok {
		event.Attributes["fPort"] = fPort
	}
}

func (e *Engine) applyTTN(parsed map[string]any, event *UplinkEvent) {
	// TTN: { "uplink_message": { "decoded_payload": { ... }, "rx_metadata": [...] } }
	if upMsg, ok := parsed["uplink_message"].(map[string]any); ok {
		if decoded, ok := upMsg["decoded_payload"].(map[string]any); ok {
			ts := time.Now().UnixMilli()
			vals := make(map[string]any)
			for k, v := range decoded {
				vals[k] = v
			}
			event.Telemetry = []TsBatch{{Ts: ts, Values: vals}}
		}
		if fPort, ok := upMsg["f_port"]; ok {
			event.Attributes["fPort"] = fPort
		}
	}
}

// extractPath retrieves a nested value by dot notation (e.g., "sensors.temperature")
func extractPath(data any, path string) any {
	if data == nil || path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch node := current.(type) {
		case map[string]any:
			v, exists := node[part]
			if !exists {
				return nil
			}
			current = v
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil
			}
			current = node[idx]
		default:
			return nil
		}
	}
	return current
}

func transformValue(val any, targetType string, scale, offset float64) any {
	switch strings.ToLower(targetType) {
	case "float", "number":
		num, ok := toFloat64(val)
		if !ok {
			return nil
		}
		if scale != 0 {
			num *= scale
		}
		if offset != 0 {
			num += offset
		}
		return num
	case "int":
		num, ok := toFloat64(val)
		if !ok {
			return nil
		}
		if scale != 0 {
			num *= scale
		}
		if offset != 0 {
			num += offset
		}
		return int64(num)
	case "string":
		return fmt.Sprint(val)
	case "bool":
		switch v := val.(type) {
		case bool:
			return v
		case string:
			b, _ := strconv.ParseBool(v)
			return b
		case float64:
			return v != 0
		case int:
			return v != 0
		}
	}
	return val
}

func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	}
	return 0, false
}

func toInt64(val any) (int64, bool) {
	switch v := val.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return i, err == nil
	}
	return 0, false
}
