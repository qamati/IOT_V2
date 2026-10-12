package model

import (
	"encoding/json"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// ConditionType defines how an alarm condition is evaluated over time.
type ConditionType string

const (
	ConditionSimple    ConditionType = "SIMPLE"
	ConditionDuration  ConditionType = "DURATION"
	ConditionRepeating ConditionType = "REPEATING"
)

// ValueType defines the data type of the filter target.
const (
	ValueTypeNumeric = "NUMERIC"
	ValueTypeString  = "STRING"
	ValueTypeBoolean = "BOOLEAN"
)

// FilterOperator defines relational comparison operators for filters.
type FilterOperator string

const (
	OpEqual          FilterOperator = "EQUAL"
	OpNotEqual       FilterOperator = "NOT_EQUAL"
	OpGreater        FilterOperator = "GREATER"
	OpGreaterOrEqual FilterOperator = "GREATER_OR_EQUAL"
	OpLess           FilterOperator = "LESS"
	OpLessOrEqual    FilterOperator = "LESS_OR_EQUAL"
	OpContains       FilterOperator = "CONTAINS"
)

// DynamicValue specifies attribute resolution from related entities.
type DynamicValue struct {
	Source string `json:"source"` // CURRENT_DEVICE, CURRENT_CUSTOMER, CURRENT_TENANT
	Key    string `json:"key"`
}

// AlarmFilter defines an individual metric/attribute comparison clause.
type AlarmFilter struct {
	Key          string         `json:"key"`
	ValueType    string         `json:"value_type"` // NUMERIC, STRING, BOOLEAN
	Operator     FilterOperator `json:"operator"`
	Value        any            `json:"value"`
	DynamicValue *DynamicValue  `json:"dynamic_value,omitempty"`
}

// AlarmCondition aggregates filters with temporal constraints.
type AlarmCondition struct {
	Type       ConditionType `json:"type"`
	DurationMs int64         `json:"duration_ms,omitempty"`
	Count      int           `json:"count,omitempty"`
	Filters    []AlarmFilter `json:"filters"`
}

// AlarmRuleCreateDef defines a severity trigger condition and details template.
type AlarmRuleCreateDef struct {
	Condition AlarmCondition `json:"condition"`
	Details   string         `json:"details,omitempty"`
}

// AlarmRule defines declarative alert thresholds attached to profiles or entities.
type AlarmRule struct {
	ID             ids.ID                             `json:"id"`
	CreatedTime    int64                              `json:"created_time"`
	TenantID       ids.ID                             `json:"tenant_id"`
	Name           string                             `json:"name"`
	Scope          string                             `json:"scope"` // DEVICE_PROFILE, DEVICE, ASSET, CUSTOMER
	TargetID       *ids.ID                            `json:"target_id,omitempty"`
	AlarmType      string                             `json:"alarm_type"`
	Enabled        bool                               `json:"enabled"`
	CreateRules    map[AlarmSeverity]AlarmRuleCreateDef `json:"create_rules"`
	ClearRule      *AlarmRuleCreateDef                `json:"clear_rule,omitempty"`
	Propagate      json.RawMessage                    `json:"propagate,omitempty"`
	AdditionalInfo json.RawMessage                    `json:"additional_info,omitempty"`
	Version        int                                `json:"version"`
}

// AlarmRuleTestDataPoint is a mock telemetry reading used for testing.
type AlarmRuleTestDataPoint struct {
	Timestamp int64          `json:"timestamp"`
	Values    map[string]any `json:"values"`
}

// AlarmRuleTestRequest feeds a series of sample points into the rule evaluator.
type AlarmRuleTestRequest struct {
	Rule       AlarmRule                `json:"rule"`
	DataPoints []AlarmRuleTestDataPoint `json:"data_points"`
}

// AlarmRuleTestTraceStep records evaluation status for a specific point in time.
type AlarmRuleTestTraceStep struct {
	Timestamp    int64         `json:"timestamp"`
	Severity     AlarmSeverity `json:"severity,omitempty"`
	Matched      bool          `json:"matched"`
	Action       string        `json:"action"` // NONE, TRIGGER, CLEAR, UPDATE
	Details      string        `json:"details,omitempty"`
	DurationTrue int64         `json:"duration_true_ms"`
	RepeatCount  int           `json:"repeat_count"`
}

// AlarmRuleTestResponse returns simulation trace results.
type AlarmRuleTestResponse struct {
	RuleName string                   `json:"rule_name"`
	Traces   []AlarmRuleTestTraceStep `json:"traces"`
}
