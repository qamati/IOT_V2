package alarm

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestEvaluatorSimpleAndClear(t *testing.T) {
	eval := NewEvaluator()
	entityID := ids.MustNew()
	ruleID := ids.MustNew()

	cond := model.AlarmCondition{
		Type: model.ConditionSimple,
		Filters: []model.AlarmFilter{
			{
				Key:       "temperature",
				ValueType: model.ValueTypeNumeric,
				Operator:  model.OpGreater,
				Value:     50.0,
			},
		},
	}

	// 1. Below threshold -> false
	matched, _, _, _ := eval.EvaluateCondition(entityID, ruleID, model.SeverityCritical, cond, 1000, map[string]any{"temperature": 45.0})
	if matched {
		t.Fatalf("expected not matched for temp 45")
	}

	// 2. Above threshold -> true
	matched, _, _, _ = eval.EvaluateCondition(entityID, ruleID, model.SeverityCritical, cond, 2000, map[string]any{"temperature": 55.0})
	if !matched {
		t.Fatalf("expected matched for temp 55")
	}
}

func TestEvaluatorDuration(t *testing.T) {
	eval := NewEvaluator()
	entityID := ids.MustNew()
	ruleID := ids.MustNew()

	cond := model.AlarmCondition{
		Type:       model.ConditionDuration,
		DurationMs: 60000, // 60 seconds
		Filters: []model.AlarmFilter{
			{
				Key:       "pressure",
				ValueType: model.ValueTypeNumeric,
				Operator:  model.OpGreater,
				Value:     100.0,
			},
		},
	}

	// T=0: First match, duration 0 < 60000 -> false
	matched, _, dur, _ := eval.EvaluateCondition(entityID, ruleID, model.SeverityCritical, cond, 1000, map[string]any{"pressure": 110.0})
	if matched || dur != 0 {
		t.Fatalf("expected not matched on first reading, got dur=%d", dur)
	}

	// T=30s: 30000 < 60000 -> false
	matched, _, dur, _ = eval.EvaluateCondition(entityID, ruleID, model.SeverityCritical, cond, 31000, map[string]any{"pressure": 115.0})
	if matched || dur != 30000 {
		t.Fatalf("expected not matched at 30s, got dur=%d", dur)
	}

	// T=61s: 61000 >= 60000 -> true!
	matched, _, dur, _ = eval.EvaluateCondition(entityID, ruleID, model.SeverityCritical, cond, 62000, map[string]any{"pressure": 120.0})
	if !matched || dur < 60000 {
		t.Fatalf("expected matched at 61s, got dur=%d", dur)
	}

	// T=70s: Pressure drops -> reset!
	matched, _, _, _ = eval.EvaluateCondition(entityID, ruleID, model.SeverityCritical, cond, 71000, map[string]any{"pressure": 90.0})
	if matched {
		t.Fatalf("expected reset when pressure drops")
	}
}

func TestEvaluatorRepeating(t *testing.T) {
	eval := NewEvaluator()
	entityID := ids.MustNew()
	ruleID := ids.MustNew()

	cond := model.AlarmCondition{
		Type:  model.ConditionRepeating,
		Count: 3,
		Filters: []model.AlarmFilter{
			{
				Key:       "voltage",
				ValueType: model.ValueTypeNumeric,
				Operator:  model.OpLess,
				Value:     200.0,
			},
		},
	}

	// 1st time
	matched, _, _, count := eval.EvaluateCondition(entityID, ruleID, model.SeverityMajor, cond, 1000, map[string]any{"voltage": 180.0})
	if matched || count != 1 {
		t.Fatalf("expected count 1, not matched")
	}

	// 2nd time
	matched, _, _, count = eval.EvaluateCondition(entityID, ruleID, model.SeverityMajor, cond, 2000, map[string]any{"voltage": 185.0})
	if matched || count != 2 {
		t.Fatalf("expected count 2, not matched")
	}

	// 3rd time -> triggered!
	matched, _, _, count = eval.EvaluateCondition(entityID, ruleID, model.SeverityMajor, cond, 3000, map[string]any{"voltage": 175.0})
	if !matched || count != 3 {
		t.Fatalf("expected count 3, matched!")
	}
}

func TestSimulationTrace(t *testing.T) {
	req := &model.AlarmRuleTestRequest{
		Rule: model.AlarmRule{
			ID:        ids.MustNew(),
			Name:      "High Temp Test",
			AlarmType: "High Temperature",
			CreateRules: map[model.AlarmSeverity]model.AlarmRuleCreateDef{
				model.SeverityCritical: {
					Condition: model.AlarmCondition{
						Type: model.ConditionSimple,
						Filters: []model.AlarmFilter{
							{Key: "temp", ValueType: model.ValueTypeNumeric, Operator: model.OpGreater, Value: 80.0},
						},
					},
					Details: "Temp is critical",
				},
			},
			ClearRule: &model.AlarmRuleCreateDef{
				Condition: model.AlarmCondition{
					Type: model.ConditionSimple,
					Filters: []model.AlarmFilter{
						{Key: "temp", ValueType: model.ValueTypeNumeric, Operator: model.OpLessOrEqual, Value: 70.0},
					},
				},
			},
		},
		DataPoints: []model.AlarmRuleTestDataPoint{
			{Timestamp: 1000, Values: map[string]any{"temp": 65.0}},
			{Timestamp: 2000, Values: map[string]any{"temp": 85.0}},
			{Timestamp: 3000, Values: map[string]any{"temp": 88.0}},
			{Timestamp: 4000, Values: map[string]any{"temp": 68.0}},
		},
	}

	res := TestSimulation(req)
	if len(res.Traces) != 4 {
		t.Fatalf("expected 4 trace steps, got %d", len(res.Traces))
	}

	if res.Traces[0].Action != "NONE" {
		t.Errorf("step 0: expected NONE, got %s", res.Traces[0].Action)
	}
	if res.Traces[1].Action != "TRIGGER" || res.Traces[1].Severity != model.SeverityCritical {
		t.Errorf("step 1: expected TRIGGER CRITICAL, got %s %s", res.Traces[1].Action, res.Traces[1].Severity)
	}
	if res.Traces[2].Action != "UPDATE" {
		t.Errorf("step 2: expected UPDATE, got %s", res.Traces[2].Action)
	}
	if res.Traces[3].Action != "CLEAR" {
		t.Errorf("step 3: expected CLEAR, got %s", res.Traces[3].Action)
	}
}

func TestAlarmRuleLifecycleAndEvaluation(t *testing.T) {
	ctx := context.Background()
	svc := NewMemoryService()
	tenantID := ids.MustNew()
	devID := ids.MustNew()

	rule := &model.AlarmRule{
		TenantID:  tenantID,
		Name:      "Boiler Pressure High",
		Scope:     "DEVICE_PROFILE",
		AlarmType: "High Pressure",
		Enabled:   true,
		CreateRules: map[model.AlarmSeverity]model.AlarmRuleCreateDef{
			model.SeverityCritical: {
				Condition: model.AlarmCondition{
					Type: model.ConditionSimple,
					Filters: []model.AlarmFilter{
						{Key: "pressure", ValueType: model.ValueTypeNumeric, Operator: model.OpGreater, Value: 100.0},
					},
				},
				Details: "Boiler pressure exceeds 100 bar",
			},
		},
		ClearRule: &model.AlarmRuleCreateDef{
			Condition: model.AlarmCondition{
				Type: model.ConditionSimple,
				Filters: []model.AlarmFilter{
					{Key: "pressure", ValueType: model.ValueTypeNumeric, Operator: model.OpLessOrEqual, Value: 80.0},
				},
			},
		},
	}

	saved, err := svc.SaveAlarmRule(ctx, rule)
	if err != nil {
		t.Fatalf("SaveAlarmRule failed: %v", err)
	}

	// 1. Send normal telemetry
	triggered, err := svc.EvaluateTelemetry(ctx, tenantID, devID, 1000, map[string]any{"pressure": 75.0})
	if err != nil {
		t.Fatalf("EvaluateTelemetry failed: %v", err)
	}
	if len(triggered) != 0 {
		t.Fatalf("expected 0 alarms triggered for pressure 75, got %d", len(triggered))
	}

	// 2. Send high telemetry -> Alarm triggers
	triggered, err = svc.EvaluateTelemetry(ctx, tenantID, devID, 2000, map[string]any{"pressure": 110.0})
	if err != nil {
		t.Fatalf("EvaluateTelemetry failed: %v", err)
	}
	if len(triggered) != 1 {
		t.Fatalf("expected 1 alarm triggered, got %d", len(triggered))
	}
	if triggered[0].Severity != model.SeverityCritical || triggered[0].Type != "High Pressure" {
		t.Fatalf("unexpected alarm properties: %+v", triggered[0])
	}

	// 3. Send low telemetry -> Alarm cleared
	triggered, err = svc.EvaluateTelemetry(ctx, tenantID, devID, 3000, map[string]any{"pressure": 70.0})
	if err != nil {
		t.Fatalf("EvaluateTelemetry failed: %v", err)
	}
	if len(triggered) != 1 || triggered[0].Status != model.AlarmClearedUnack {
		t.Fatalf("expected alarm cleared, got %+v", triggered)
	}

	// 4. Test delete rule
	err = svc.DeleteAlarmRule(ctx, tenantID, saved.ID)
	if err != nil {
		t.Fatalf("DeleteAlarmRule failed: %v", err)
	}
}
