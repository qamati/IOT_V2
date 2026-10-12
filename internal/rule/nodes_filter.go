package rule

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// --- filter.msg_type_switch ---

type MsgTypeSwitchNode struct{}

func (n *MsgTypeSwitchNode) Init(cfg json.RawMessage, env Env) error {
	return nil
}

func (n *MsgTypeSwitchNode) OnMsg(ctx NodeCtx, msg *Message) {
	switch msg.Type {
	case TypePostTelemetry, "POST_TELEMETRY":
		ctx.TellNext(msg, RelationPostTelemetry)
	case TypePostAttributes, "POST_ATTRIBUTES", TypeAttributesUpdated:
		ctx.TellNext(msg, RelationPostAttributes)
	case TypeActivityEvent:
		ctx.TellNext(msg, RelationActivityEvent)
	case TypeInactivityEvent:
		ctx.TellNext(msg, RelationInactivityEvent)
	default:
		ctx.TellNext(msg, RelationOther)
	}
}

func (n *MsgTypeSwitchNode) Close() {}

// --- filter.script ---

type FilterScriptConfig struct {
	Field      string   `json:"field,omitempty"`
	Op         string   `json:"op,omitempty"` // >, <, >=, <=, ==, !=
	Value      *float64 `json:"value,omitempty"`
	StringVal  *string  `json:"stringValue,omitempty"`
	Expression string   `json:"expression,omitempty"` // e.g. "temperature > 45"
}

type FilterScriptNode struct {
	cfg FilterScriptConfig
}

func (n *FilterScriptNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &n.cfg); err != nil {
			return fmt.Errorf("invalid filter.script config: %w", err)
		}
	}
	// Parse simple expression if provided (e.g. "temperature > 45" or "msg.temperature > 45")
	if n.cfg.Expression != "" && n.cfg.Field == "" {
		parts := strings.Fields(n.cfg.Expression)
		if len(parts) >= 3 {
			field := strings.TrimPrefix(parts[0], "msg.")
			n.cfg.Field = strings.TrimPrefix(field, "$[")
			n.cfg.Field = strings.TrimSuffix(n.cfg.Field, "]")
			n.cfg.Op = parts[1]
			if val, err := strconv.ParseFloat(parts[2], 64); err == nil {
				n.cfg.Value = &val
			} else {
				str := strings.Trim(parts[2], `"'`)
				n.cfg.StringVal = &str
			}
		}
	}
	return nil
}

func (n *FilterScriptNode) OnMsg(ctx NodeCtx, msg *Message) {
	matched, err := n.evaluate(msg)
	if err != nil {
		ctx.TellNext(msg, RelationFailure)
		return
	}
	if matched {
		ctx.TellNext(msg, RelationTrue)
	} else {
		ctx.TellNext(msg, RelationFalse)
	}
}

func (n *FilterScriptNode) evaluate(msg *Message) (bool, error) {
	if n.cfg.Field == "" {
		return true, nil
	}

	var dataMap map[string]any
	if err := json.Unmarshal(msg.Data, &dataMap); err != nil {
		var list []map[string]any
		if errList := json.Unmarshal(msg.Data, &list); errList == nil {
			for _, item := range list {
				if k, ok := item["key"].(string); ok && k == n.cfg.Field {
					return n.checkValue(item["value"])
				}
			}
		}
		if val, ok := msg.Metadata[n.cfg.Field]; ok {
			if fVal, err := strconv.ParseFloat(val, 64); err == nil {
				return n.checkNumber(fVal)
			}
			return n.checkString(val), nil
		}
		return false, fmt.Errorf("failed to parse message payload: %w", err)
	}

	val, ok := dataMap[n.cfg.Field]
	if !ok {
		if strVal, ok := msg.Metadata[n.cfg.Field]; ok {
			if fVal, err := strconv.ParseFloat(strVal, 64); err == nil {
				return n.checkNumber(fVal)
			}
			return n.checkString(strVal), nil
		}
		return false, nil
	}

	return n.checkValue(val)
}

func (n *FilterScriptNode) checkValue(val any) (bool, error) {
	switch v := val.(type) {
	case float64:
		return n.checkNumber(v)
	case int:
		return n.checkNumber(float64(v))
	case int64:
		return n.checkNumber(float64(v))
	case string:
		if fVal, err := strconv.ParseFloat(v, 64); err == nil && n.cfg.Value != nil {
			return n.checkNumber(fVal)
		}
		return n.checkString(v), nil
	case bool:
		if n.cfg.StringVal != nil {
			return fmt.Sprintf("%v", v) == *n.cfg.StringVal, nil
		}
		return v, nil
	default:
		return false, nil
	}
}

func (n *FilterScriptNode) checkNumber(v float64) (bool, error) {
	if n.cfg.Value == nil {
		return false, nil
	}
	target := *n.cfg.Value
	switch n.cfg.Op {
	case ">":
		return v > target, nil
	case "<":
		return v < target, nil
	case ">=":
		return v >= target, nil
	case "<=":
		return v <= target, nil
	case "==", "=":
		return v == target, nil
	case "!=":
		return v != target, nil
	default:
		return v > target, nil
	}
}

func (n *FilterScriptNode) checkString(v string) bool {
	if n.cfg.StringVal == nil {
		return false
	}
	target := *n.cfg.StringVal
	switch n.cfg.Op {
	case "!=":
		return v != target
	default:
		return v == target
	}
}

func (n *FilterScriptNode) Close() {}

// --- filter.msg_type ---

type FilterMsgTypeConfig struct {
	MessageTypes []string `json:"messageTypes"`
}

type FilterMsgTypeNode struct {
	cfg FilterMsgTypeConfig
}

func (n *FilterMsgTypeNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	return nil
}

func (n *FilterMsgTypeNode) OnMsg(ctx NodeCtx, msg *Message) {
	for _, t := range n.cfg.MessageTypes {
		if strings.EqualFold(t, msg.Type) {
			ctx.TellNext(msg, RelationTrue)
			return
		}
	}
	ctx.TellNext(msg, RelationFalse)
}

func (n *FilterMsgTypeNode) Close() {}

// --- filter.alarm_status ---

type AlarmStatusFilterConfig struct {
	Statuses []string `json:"statuses"` // ACTIVE_UNACK, ACTIVE_ACK, CLEARED_UNACK, CLEARED_ACK
}

type AlarmStatusFilterNode struct {
	cfg AlarmStatusFilterConfig
}

func (n *AlarmStatusFilterNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	return nil
}

func (n *AlarmStatusFilterNode) OnMsg(ctx NodeCtx, msg *Message) {
	status := msg.Metadata["alarmStatus"]
	if status == "" {
		status = msg.Metadata["status"]
	}
	for _, s := range n.cfg.Statuses {
		if strings.EqualFold(s, status) {
			ctx.TellNext(msg, RelationTrue)
			return
		}
	}
	ctx.TellNext(msg, RelationFalse)
}

func (n *AlarmStatusFilterNode) Close() {}

// --- filter.asset_profile_switch ---

type AssetProfileSwitchNode struct{}

func (n *AssetProfileSwitchNode) Init(cfg json.RawMessage, env Env) error { return nil }
func (n *AssetProfileSwitchNode) OnMsg(ctx NodeCtx, msg *Message) {
	profile := msg.Metadata["assetProfile"]
	if profile == "" {
		profile = msg.Metadata["assetProfileName"]
	}
	if profile == "" {
		profile = "default"
	}
	ctx.TellNext(msg, profile)
}
func (n *AssetProfileSwitchNode) Close() {}

// --- filter.check_fields_presence ---

type CheckFieldsPresenceConfig struct {
	Fields []string `json:"fields"`
}

type CheckFieldsPresenceNode struct {
	cfg CheckFieldsPresenceConfig
}

func (n *CheckFieldsPresenceNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	return nil
}

func (n *CheckFieldsPresenceNode) OnMsg(ctx NodeCtx, msg *Message) {
	var dataMap map[string]any
	_ = json.Unmarshal(msg.Data, &dataMap)

	for _, field := range n.cfg.Fields {
		_, inData := dataMap[field]
		_, inMeta := msg.Metadata[field]
		if !inData && !inMeta {
			ctx.TellNext(msg, RelationFalse)
			return
		}
	}
	ctx.TellNext(msg, RelationTrue)
}

func (n *CheckFieldsPresenceNode) Close() {}

// --- filter.check_relation_presence ---

type CheckRelationPresenceNode struct{}

func (n *CheckRelationPresenceNode) Init(cfg json.RawMessage, env Env) error { return nil }
func (n *CheckRelationPresenceNode) OnMsg(ctx NodeCtx, msg *Message) {
	if _, ok := msg.Metadata["relationType"]; ok {
		ctx.TellNext(msg, RelationTrue)
	} else {
		ctx.TellNext(msg, RelationFalse)
	}
}
func (n *CheckRelationPresenceNode) Close() {}

// --- filter.device_profile_switch ---

type DeviceProfileSwitchNode struct{}

func (n *DeviceProfileSwitchNode) Init(cfg json.RawMessage, env Env) error { return nil }
func (n *DeviceProfileSwitchNode) OnMsg(ctx NodeCtx, msg *Message) {
	profile := msg.Metadata["deviceProfile"]
	if profile == "" {
		profile = msg.Metadata["deviceProfileName"]
	}
	if profile == "" {
		profile = "default"
	}
	ctx.TellNext(msg, profile)
}
func (n *DeviceProfileSwitchNode) Close() {}

// --- filter.entity_type_filter ---

type EntityTypeFilterConfig struct {
	EntityTypes []string `json:"entityTypes"`
}

type EntityTypeFilterNode struct {
	cfg EntityTypeFilterConfig
}

func (n *EntityTypeFilterNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	return nil
}

func (n *EntityTypeFilterNode) OnMsg(ctx NodeCtx, msg *Message) {
	entType := msg.Originator.EntityType
	for _, t := range n.cfg.EntityTypes {
		if strings.EqualFold(t, entType) {
			ctx.TellNext(msg, RelationTrue)
			return
		}
	}
	ctx.TellNext(msg, RelationFalse)
}

func (n *EntityTypeFilterNode) Close() {}

// --- filter.entity_type_switch ---

type EntityTypeSwitchNode struct{}

func (n *EntityTypeSwitchNode) Init(cfg json.RawMessage, env Env) error { return nil }
func (n *EntityTypeSwitchNode) OnMsg(ctx NodeCtx, msg *Message) {
	entType := msg.Originator.EntityType
	if entType == "" {
		entType = "DEVICE"
	}
	ctx.TellNext(msg, strings.Title(strings.ToLower(entType)))
}
func (n *EntityTypeSwitchNode) Close() {}

// --- filter.gps_geofencing_filter ---

type GpsGeofencingFilterConfig struct {
	LatitudeKey  string  `json:"latitudeKey,omitempty"`
	LongitudeKey string  `json:"longitudeKey,omitempty"`
	MinLat       float64 `json:"minLat"`
	MaxLat       float64 `json:"maxLat"`
	MinLon       float64 `json:"minLon"`
	MaxLon       float64 `json:"maxLon"`
}

type GpsGeofencingFilterNode struct {
	cfg GpsGeofencingFilterConfig
}

func (n *GpsGeofencingFilterNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	if n.cfg.LatitudeKey == "" {
		n.cfg.LatitudeKey = "latitude"
	}
	if n.cfg.LongitudeKey == "" {
		n.cfg.LongitudeKey = "longitude"
	}
	return nil
}

func (n *GpsGeofencingFilterNode) OnMsg(ctx NodeCtx, msg *Message) {
	var data map[string]any
	_ = json.Unmarshal(msg.Data, &data)

	latVal, hasLat := data[n.cfg.LatitudeKey]
	lonVal, hasLon := data[n.cfg.LongitudeKey]

	if !hasLat || !hasLon {
		ctx.TellNext(msg, RelationFalse)
		return
	}

	lat, ok1 := latVal.(float64)
	lon, ok2 := lonVal.(float64)
	if !ok1 || !ok2 {
		ctx.TellNext(msg, RelationFalse)
		return
	}

	if lat >= n.cfg.MinLat && lat <= n.cfg.MaxLat && lon >= n.cfg.MinLon && lon <= n.cfg.MaxLon {
		ctx.TellNext(msg, RelationTrue)
	} else {
		ctx.TellNext(msg, RelationFalse)
	}
}

func (n *GpsGeofencingFilterNode) Close() {}

// --- filter.switch ---

type SwitchNode struct{}

func (n *SwitchNode) Init(cfg json.RawMessage, env Env) error { return nil }
func (n *SwitchNode) OnMsg(ctx NodeCtx, msg *Message) {
	if rel, ok := msg.Metadata["route"]; ok && rel != "" {
		ctx.TellNext(msg, rel)
	} else {
		ctx.TellNext(msg, RelationOther)
	}
}
func (n *SwitchNode) Close() {}

func registerFilterNodes(c *Catalog) {
	// Exact ThingsBoard Filter Catalog Nodes
	c.Register(Descriptor{
		Type:         "filter.alarm_status",
		Name:         "alarm status filter",
		Category:     "FILTER",
		Relations:    []string{RelationTrue, RelationFalse},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Filters messages by alarm status (ACTIVE_UNACK, ACTIVE_ACK, etc.)",
	}, func() Node { return &AlarmStatusFilterNode{} })

	c.Register(Descriptor{
		Type:         "filter.asset_profile_switch",
		Name:         "asset profile switch",
		Category:     "FILTER",
		Relations:    []string{"default", "building", "vehicle", "other"},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Routes messages based on the asset profile name",
	}, func() Node { return &AssetProfileSwitchNode{} })

	c.Register(Descriptor{
		Type:         "filter.check_fields_presence",
		Name:         "check fields presence",
		Category:     "FILTER",
		Relations:    []string{RelationTrue, RelationFalse},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Checks whether specified fields exist in telemetry data or metadata",
	}, func() Node { return &CheckFieldsPresenceNode{} })

	c.Register(Descriptor{
		Type:         "filter.check_relation_presence",
		Name:         "check relation presence",
		Category:     "FILTER",
		Relations:    []string{RelationTrue, RelationFalse},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Checks whether a relation exists between the message originator and target entity",
	}, func() Node { return &CheckRelationPresenceNode{} })

	c.Register(Descriptor{
		Type:         "filter.device_profile_switch",
		Name:         "device profile switch",
		Category:     "FILTER",
		Relations:    []string{"default", "sensor", "meter", "gateway", "other"},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Routes messages based on the device profile name",
	}, func() Node { return &DeviceProfileSwitchNode{} })

	c.Register(Descriptor{
		Type:         "filter.entity_type_filter",
		Name:         "entity type filter",
		Category:     "FILTER",
		Relations:    []string{RelationTrue, RelationFalse},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Filters messages based on originator entity type (DEVICE, ASSET, CUSTOMER, etc.)",
	}, func() Node { return &EntityTypeFilterNode{} })

	c.Register(Descriptor{
		Type:         "filter.entity_type_switch",
		Name:         "entity type switch",
		Category:     "FILTER",
		Relations:    []string{"Device", "Asset", "Customer", "Tenant", "Other"},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Routes messages to relation matching the originator entity type",
	}, func() Node { return &EntityTypeSwitchNode{} })

	c.Register(Descriptor{
		Type:         "filter.gps_geofencing_filter",
		Name:         "gps geofencing filter",
		Category:     "FILTER",
		Relations:    []string{RelationTrue, RelationFalse},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Checks whether GPS coordinates (latitude, longitude) fall inside configured boundary",
	}, func() Node { return &GpsGeofencingFilterNode{} })

	c.Register(Descriptor{
		Type:         "filter.msg_type",
		Name:         "message type filter",
		Category:     "FILTER",
		Relations:    []string{RelationTrue, RelationFalse},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Checks if the incoming message type matches allowed message types",
	}, func() Node { return &FilterMsgTypeNode{} })

	c.Register(Descriptor{
		Type:         "filter.msg_type_switch",
		Name:         "message type switch",
		Category:     "FILTER",
		Relations:    []string{RelationPostTelemetry, RelationPostAttributes, RelationActivityEvent, RelationInactivityEvent, RelationOther},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Routes messages based on type: Post telemetry, Post attributes, Activity Event, etc.",
	}, func() Node { return &MsgTypeSwitchNode{} })

	c.Register(Descriptor{
		Type:      "filter.script",
		Name:      "script",
		Category:  "FILTER",
		Relations: []string{RelationTrue, RelationFalse, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"expression": {"type": "string", "title": "Expression", "default": "temperature > 45"},
				"field": {"type": "string", "title": "Field Name"},
				"op": {"type": "string", "title": "Operator", "enum": [">", "<", ">=", "<=", "==", "!="]},
				"value": {"type": "number", "title": "Threshold Value"}
			}
		}`),
		Version: 1,
		Docs:    "Evaluates JavaScript or mathematical expression and routes to True, False, or Failure",
	}, func() Node { return &FilterScriptNode{} })

	c.Register(Descriptor{
		Type:         "filter.switch",
		Name:         "switch",
		Category:     "FILTER",
		Relations:    []string{"Low", "Normal", "High", "Critical", "Other"},
		ConfigSchema: json.RawMessage(`{"type":"object"}`),
		Version:      1,
		Docs:         "Multi-relation condition switch routing to designated relation",
	}, func() Node { return &SwitchNode{} })
}
