package alarm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// EvaluatorState tracks historical window metrics for duration & repeating alarm conditions.
type EvaluatorState struct {
	TrueSince   int64 // Unix timestamp ms when condition first became true
	TrueCount   int   // Number of consecutive events condition was true
	LastMatched bool
}

// Evaluator assesses incoming telemetry against active alarm rules.
type Evaluator struct {
	mu     sync.RWMutex
	states map[string]*EvaluatorState // key: "entityID:ruleID:severity"
}

// NewEvaluator creates a new Evaluator instance.
func NewEvaluator() *Evaluator {
	return &Evaluator{
		states: make(map[string]*EvaluatorState),
	}
}

// stateKey formats the lookup key for entity condition tracking.
func stateKey(entityID, ruleID ids.ID, severity model.AlarmSeverity) string {
	return fmt.Sprintf("%s:%s:%s", entityID.String(), ruleID.String(), string(severity))
}

func (e *Evaluator) getState(key string) *EvaluatorState {
	e.mu.Lock()
	defer e.mu.Unlock()
	st, ok := e.states[key]
	if !ok {
		st = &EvaluatorState{}
		e.states[key] = st
	}
	return st
}

// EvaluateFilter checks if a single metric filter matches given telemetry values.
func EvaluateFilter(f model.AlarmFilter, values map[string]any) bool {
	rawVal, ok := values[f.Key]
	if !ok {
		return false
	}

	switch f.ValueType {
	case model.ValueTypeNumeric, "":
		left, err1 := toFloat64(rawVal)
		right, err2 := toFloat64(f.Value)
		if err1 != nil || err2 != nil {
			return false
		}
		switch f.Operator {
		case model.OpEqual:
			return left == right
		case model.OpNotEqual:
			return left != right
		case model.OpGreater:
			return left > right
		case model.OpGreaterOrEqual:
			return left >= right
		case model.OpLess:
			return left < right
		case model.OpLessOrEqual:
			return left <= right
		default:
			return false
		}

	case model.ValueTypeString:
		left := fmt.Sprintf("%v", rawVal)
		right := fmt.Sprintf("%v", f.Value)
		switch f.Operator {
		case model.OpEqual:
			return left == right
		case model.OpNotEqual:
			return left != right
		case model.OpContains:
			return strings.Contains(left, right)
		default:
			return left == right
		}

	case model.ValueTypeBoolean:
		left := toBool(rawVal)
		right := toBool(f.Value)
		switch f.Operator {
		case model.OpEqual:
			return left == right
		case model.OpNotEqual:
			return left != right
		default:
			return left == right
		}
	}

	return false
}

// EvaluateCondition checks if an entire condition (all filters + temporal criteria) is satisfied.
func (e *Evaluator) EvaluateCondition(
	entityID, ruleID ids.ID,
	severity model.AlarmSeverity,
	cond model.AlarmCondition,
	ts int64,
	values map[string]any,
) (matched bool, actionReason string, durTrue int64, countTrue int) {
	if len(cond.Filters) == 0 {
		return false, "no filters specified", 0, 0
	}

	// 1. All filters must evaluate to true (logical AND)
	filtersPass := true
	for _, f := range cond.Filters {
		if !EvaluateFilter(f, values) {
			filtersPass = false
			break
		}
	}

	k := stateKey(entityID, ruleID, severity)
	st := e.getState(k)

	if !filtersPass {
		// Reset tracking state on non-match
		st.TrueSince = 0
		st.TrueCount = 0
		st.LastMatched = false
		return false, "filter condition not met", 0, 0
	}

	// 2. Evaluate temporal constraint
	switch cond.Type {
	case model.ConditionSimple, "":
		st.TrueCount++
		st.LastMatched = true
		return true, "simple condition matched", 0, st.TrueCount

	case model.ConditionDuration:
		if st.TrueSince == 0 {
			st.TrueSince = ts
		}
		durationElapsed := ts - st.TrueSince
		if durationElapsed < 0 {
			durationElapsed = 0
		}
		durTrue = durationElapsed

		if durationElapsed >= cond.DurationMs {
			st.LastMatched = true
			return true, fmt.Sprintf("duration satisfied (%d ms >= %d ms)", durationElapsed, cond.DurationMs), durationElapsed, st.TrueCount
		}
		return false, fmt.Sprintf("waiting on duration (%d ms / %d ms)", durationElapsed, cond.DurationMs), durationElapsed, st.TrueCount

	case model.ConditionRepeating:
		st.TrueCount++
		countTrue = st.TrueCount
		if st.TrueCount >= cond.Count {
			st.LastMatched = true
			return true, fmt.Sprintf("repeating count satisfied (%d >= %d)", st.TrueCount, cond.Count), 0, st.TrueCount
		}
		return false, fmt.Sprintf("repeating count progressing (%d / %d)", st.TrueCount, cond.Count), 0, st.TrueCount
	}

	return false, "unknown condition type", 0, 0
}

// TestSimulation runs a dry-run trace across simulated or historical data points.
func TestSimulation(req *model.AlarmRuleTestRequest) *model.AlarmRuleTestResponse {
	eval := NewEvaluator()
	dummyEntity := ids.MustNew()
	rule := req.Rule

	resp := &model.AlarmRuleTestResponse{
		RuleName: rule.Name,
		Traces:   make([]model.AlarmRuleTestTraceStep, 0, len(req.DataPoints)),
	}

	// Severity evaluation hierarchy
	severities := []model.AlarmSeverity{
		model.SeverityCritical,
		model.SeverityMajor,
		model.SeverityMinor,
		model.SeverityWarning,
		model.SeverityIndeterminate,
	}

	alarmActive := false

	for _, dp := range req.DataPoints {
		step := model.AlarmRuleTestTraceStep{
			Timestamp: dp.Timestamp,
			Action:    "NONE",
		}

		matchedSeverity := model.AlarmSeverity("")
		var matchedReason string
		var finalDur int64
		var finalCount int

		for _, sev := range severities {
			createDef, exists := rule.CreateRules[sev]
			if !exists {
				continue
			}

			matched, reason, dur, count := eval.EvaluateCondition(dummyEntity, rule.ID, sev, createDef.Condition, dp.Timestamp, dp.Values)
			if matched {
				matchedSeverity = sev
				matchedReason = reason
				finalDur = dur
				finalCount = count
				break
			}
		}

		if matchedSeverity != "" {
			step.Matched = true
			step.Severity = matchedSeverity
			step.Details = fmt.Sprintf("Triggered %s: %s", matchedSeverity, matchedReason)
			step.DurationTrue = finalDur
			step.RepeatCount = finalCount
			if !alarmActive {
				step.Action = "TRIGGER"
				alarmActive = true
			} else {
				step.Action = "UPDATE"
			}
		} else if rule.ClearRule != nil && alarmActive {
			// Check clear rule
			matched, reason, _, _ := eval.EvaluateCondition(dummyEntity, rule.ID, "CLEAR", rule.ClearRule.Condition, dp.Timestamp, dp.Values)
			if matched {
				step.Matched = true
				step.Action = "CLEAR"
				step.Details = fmt.Sprintf("Cleared alarm: %s", reason)
				alarmActive = false
			} else {
				step.Details = "Filters did not match, active alarm remains"
			}
		} else {
			step.Details = "Normal parameters"
		}

		resp.Traces = append(resp.Traces, step)
	}

	return resp
}

func toFloat64(v any) (float64, error) {
	if v == nil {
		return 0, fmt.Errorf("nil value")
	}
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case json.Number:
		return val.Float64()
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return strconv.ParseFloat(fmt.Sprintf("%v", val), 64)
	}
}

func toBool(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		b, _ := strconv.ParseBool(val)
		return b
	case int, int64:
		return val != 0
	default:
		return false
	}
}
