package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/api"
	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/dashboard"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/edge"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/integration"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rpc"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/internal/scada"
	"github.com/qamati/IOT_V2/internal/telemetry"
	transportmqtt "github.com/qamati/IOT_V2/internal/transport/mqtt"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/bus/mem"
	"github.com/qamati/IOT_V2/pkg/config"
	"github.com/qamati/IOT_V2/pkg/db"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/qamati/IOT_V2/pkg/obs"
	"github.com/qamati/IOT_V2/pkg/svc"
)

type Config struct {
	Service   svc.Config           `koanf:"service"`
	Database  db.Config            `koanf:"database"`
	MQTT      transportmqtt.Config `koanf:"mqtt"`
	JWTSecret string               `koanf:"jwt_secret"`
	Pepper    string               `koanf:"pepper"`
}

func main() {
	cfg, err := config.Load(config.Options[Config]{
		Default: Config{
			Service: svc.Config{
				Name:            "iotp-lite",
				HTTPPort:        8080,
				ShutdownTimeout: 10 * time.Second,
			},
			Database: db.Config{
				Host:     "127.0.0.1",
				Port:     5432,
				Database: "iotp",
				User:     "iotp",
				Password: "iotp_secret",
				SSLMode:  "disable",
			},
			MQTT: transportmqtt.Config{
				Addr: ":1883",
				Port: 1883,
			},
			JWTSecret: "iotp-super-secret-development-jwt-key-32b",
			Pepper:    "iotp-default-token-pepper-secret-32b",
		},
		EnvPrefix: "IOTP_",
	})
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := obs.NewLogger(os.Stdout, slog.LevelInfo, cfg.Service.Name)
	logger.Info("bootstrapping iotp-lite modular monolith",
		"http_port", cfg.Service.HTTPPort,
		"mqtt_port", cfg.MQTT.Port)

	// 1. Storage layers (Postgres or in-memory fallback)
	ctx := context.Background()
	var userStore identity.UserStore
	var devStore core.DeviceStore
	var tsStore telemetry.Store

	pool, dbErr := db.NewPool(ctx, cfg.Database)
	if dbErr == nil && pool.Ping(ctx) == nil {
		logger.Info("connected to PostgreSQL/TimescaleDB", "database", cfg.Database.Database)
		// Run migrations automatically
		if err := db.MigrateUp(cfg.Database.ConnString(), "db/migrations"); err != nil {
			logger.Warn("auto-migration warning (continuing)", "error", err)
		}
		userStore = identity.NewPostgresUserStore(pool)
		devStore = core.NewPostgresDeviceStore(pool)
		tsStore = telemetry.NewPostgresStore(pool)
	} else {
		logger.Info("using high-speed in-memory store for standalone dev/demo mode")
		userStore = identity.NewMemoryUserStore()
		devStore = core.NewMemoryDeviceStore()
		tsStore = telemetry.NewMemoryStore()
	}

	// 2. Identity Service
	idSvc := identity.NewService(userStore, []byte(cfg.JWTSecret), 24*time.Hour)

	// Seed default Demo Tenant and Admin Users
	demoTenantID := ids.MustNew()
	demoUser1 := &model.User{
		ID:          ids.MustNew(),
		CreatedTime: time.Now().UnixMilli(),
		TenantID:    &demoTenantID,
		Authority:   model.AuthorityTenantAdmin,
		Email:       "demo@iotp.local",
		FirstName:   "Demo",
		LastName:    "Administrator",
		Version:     1,
	}
	_ = idSvc.Register(ctx, demoUser1, "demo1234")

	demoUser2 := &model.User{
		ID:          ids.MustNew(),
		CreatedTime: time.Now().UnixMilli(),
		TenantID:    &demoTenantID,
		Authority:   model.AuthorityTenantAdmin,
		Email:       "tenant@example.com",
		FirstName:   "Acme",
		LastName:    "Admin",
		Version:     1,
	}
	_ = idSvc.Register(ctx, demoUser2, "Secret123!")
	logger.Info("seeded demo users", "user1", "demo@iotp.local", "user2", "tenant@example.com", "tenant_id", demoTenantID)

	// 3. Core Device Manager
	devMgr := core.NewDeviceManager(devStore, []byte(cfg.Pepper))

	// 4. Extended Platform Services: Attributes, DevState, Alarms, Rule Engine, Dashboards, RPC
	attrStore := telemetry.NewMemoryAttributeStore()
	tracker := devstate.NewMemoryTracker(60 * time.Second)
	alarmSvc := alarm.NewMemoryService()
	ruleEngine := rule.NewEngine(alarmSvc, logger)
	dashboardSvc := dashboard.NewMemoryService()
	rpcSvc := rpc.NewMemoryService()
	integrationSvc := integration.NewMemoryService(devMgr, tsStore, attrStore, tracker, ruleEngine)
	edgeHub := edge.NewMemoryHub(tsStore, alarmSvc)
	scadaSvc := scada.NewMemoryService()

	// Seed demo webhook integration
	convs, _ := integrationSvc.ListConverters(ctx, demoTenantID)
	if len(convs) > 0 {
		_ = integrationSvc.CreateIntegration(ctx, &integration.Integration{
			TenantID:          demoTenantID,
			Name:              "Acme Weather Webhook",
			Type:              integration.TypeHTTPWebhook,
			Enabled:           true,
			UplinkConverterID: convs[0].ID,
			RoutingKey:        "acme-weather-wh",
			AllowCreateDevice: true,
			DefaultDeviceType: "weather_station",
		})
	}

	// Seed sample device
	sampleDev, _, _ := devMgr.CreateDevice(ctx, demoTenantID, "weather-station-alpha", "Building A Rooftop Sensor", "DEV_ALPHA_TOKEN_123")
	logger.Info("seeded sample device", "name", "weather-station-alpha", "token", "DEV_ALPHA_TOKEN_123")

	// Seed sample attributes for the device
	if sampleDev != nil {
		_ = attrStore.SaveAttributes(ctx, sampleDev.ID, model.ScopeClient, map[string]any{
			"firmware_version": "v2.1.0",
			"ip_address":       "192.168.1.42",
			"hardware_rev":     "ESP32-S3-WROOM",
		})
		_ = attrStore.SaveAttributes(ctx, sampleDev.ID, model.ScopeServer, map[string]any{
			"location":        "Building A Rooftop",
			"latitude":        37.7749,
			"longitude":       -122.4194,
			"target_temp_c":   22.0,
		})
		_ = attrStore.SaveAttributes(ctx, sampleDev.ID, model.ScopeShared, map[string]any{
			"upload_interval_sec": 5,
			"low_power_mode":      false,
		})
		tracker.RecordActivity(ctx, sampleDev.ID, time.Now().UnixMilli())

		// Seed a default Live Dashboard
		dashConfig, _ := json.Marshal(map[string]any{
			"deviceId": sampleDev.ID.String(),
			"widgets": []map[string]any{
				{"id": "w1", "type": "metric_card", "title": "Ambient Temperature", "key": "temperature", "unit": "°C"},
				{"id": "w2", "type": "metric_card", "title": "Relative Humidity", "key": "humidity", "unit": "%"},
				{"id": "w3", "type": "timeseries_chart", "title": "Temperature Trend", "key": "temperature"},
				{"id": "w4", "type": "rpc_control", "title": "Relay Switch #1", "method": "setRelay"},
			},
		})
		_, _ = dashboardSvc.CreateDashboard(ctx, demoTenantID, "Environmental Station Alpha Dashboard", dashConfig)
	}

	// 5. Message Bus & Direct-Save Worker with Rule Engine & Activity Hook
	b := mem.New(8)
	worker := telemetry.NewDirectSaveWorker(b, tsStore, logger)
	worker.AddHook(func(ctx context.Context, tenantID, deviceID ids.ID, points []*model.TelemetryDataPoint) {
		tracker.RecordActivity(ctx, deviceID, time.Now().UnixMilli())
		ruleEngine.EvaluateTelemetry(ctx, tenantID, deviceID, points)
	})

	if err := worker.Start(ctx); err != nil {
		logger.Error("failed to start telemetry worker", "error", err)
		os.Exit(1)
	}

	// Publish initial sample telemetry point for seeded device
	if sampleDev != nil {
		initPayload := []byte(`{"temperature":24.6,"humidity":55.2,"voltage":3.74,"battery":92,"status":"ONLINE"}`)
		msg := bus.NewMessage(demoTenantID.String(), "DEVICE", sampleDev.ID.String(), bus.TypeTelemetryUp, initPayload)
		_ = b.Publish(ctx, "telemetry.up", msg)
	}

	// 6. Embedded MQTT Broker
	mqttServer, err := transportmqtt.NewServer(cfg.MQTT, devMgr, b, logger)
	if err != nil {
		logger.Error("failed to initialize MQTT server", "error", err)
		os.Exit(1)
	}

	// 7. REST API Router with full service attachments
	apiSvc := api.NewAPI(idSvc, devMgr, tsStore, b).
		WithAttributes(attrStore).
		WithTracker(tracker).
		WithAlarms(alarmSvc).
		WithDashboards(dashboardSvc).
		WithRPC(rpcSvc).
		WithIntegrations(integrationSvc).
		WithEdge(edgeHub).
		WithScada(scadaSvc)

	// 8. Base Service Supervisor
	s := svc.New(cfg.Service.Name, cfg.Service, logger)

	// Mount REST API on /api/v1/
	s.Mux().Handle("/api/v1/", apiSvc.Router())

	// Mount Web Console SPA on root / if built, otherwise JSON API landing page
	distDir := filepath.Join("web", "apps", "console", "dist")
	if info, err := os.Stat(distDir); err == nil && info.IsDir() {
		logger.Info("serving Web Console SPA", "dir", distDir)
		fileServer := http.FileServer(http.Dir(distDir))
		s.Mux().HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/healthz") || strings.HasPrefix(r.URL.Path, "/readyz") || strings.HasPrefix(r.URL.Path, "/metrics") {
				http.NotFound(w, r)
				return
			}
			filePath := filepath.Join(distDir, filepath.Clean(r.URL.Path))
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	} else {
		s.Mux().HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"platform": "iotp",
				"version": "0.1.0",
				"status": "ONLINE",
				"endpoints": {
					"login": "POST /api/v1/auth/login",
					"devices": "GET/POST /api/v1/devices",
					"telemetry": "GET /api/v1/telemetry/DEVICE/{id}/values/latest"
				}
			}`))
		})
	}

	// Background task: MQTT broker
	s.AddTask(func(ctx context.Context) error {
		go func() {
			<-ctx.Done()
			_ = mqttServer.Close()
		}()
		return mqttServer.Serve()
	})

	if err := s.Run(ctx); err != nil {
		logger.Error("service terminated with error", "error", err)
		os.Exit(1)
	}
}
