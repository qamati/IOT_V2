package rule_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeCatalogDescriptors(t *testing.T) {
	catalog := rule.DefaultCatalog
	descriptors := catalog.ListDescriptors()
	require.NotEmpty(t, descriptors)

	types := make(map[string]bool)
	for _, d := range descriptors {
		types[d.Type] = true
	}

	assert.True(t, types["filter.msg_type_switch"])
	assert.True(t, types["filter.script"])
	assert.True(t, types["filter.msg_type"])
	assert.True(t, types["enrich.originator_attributes"])
	assert.True(t, types["action.save_timeseries"])
	assert.True(t, types["action.save_attributes"])
	assert.True(t, types["action.create_alarm"])
	assert.True(t, types["action.clear_alarm"])
	assert.True(t, types["action.log"])
	assert.True(t, types["external.rest_call"])
	assert.True(t, types["flow.output"])
	assert.True(t, types["flow.rule_chain"])
}

func TestDAGCompilationAndExecution(t *testing.T) {
	ctx := context.Background()
	alarmSvc := alarm.NewMemoryService()
	tsStore := telemetry.NewMemoryStore()
	attrStore := telemetry.NewMemoryAttributeStore()

	services := &rule.Services{
		Alarms:     alarmSvc,
		Telemetry:  tsStore,
		Attributes: attrStore,
	}

	engine := rule.NewEngineWithStore(rule.NewMemoryStore(), services, slog.Default())
	tenantID := ids.MustNew()
	devID := ids.MustNew()

	// Build a custom DAG:
	// Entry: msg_type_switch
	// Post telemetry -> filter.script (temp > 50)
	// True -> action.create_alarm (Critical Overheat)
	// False -> action.save_timeseries
	def := &model.ChainDefinition{
		FirstNodeID: "switch",
		Nodes: []model.RuleNode{
			{
				ID:   "switch",
				Type: "filter.msg_type_switch",
				Name: "Switch by Type",
			},
			{
				ID:            "check_overheat",
				Type:          "filter.script",
				Name:          "Check Overheat (>50)",
				Configuration: json.RawMessage(`{"field":"temperature","op":">","value":50}`),
			},
			{
				ID:            "create_alarm",
				Type:          "action.create_alarm",
				Name:          "Create Overheat Alarm",
				Configuration: json.RawMessage(`{"alarmType":"Critical Overheat Alert","severity":"CRITICAL"}`),
			},
			{
				ID:            "save_ts",
				Type:          "action.save_timeseries",
				Name:          "Save Normal Telemetry",
				Configuration: json.RawMessage(`{}`),
			},
		},
		Connections: []model.RuleConnection{
			{
				From:  "switch",
				To:    "check_overheat",
				Label: rule.RelationPostTelemetry,
			},
			{
				From:  "check_overheat",
				To:    "create_alarm",
				Label: rule.RelationTrue,
			},
			{
				From:  "check_overheat",
				To:    "save_ts",
				Label: rule.RelationFalse,
			},
		},
	}

	// 1. Test payload with temp = 72 (> 50) -> Overheat Alarm must be created
	msgOverheat := &rule.Message{
		ID:       ids.MustNew(),
		TenantID: tenantID,
		Ts:       time.Now().UnixMilli(),
		Type:     rule.TypePostTelemetry,
		Originator: model.EntityRef{
			EntityType: "DEVICE",
			ID:         devID,
		},
		Metadata: map[string]string{"deviceName": "sensor-1"},
		DataType: "JSON",
		Data:     []byte(`{"temperature": 72.5}`),
	}

	trace1, err := engine.ExecuteTestDefinition(ctx, tenantID, def, msgOverheat)
	require.NoError(t, err)
	require.True(t, trace1.Success)
	require.NotEmpty(t, trace1.Records)

	// Verify Alarm was created in alarm service
	alarms, count, err := alarmSvc.ListAlarms(ctx, tenantID, &devID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, "Critical Overheat Alert", alarms[0].Type)
	assert.Equal(t, model.SeverityCritical, alarms[0].Severity)

	// 2. Test payload with temp = 22 (<= 50) -> Timeseries must be saved, no new alarm
	msgNormal := &rule.Message{
		ID:       ids.MustNew(),
		TenantID: tenantID,
		Ts:       time.Now().UnixMilli(),
		Type:     rule.TypePostTelemetry,
		Originator: model.EntityRef{
			EntityType: "DEVICE",
			ID:         devID,
		},
		Metadata: map[string]string{"deviceName": "sensor-1"},
		DataType: "JSON",
		Data:     []byte(`{"temperature": 22.0}`),
	}

	trace2, err := engine.ExecuteTestDefinition(ctx, tenantID, def, msgNormal)
	require.NoError(t, err)
	require.True(t, trace2.Success)

	// Verify telemetry was saved in tsStore
	latest, err := tsStore.GetLatest(ctx, devID, []string{"temperature"})
	require.NoError(t, err)
	require.NotNil(t, latest["temperature"])
	assert.Equal(t, 22.0, *latest["temperature"].DblV)
}

func TestEnrichAttributesNode(t *testing.T) {
	ctx := context.Background()
	attrStore := telemetry.NewMemoryAttributeStore()
	tenantID := ids.MustNew()
	devID := ids.MustNew()

	// Seed server attributes on devID
	err := attrStore.SaveAttributes(ctx, devID, model.ScopeServer, map[string]any{
		"firmwareVersion": "v2.4.1",
		"location":        "Sector-7",
	})
	require.NoError(t, err)

	services := &rule.Services{
		Attributes: attrStore,
	}
	engine := rule.NewEngineWithStore(rule.NewMemoryStore(), services, slog.Default())

	def := &model.ChainDefinition{
		FirstNodeID: "enrich",
		Nodes: []model.RuleNode{
			{
				ID:            "enrich",
				Type:          "enrich.originator_attributes",
				Name:          "Enrich Server Attributes",
				Configuration: json.RawMessage(`{"target":"metadata"}`),
			},
			{
				ID:            "out",
				Type:          "flow.output",
				Name:          "Exit",
				Configuration: json.RawMessage(`{"relation":"Success"}`),
			},
		},
		Connections: []model.RuleConnection{
			{
				From:  "enrich",
				To:    "out",
				Label: rule.RelationSuccess,
			},
		},
	}

	msg := &rule.Message{
		ID:       ids.MustNew(),
		TenantID: tenantID,
		Ts:       time.Now().UnixMilli(),
		Type:     rule.TypePostTelemetry,
		Originator: model.EntityRef{
			EntityType: "DEVICE",
			ID:         devID,
		},
		Metadata: map[string]string{},
		Data:     []byte(`{"val": 10}`),
	}

	trace, err := engine.ExecuteTestDefinition(ctx, tenantID, def, msg)
	require.NoError(t, err)
	require.True(t, trace.Success)

	// Check that metadata was enriched
	lastRecord := trace.Records[len(trace.Records)-1]
	require.NotNil(t, lastRecord.InMessage)
	assert.Equal(t, "v2.4.1", lastRecord.InMessage.Metadata["firmwareVersion"])
	assert.Equal(t, "Sector-7", lastRecord.InMessage.Metadata["location"])
}

func TestHopLimitCycleProtection(t *testing.T) {
	ctx := context.Background()
	engine := rule.NewEngine(alarm.NewMemoryService(), slog.Default())
	tenantID := ids.MustNew()

	// Build a cycle: node1 -> node2 -> node1
	def := &model.ChainDefinition{
		FirstNodeID: "n1",
		Nodes: []model.RuleNode{
			{
				ID:   "n1",
				Type: "action.log",
				Name: "Log 1",
			},
			{
				ID:   "n2",
				Type: "action.log",
				Name: "Log 2",
			},
		},
		Connections: []model.RuleConnection{
			{
				From:  "n1",
				To:    "n2",
				Label: rule.RelationSuccess,
			},
			{
				From:  "n2",
				To:    "n1",
				Label: rule.RelationSuccess,
			},
		},
	}

	msg := &rule.Message{
		ID:       ids.MustNew(),
		TenantID: tenantID,
		Ts:       time.Now().UnixMilli(),
		Type:     rule.TypePostTelemetry,
		Originator: model.EntityRef{
			EntityType: "DEVICE",
			ID:         ids.MustNew(),
		},
		Metadata: map[string]string{},
		Data:     []byte(`{}`),
	}

	// Should not crash or infinite loop, but stop cleanly when hops reach 100
	trace, err := engine.ExecuteTestDefinition(ctx, tenantID, def, msg)
	assert.Error(t, err)
	assert.False(t, trace.Success)
	assert.Contains(t, trace.Error, "maximum hops guard limit")
}
