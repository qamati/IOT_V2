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
		return e.convertExpression(conv, raw, meta)
	default:
		return nil, fmt.Errorf("unsupported converter mode: %s", conv.Mode)
	}
}

// ConvertDownlink encodes a platform downlink event into external message payload.
func (e *Engine) ConvertDownlink(conv *Converter, d *DownlinkEvent) ([]byte, error) {
	if conv == nil || d == nil {
		return nil, errs.InvalidArgument("converter and downlink event required")
	}

	// Default canonical downlink mapping
	out := map[string]any{
		"targetDevice": d.Device.Name,
		"kind":         d.Kind,
		"method":       d.Method,
		"params":       d.Params,
		"id":           d.ID,
		"timestamp":    time.Now().UnixMilli(),
	}

	return json.Marshal(out)
}

func (e *Engine) convertDeclarative(conv *Converter, raw []byte, meta map[string]string) (*UplinkEvent, error) {
	var def DeclarativeDefinition
	if len(conv.Definition) > 0 {
		if err := json.Unmarshal(conv.Definition, &def); err != nil {
			var strDef string
			if strErr := json.Unmarshal(conv.Definition, &strDef); strErr == nil {
				if err2 := json.Unmarshal([]byte(strDef), &def); err2 != nil {
					return nil, errs.InvalidArgument("invalid converter definition JSON: " + err2.Error())
				}
			} else {
				return nil, errs.InvalidArgument("invalid converter definition JSON: " + err.Error())
			}
		}
	}

	if meta == nil {
		meta = make(map[string]string)
	}

	// 1. Parse payload if format is json, csv, hex, chirpstack, ttn or default
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
	} else if def.Format == "csv" {
		line := strings.TrimSpace(string(raw))
		cols := strings.Split(line, ",")
		parsed = make(map[string]any)
		for idx, col := range cols {
			colTrim := strings.TrimSpace(col)
			if f, err := strconv.ParseFloat(colTrim, 64); err == nil {
				parsed[strconv.Itoa(idx)] = f
			} else {
				parsed[strconv.Itoa(idx)] = colTrim
			}
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

// ExpressionDefinition holds configuration for ModeExpression converters.
type ExpressionDefinition struct {
	DeclarativeDefinition
	Expressions map[string]string `json:"expressions,omitempty"` // target -> expression e.g. "telemetry.fahrenheit": "temp * 1.8 + 32"
}

func (e *Engine) convertExpression(conv *Converter, raw []byte, meta map[string]string) (*UplinkEvent, error) {
	// First perform declarative base mapping
	event, err := e.convertDeclarative(conv, raw, meta)
	if err != nil {
		return nil, err
	}

	var expDef ExpressionDefinition
	if len(conv.Definition) > 0 {
		if err := json.Unmarshal(conv.Definition, &expDef); err != nil {
			var strDef string
			if strErr := json.Unmarshal(conv.Definition, &strDef); strErr == nil {
				_ = json.Unmarshal([]byte(strDef), &expDef)
			}
		}
	}

	if len(expDef.Expressions) == 0 {
		return event, nil
	}

	// Evaluate custom expressions
	for target, exprStr := range expDef.Expressions {
		val := evalLinearExpr(exprStr, event)
		if val != nil {
			if strings.HasPrefix(target, "telemetry.") {
				k := strings.TrimPrefix(target, "telemetry.")
				if len(event.Telemetry) == 0 {
					event.Telemetry = append(event.Telemetry, TsBatch{
						Ts:     time.Now().UnixMilli(),
						Values: make(map[string]any),
					})
				}
				event.Telemetry[0].Values[k] = val
			} else if strings.HasPrefix(target, "attributes.") {
				k := strings.TrimPrefix(target, "attributes.")
				event.Attributes[k] = val
			}
		}
	}

	return event, nil
}

func evalLinearExpr(exprStr string, event *UplinkEvent) any {
	exprStr = strings.TrimSpace(exprStr)
	if exprStr == "" {
		return nil
	}

	// Tokenize simple linear expression e.g. "temp * 1.8 + 32" or "raw / 100"
	tokens := strings.Fields(exprStr)
	if len(tokens) == 1 {
		// Direct key lookup
		return lookupEventVal(tokens[0], event)
	}

	if len(tokens) >= 3 {
		leftVal := lookupEventVal(tokens[0], event)
		fVal, ok := toFloat64(leftVal)
		if !ok {
			return nil
		}

		op := tokens[1]
		rightNum, err := strconv.ParseFloat(tokens[2], 64)
		if err != nil {
			return nil
		}

		switch op {
		case "*":
			fVal *= rightNum
		case "/":
			if rightNum != 0 {
				fVal /= rightNum
			}
		case "+":
			fVal += rightNum
		case "-":
			fVal -= rightNum
		}

		// Optional chained offset e.g. "temp * 1.8 + 32" (5 tokens)
		if len(tokens) == 5 {
			op2 := tokens[3]
			rightNum2, err := strconv.ParseFloat(tokens[4], 64)
			if err == nil {
				switch op2 {
				case "+":
					fVal += rightNum2
				case "-":
					fVal -= rightNum2
				case "*":
					fVal *= rightNum2
				case "/":
					if rightNum2 != 0 {
						fVal /= rightNum2
					}
				}
			}
		}
		return fVal
	}

	return nil
}

func lookupEventVal(key string, event *UplinkEvent) any {
	if len(event.Telemetry) > 0 {
		if v, ok := event.Telemetry[0].Values[key]; ok {
			return v
		}
	}
	if v, ok := event.Attributes[key]; ok {
		return v
	}
	if v, ok := event.Meta[key]; ok {
		return v
	}
	return nil
}

