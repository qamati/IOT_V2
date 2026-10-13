package scada

import (
	"time"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// DefaultSymbols returns the out-of-the-box ThingsBoard-compatible industrial SVG symbols.
func DefaultSymbols() []*Symbol {
	now := time.Now().UnixMilli()

	return []*Symbol{
		{
			ID:       ids.MustNew(),
			Name:     "Centrifugal Process Pump",
			Category: "Pumps",
			Style:    "standard",
			SVGContent: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 140" data-iotp-symbol="1">
  <defs>
    <filter id="symGlow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="3" result="blur" />
      <feComposite in="SourceGraphic" in2="blur" operator="over" />
    </filter>
  </defs>
  <g id="pump-discharge" transform="translate(100, 10)">
    <rect x="-8" y="0" width="16" height="35" rx="2" fill="#475569" stroke="#64748b" stroke-width="2" />
    <path d="M -14 0 L 14 0" stroke="#94a3b8" stroke-width="4" stroke-linecap="round" />
  </g>
  <g id="pump-base" transform="translate(80, 125)">
    <rect x="-45" y="0" width="90" height="12" rx="3" fill="#334155" stroke="#475569" stroke-width="2" />
  </g>
  <g id="pump-body" transform="translate(80, 75)">
    <circle id="volute" cx="0" cy="0" r="42" fill="#1e293b" stroke="#06b6d4" stroke-width="3" filter="url(#symGlow)" />
    <g id="impeller">
      <circle cx="0" cy="0" r="12" fill="#0f172a" stroke="#cbd5e1" stroke-width="2" />
      <path d="M 0 -28 L 0 -12" stroke="#e2e8f0" stroke-width="3" stroke-linecap="round" />
      <path d="M 0 12 L 0 28" stroke="#e2e8f0" stroke-width="3" stroke-linecap="round" />
      <path d="M -28 0 L -12 0" stroke="#e2e8f0" stroke-width="3" stroke-linecap="round" />
      <path d="M 12 0 L 28 0" stroke="#e2e8f0" stroke-width="3" stroke-linecap="round" />
      <circle cx="0" cy="0" r="5" fill="#38bdf8" />
    </g>
  </g>
  <text id="pump-label" x="80" y="105" text-anchor="middle" fill="#f8fafc" font-size="11" font-weight="700">P-101</text>
</svg>`,
			Tags: []SymbolTag{
				{Name: "body", Role: "shape", ElementID: "volute"},
				{Name: "impeller", Role: "shape", ElementID: "impeller"},
				{Name: "label", Role: "text", ElementID: "pump-label"},
			},
			Properties: []SymbolProperty{
				{ID: "runColor", Type: "color", Default: "#10b981", Label: "Running Color"},
				{ID: "stopColor", Type: "color", Default: "#64748b", Label: "Stopped Color"},
				{ID: "faultColor", Type: "color", Default: "#f43f5e", Label: "Fault Color"},
				{ID: "title", Type: "string", Default: "P-101", Label: "Equipment Tag"},
			},
			Behaviors: []SymbolBehavior{
				{ID: "running", Kind: "value", ValueType: "BOOL", Label: "Running State"},
				{ID: "speed", Kind: "value", ValueType: "DOUBLE", Label: "Impeller RPM (%)"},
				{ID: "fault", Kind: "value", ValueType: "BOOL", Label: "Trip / Fault"},
				{ID: "start", Kind: "action", Label: "Start Pump", Control: map[string]any{"sbo": true, "confirm": "Confirm Starting Pump?"}},
				{ID: "stop", Kind: "action", Label: "Stop Pump", Control: map[string]any{"sbo": true, "confirm": "Confirm Stopping Pump?"}},
				{ID: "openFaceplate", Kind: "widgetAction", Label: "Open Faceplate"},
			},
			Animations: []SymbolAnimation{
				{Target: "volute", When: "fault", Set: map[string]any{"stroke": "prop:faultColor", "fill": "rgba(244,63,94,0.2)"}},
				{Target: "volute", When: "running && !fault", Set: map[string]any{"stroke": "prop:runColor", "fill": "rgba(16,185,129,0.15)"}},
				{Target: "volute", When: "!running && !fault", Set: map[string]any{"stroke": "prop:stopColor", "fill": "rgba(100,116,139,0.15)"}},
				{Target: "impeller", When: "running", Rotate: map[string]any{"speedFactor": 1.0}},
			},
			Version:   1,
			CreatedAt: now,
		},
		{
			ID:       ids.MustNew(),
			Name:     "Modulating Control Valve",
			Category: "Valves",
			Style:    "standard",
			SVGContent: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 140" data-iotp-symbol="1">
  <defs>
    <linearGradient id="valveGrad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stopColor="#0ea5e9" stopOpacity="0.8" />
      <stop offset="100%" stopColor="#0284c7" stopOpacity="0.95" />
    </linearGradient>
  </defs>
  <!-- Diaphragm Actuator -->
  <g id="actuator" transform="translate(80, 25)">
    <path d="M -30 0 C -30 -16 30 -16 30 0 Z" fill="#334155" stroke="#64748b" stroke-width="2" />
    <line x1="0" y1="0" x2="0" y2="35" stroke="#94a3b8" stroke-width="4" />
    <rect x="-18" y="10" width="36" height="15" rx="3" fill="#1e293b" stroke="#06b6d4" stroke-width="1.5" />
    <text id="pos-text" x="0" y="21" text-anchor="middle" fill="#38bdf8" font-size="9" font-weight="800">100%</text>
  </g>
  <!-- Valve Body -->
  <g id="valve-body" transform="translate(80, 85)">
    <polygon points="-40,-25 -40,25 0,0" fill="url(#valveGrad)" stroke="#38bdf8" stroke-width="2" />
    <polygon points="40,-25 40,25 0,0" fill="url(#valveGrad)" stroke="#38bdf8" stroke-width="2" />
    <circle cx="0" cy="0" r="7" fill="#f8fafc" stroke="#0f172a" stroke-width="2" />
  </g>
  <text id="valve-label" x="80" y="125" text-anchor="middle" fill="#94a3b8" font-size="11" font-weight="700">FCV-101</text>
</svg>`,
			Tags: []SymbolTag{
				{Name: "body", Role: "shape", ElementID: "valve-body"},
				{Name: "actuator", Role: "shape", ElementID: "actuator"},
				{Name: "positionText", Role: "text", ElementID: "pos-text"},
				{Name: "label", Role: "text", ElementID: "valve-label"},
			},
			Properties: []SymbolProperty{
				{ID: "openColor", Type: "color", Default: "#10b981", Label: "Open Color"},
				{ID: "closedColor", Type: "color", Default: "#64748b", Label: "Closed Color"},
				{ID: "title", Type: "string", Default: "FCV-101", Label: "Valve Tag"},
			},
			Behaviors: []SymbolBehavior{
				{ID: "position", Kind: "value", ValueType: "DOUBLE", Label: "Stem Position (0-100%)"},
				{ID: "state", Kind: "value", ValueType: "STRING", Label: "State (OPEN/CLOSED/THROTTLE)"},
				{ID: "setpoint", Kind: "action", Label: "Set Position %", Control: map[string]any{"sbo": true}},
				{ID: "openFaceplate", Kind: "widgetAction", Label: "Open Faceplate"},
			},
			Animations: []SymbolAnimation{
				{Target: "pos-text", When: "position >= 0", Set: map[string]any{"text": "val:position + '%'"}},
			},
			Version:   1,
			CreatedAt: now,
		},
		{
			ID:       ids.MustNew(),
			Name:     "Process Storage Tank",
			Category: "Tanks",
			Style:    "standard",
			SVGContent: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 180" data-iotp-symbol="1">
  <defs>
    <linearGradient id="tankLiquidGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stopColor="#06b6d4" stopOpacity="0.85" />
      <stop offset="100%" stopColor="#0284c7" stopOpacity="0.95" />
    </linearGradient>
  </defs>
  <!-- Tank Shell -->
  <rect id="tank-shell" x="20" y="25" width="120" height="130" rx="8" fill="rgba(30, 41, 59, 0.7)" stroke="#64748b" stroke-width="2.5" />
  <!-- Liquid Level -->
  <rect id="liquid-level" x="24" y="55" width="112" height="96" rx="4" fill="url(#tankLiquidGrad)" />
  <!-- Hi-Hi Threshold Line -->
  <line id="hihi-line" x1="20" y1="38" x2="140" y2="38" stroke="#f43f5e" stroke-width="2" stroke-dasharray="4 3" />
  <text x="80" y="18" text-anchor="middle" fill="#94a3b8" font-size="11" font-weight="700">TK-101</text>
  <text id="level-text" x="80" y="105" text-anchor="middle" fill="#ffffff" font-size="16" font-weight="800">76.4 %</text>
</svg>`,
			Tags: []SymbolTag{
				{Name: "shell", Role: "shape", ElementID: "tank-shell"},
				{Name: "liquid", Role: "shape", ElementID: "liquid-level"},
				{Name: "hiHiLine", Role: "shape", ElementID: "hihi-line"},
				{Name: "levelText", Role: "text", ElementID: "level-text"},
			},
			Properties: []SymbolProperty{
				{ID: "liquidColor", Type: "color", Default: "#06b6d4", Label: "Liquid Color"},
				{ID: "hiHiLimit", Type: "number", Default: 90.0, Label: "Hi-Hi Limit %"},
				{ID: "title", Type: "string", Default: "TK-101", Label: "Tank Tag"},
			},
			Behaviors: []SymbolBehavior{
				{ID: "level", Kind: "value", ValueType: "DOUBLE", Label: "Vessel Level %"},
				{ID: "hiHiAlarm", Kind: "value", ValueType: "BOOL", Label: "Hi-Hi Process Alarm"},
				{ID: "openFaceplate", Kind: "widgetAction", Label: "Open Faceplate"},
			},
			Animations: []SymbolAnimation{
				{Target: "liquid-level", When: "level >= 0", Set: map[string]any{"height": "level * 1.25"}},
				{Target: "level-text", When: "level >= 0", Set: map[string]any{"text": "val:level + ' %'"}},
			},
			Version:   1,
			CreatedAt: now,
		},
		{
			ID:       ids.MustNew(),
			Name:     "Electromagnetic Flowmeter",
			Category: "Meters",
			Style:    "standard",
			SVGContent: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 120" data-iotp-symbol="1">
  <g id="flowmeter-flanges">
    <rect x="25" y="40" width="12" height="40" rx="2" fill="#475569" stroke="#64748b" stroke-width="2" />
    <rect x="123" y="40" width="12" height="40" rx="2" fill="#475569" stroke="#64748b" stroke-width="2" />
    <rect x="37" y="48" width="86" height="24" fill="#334155" stroke="#64748b" stroke-width="2" />
  </g>
  <!-- Converter Housing -->
  <g id="converter" transform="translate(80, 40)">
    <rect x="-35" y="-30" width="70" height="36" rx="6" fill="#0f172a" stroke="#06b6d4" stroke-width="2" />
    <text x="0" y="-16" text-anchor="middle" fill="#94a3b8" font-size="8" font-weight="700">MAGFLOW (FT-101)</text>
    <text id="rate-text" x="0" y="-2" text-anchor="middle" fill="#38bdf8" font-size="12" font-weight="800">342.8 GPM</text>
  </g>
  <text x="80" y="100" text-anchor="middle" fill="#94a3b8" font-size="11" font-weight="700">FT-101</text>
</svg>`,
			Tags: []SymbolTag{
				{Name: "flanges", Role: "shape", ElementID: "flowmeter-flanges"},
				{Name: "converter", Role: "shape", ElementID: "converter"},
				{Name: "rateText", Role: "text", ElementID: "rate-text"},
			},
			Properties: []SymbolProperty{
				{ID: "unit", Type: "string", Default: "GPM", Label: "Engineering Unit"},
				{ID: "title", Type: "string", Default: "FT-101", Label: "Instrument Tag"},
			},
			Behaviors: []SymbolBehavior{
				{ID: "flowRate", Kind: "value", ValueType: "DOUBLE", Label: "Instantaneous Flow Rate"},
				{ID: "totalizer", Kind: "value", ValueType: "DOUBLE", Label: "Accumulated Mass/Vol"},
				{ID: "openFaceplate", Kind: "widgetAction", Label: "Open Faceplate"},
			},
			Animations: []SymbolAnimation{
				{Target: "rate-text", When: "flowRate >= 0", Set: map[string]any{"text": "val:flowRate + ' GPM'"}},
			},
			Version:   1,
			CreatedAt: now,
		},
		{
			ID:       ids.MustNew(),
			Name:     "High-Performance Pump (ISA-101)",
			Category: "Pumps",
			Style:    "isa-101",
			SVGContent: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 140" data-iotp-symbol="1">
  <!-- ISA-101 High Performance Gray Scale Theme -->
  <g id="isa-base" transform="translate(80, 120)">
    <rect x="-40" y="0" width="80" height="8" rx="1" fill="#475569" stroke="#64748b" stroke-width="1.5" />
  </g>
  <g id="isa-body" transform="translate(80, 70)">
    <circle id="isa-volute" cx="0" cy="0" r="38" fill="#334155" stroke="#94a3b8" stroke-width="2" />
    <circle cx="0" cy="0" r="14" fill="#1e293b" stroke="#64748b" stroke-width="1.5" />
    <path d="M -14 0 L 14 0" stroke="#cbd5e1" stroke-width="2" />
    <path d="M 0 -14 L 0 14" stroke="#cbd5e1" stroke-width="2" />
  </g>
  <!-- ISA-101 Alarm Priority Symbol (Diamond for Critical) -->
  <polygon id="alarm-indicator" points="80,18 92,30 80,42 68,30" fill="none" stroke="#64748b" stroke-width="2" />
  <text id="isa-label" x="80" y="98" text-anchor="middle" fill="#cbd5e1" font-size="11" font-weight="600">P-101-HP</text>
</svg>`,
			Tags: []SymbolTag{
				{Name: "volute", Role: "shape", ElementID: "isa-volute"},
				{Name: "alarmDiamond", Role: "shape", ElementID: "alarm-indicator"},
				{Name: "label", Role: "text", ElementID: "isa-label"},
			},
			Properties: []SymbolProperty{
				{ID: "title", Type: "string", Default: "P-101-HP", Label: "Equipment Tag"},
			},
			Behaviors: []SymbolBehavior{
				{ID: "running", Kind: "value", ValueType: "BOOL", Label: "Running State"},
				{ID: "alarm", Kind: "value", ValueType: "BOOL", Label: "Abnormal Alarm"},
				{ID: "openFaceplate", Kind: "widgetAction", Label: "Open Faceplate"},
			},
			Animations: []SymbolAnimation{
				{Target: "alarm-indicator", When: "alarm", Set: map[string]any{"fill": "#f43f5e", "stroke": "#ffffff"}},
				{Target: "isa-volute", When: "running", Set: map[string]any{"fill": "#1e293b", "stroke": "#f8fafc"}},
			},
			Version:   1,
			CreatedAt: now,
		},
	}
}
