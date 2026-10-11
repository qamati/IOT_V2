package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// HookFunc allows components like Rule Engine and Device State to observe processed telemetry points.
type HookFunc func(ctx context.Context, tenantID, deviceID ids.ID, points []*model.TelemetryDataPoint)

// DirectSaveWorker consumes telemetry messages from the bus and writes them to the time-series store.
type DirectSaveWorker struct {
	bus    bus.Bus
	store  Store
	logger *slog.Logger
	sub    bus.Subscription
	hooks  []HookFunc
}

func NewDirectSaveWorker(b bus.Bus, store Store, logger *slog.Logger) *DirectSaveWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &DirectSaveWorker{
		bus:    b,
		store:  store,
		logger: logger,
	}
}

// AddHook registers a hook called whenever telemetry is successfully parsed and saved.
func (w *DirectSaveWorker) AddHook(h HookFunc) {
	w.hooks = append(w.hooks, h)
}

// Start begins consuming telemetry from the bus.
func (w *DirectSaveWorker) Start(ctx context.Context) error {
	sub, err := w.bus.Subscribe(ctx, "telemetry.up", "telemetry-save-group", func(ctx context.Context, msg *bus.Message) error {
		deviceID, err := ids.Parse(msg.OriginatorID)
		if err != nil {
			w.logger.Error("invalid device ID in telemetry message", "id", msg.OriginatorID, "error", err)
			return nil // Drop poison message
		}

		points, err := ParsePayload(msg.Payload, msg.Timestamp)
		if err != nil {
			w.logger.Error("failed to parse telemetry payload", "device_id", deviceID, "error", err)
			return nil
		}

		if err := w.store.Save(ctx, deviceID, points); err != nil {
			w.logger.Error("failed to save telemetry data", "device_id", deviceID, "error", err)
			return err
		}

		tenantID, _ := ids.Parse(msg.TenantID)
		for _, h := range w.hooks {
			h(ctx, tenantID, deviceID, points)
		}

		w.logger.Debug("saved telemetry data", "device_id", deviceID, "points", len(points))
		return nil
	})
	if err != nil {
		return fmt.Errorf("telemetry worker: failed to subscribe: %w", err)
	}

	w.sub = sub
	w.logger.Info("telemetry direct-save worker started")
	return nil
}

// Stop unsubscribes the worker.
func (w *DirectSaveWorker) Stop() error {
	if w.sub != nil {
		return w.sub.Unsubscribe()
	}
	return nil
}
