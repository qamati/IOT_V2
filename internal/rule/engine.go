package rule

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Engine evaluates rules on telemetry points and triggers alerts/alarms.
type Engine struct {
	alarms alarm.Service
	logger *slog.Logger
}

// NewEngine creates a new Rule Engine instance.
func NewEngine(alarmSvc alarm.Service, logger *slog.Logger) *Engine {
	return &Engine{
		alarms: alarmSvc,
		logger: logger.With("component", "rule_engine"),
	}
}

// EvaluateTelemetry checks thresholds on ingested data points.
func (e *Engine) EvaluateTelemetry(ctx context.Context, tenantID, deviceID ids.ID, points []*model.TelemetryDataPoint) {
	for _, pt := range points {
		val := pt.Value()
		if val == nil {
			continue
		}

		// Check temperature threshold (> 45°C: CRITICAL, > 35°C: WARNING)
		if pt.Key == "temperature" {
			var numVal float64
			switch v := val.(type) {
			case float64:
				numVal = v
			case int64:
				numVal = float64(v)
			case int:
				numVal = float64(v)
			default:
				continue
			}

			if numVal > 45.0 {
				details, _ := json.Marshal(map[string]any{
					"reading":   numVal,
					"threshold": 45.0,
					"message":   "High Temperature Exceeded Critical Threshold",
				})
				_, _ = e.alarms.CreateOrUpdateAlarm(ctx, &model.Alarm{
					TenantID:     tenantID,
					OriginatorID: deviceID,
					Type:         "High Temperature Alert",
					Severity:     model.SeverityCritical,
					Details:      details,
				})
				e.logger.Warn("critical temperature alarm raised", "device_id", deviceID, "reading", numVal)
			} else if numVal > 35.0 {
				details, _ := json.Marshal(map[string]any{
					"reading":   numVal,
					"threshold": 35.0,
					"message":   "Temperature Warning Threshold Reached",
				})
				_, _ = e.alarms.CreateOrUpdateAlarm(ctx, &model.Alarm{
					TenantID:     tenantID,
					OriginatorID: deviceID,
					Type:         "Temperature Warning",
					Severity:     model.SeverityWarning,
					Details:      details,
				})
			}
		}

		// Check battery threshold (< 15%: WARNING)
		if pt.Key == "battery" {
			var numVal float64
			switch v := val.(type) {
			case float64:
				numVal = v
			case int64:
				numVal = float64(v)
			case int:
				numVal = float64(v)
			default:
				continue
			}

			if numVal < 15.0 {
				details, _ := json.Marshal(map[string]any{
					"reading":   numVal,
					"threshold": 15.0,
					"message":   "Battery Level Critically Low",
				})
				_, _ = e.alarms.CreateOrUpdateAlarm(ctx, &model.Alarm{
					TenantID:     tenantID,
					OriginatorID: deviceID,
					Type:         "Low Battery Alert",
					Severity:     model.SeverityMinor,
					Details:      details,
				})
			}
		}
	}
}
