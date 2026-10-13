package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/qamati/IOT_V2/internal/edge"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

func main() {
	hubURL := flag.String("hub-url", "http://localhost:8080", "Cloud Edge Hub base URL")
	edgeKey := flag.String("edge-key", "EDGE_FACILITY_KEY_01", "Edge node key for authentication")
	edgeIDStr := flag.String("edge-id", "", "Edge Node UUID (optional, fetched from hub if empty)")
	syncInterval := flag.Duration("sync-interval", 3*time.Second, "Northbound store-and-forward sync interval")
	heartbeatInterval := flag.Duration("heartbeat-interval", 10*time.Second, "Heartbeat ping interval")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).
		With("service", "iotp-edge")

	logger.Info("starting autonomous iotp-edge runtime",
		"hub_url", *hubURL,
		"edge_key", *edgeKey,
		"sync_interval", *syncInterval,
	)

	client := &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Initial configuration fetch
	var edgeID ids.ID
	if *edgeIDStr != "" {
		edgeID = ids.MustParse(*edgeIDStr)
	} else {
		cfgURL := fmt.Sprintf("%s/api/v1/edge/configuration?key=%s", *hubURL, *edgeKey)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfgURL, nil)
		req.Header.Set("X-Edge-Key", *edgeKey)
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var edgeCfg edge.EdgeConfiguration
			if err := json.NewDecoder(resp.Body).Decode(&edgeCfg); err == nil && edgeCfg.EdgeID != ids.Nil {
				edgeID = edgeCfg.EdgeID
				logger.Info("retrieved edge configuration from cloud hub",
					"edge_id", edgeID,
					"name", edgeCfg.Name,
					"assigned_devices_count", len(edgeCfg.AssignedDevices),
				)
			}
			_ = resp.Body.Close()
		}
	}

	if edgeID == ids.Nil {
		edgeID = ids.MustNew()
		logger.Warn("could not fetch configuration from hub, operating with autonomous node ID", "edge_id", edgeID)
	}

	// 2. Initialize local Store-and-Forward Queue
	queue := edge.NewQueue(edgeID, 10000)

	// 3. Southbound Sensor Data Collector (Local generation & Modbus simulation)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		var counter int64
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				counter++
				now := time.Now().UnixMilli()

				// Simulated industrial sensor telemetry
				pressure := 45.0 + rand.Float64()*10.0
				flow := 120.0 + rand.Float64()*25.0
				vibration := 1.2 + rand.Float64()*0.4

				telemetryPayload, _ := json.Marshal([]*model.TelemetryDataPoint{
					{Key: "line_pressure_bar", Timestamp: now, DblV: &pressure},
					{Key: "feed_flow_lpm", Timestamp: now, DblV: &flow},
					{Key: "motor_vibration_g", Timestamp: now, DblV: &vibration},
				})

				_, _ = queue.Enqueue(ctx, edge.LaneL1, edge.SyncItem{
					EntityID:  edgeID,
					Timestamp: now,
					Type:      "TELEMETRY",
					Payload:   telemetryPayload,
				})

				// Simulate local critical alarm on edge when pressure spikes
				if pressure > 52.0 {
					alarmPayload, _ := json.Marshal(map[string]any{
						"type":     "CRITICAL_LINE_PRESSURE",
						"severity": "CRITICAL",
						"details": map[string]any{
							"message":  fmt.Sprintf("Local edge safety trigger: pressure spike at %.2f bar", pressure),
							"pressure": pressure,
						},
					})
					_, _ = queue.Enqueue(ctx, edge.LaneL0, edge.SyncItem{
						EntityID:  edgeID,
						Timestamp: now,
						Type:      "ALARM",
						Payload:   alarmPayload,
					})
					logger.Warn("local edge alarm triggered and enqueued to L0", "pressure", pressure)
				}
			}
		}
	}()

	// 4. Northbound Store-and-Forward Sync Pump
	go func() {
		ticker := time.NewTicker(*syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := queue.Stats()
				if stats.TotalQueued == 0 {
					continue
				}

				batch, err := queue.DrainBatch(ctx, 100)
				if err != nil || batch == nil {
					continue
				}

				body, _ := json.Marshal(batch)
				syncURL := fmt.Sprintf("%s/api/v1/edge/sync", *hubURL)
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, syncURL, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Edge-Key", *edgeKey)

				resp, err := client.Do(req)
				if err != nil {
					logger.Warn("cloud hub unreachable, store-and-forward queue buffering offline",
						"queued_items", stats.TotalQueued,
						"error", err.Error(),
					)
					continue
				}

				if resp.StatusCode == http.StatusOK {
					var ack edge.SyncAck
					_ = json.NewDecoder(resp.Body).Decode(&ack)
					_ = resp.Body.Close()

					_ = queue.Ack(batch.Sequence)
					logger.Info("successfully synced batch with cloud hub",
						"sequence", batch.Sequence,
						"processed_items", ack.ProcessedItems,
						"remaining_in_queue", queue.Stats().TotalQueued,
					)
				} else {
					bodyBytes, _ := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
					logger.Warn("hub rejected sync batch, will retry", "status", resp.StatusCode, "body", string(bodyBytes))
				}
			}
		}
	}()

	// 5. Periodic Heartbeat Worker
	go func() {
		ticker := time.NewTicker(*heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hbURL := fmt.Sprintf("%s/api/v1/edge/heartbeat", *hubURL)
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, hbURL, nil)
				req.Header.Set("X-Edge-Key", *edgeKey)
				resp, err := client.Do(req)
				if err == nil {
					_ = resp.Body.Close()
				}
			}
		}
	}()

	logger.Info("iotp-edge runtime running; press Ctrl+C to stop")
	<-ctx.Done()
	logger.Info("iotp-edge shutting down gracefully")
}
