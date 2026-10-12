package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/api"
	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/dashboard"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/edge"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/integration"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/notification"
	"github.com/qamati/IOT_V2/internal/ota"
	"github.com/qamati/IOT_V2/internal/realtime"
	"github.com/qamati/IOT_V2/internal/rpc"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/internal/scada"
	"github.com/qamati/IOT_V2/internal/telemetry"
	transportcoap "github.com/qamati/IOT_V2/internal/transport/coap"
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
	var attrStore telemetry.AttributeStore
	var alarmSvc alarm.Service
	var notifSvc notification.Service
	var dashboardSvc dashboard.Service
	var scadaSvc scada.Service
	var ruleStore rule.Store

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
		attrStore = telemetry.NewPostgresAttributeStore(pool)
		alarmSvc = alarm.NewPostgresService(pool)
		notifSvc = notification.NewPostgresService(pool, notification.Config{
			SMTPHost: "127.0.0.1",
			SMTPPort: 1025,
		})
		dashboardSvc = dashboard.NewPostgresService(pool)
		scadaSvc = scada.NewPostgresService(pool)
		ruleStore = rule.NewPostgresStore(pool)
		logger.Info("durable PostgreSQL stores initialized for users, devices, telemetry, attributes, alarms, notifications, dashboards, SCADA, and rule chains")
	} else {
		logger.Info("using high-speed in-memory store for standalone dev/demo mode")
		userStore = identity.NewMemoryUserStore()
		devStore = core.NewMemoryDeviceStore()
		tsStore = telemetry.NewMemoryStore()
		attrStore = telemetry.NewMemoryAttributeStore()
		alarmSvc = alarm.NewMemoryService()
		notifSvc = notification.NewMemoryService(notification.Config{
			SMTPHost: "127.0.0.1",
			SMTPPort: 1025,
		})
		dashboardSvc = dashboard.NewMemoryService()
		scadaSvc = scada.NewMemoryService()
		ruleStore = rule.NewMemoryStore()
	}

	// 2. Identity Service
	idSvc := identity.NewService(userStore, []byte(cfg.JWTSecret), 24*time.Hour)

	// Look up or seed default Demo Tenant and Admin Users
	var demoTenantID ids.ID
	if pool != nil {
		var err error
		demoTenantID, err = ensureDefaultTenant(ctx, pool)
		if err != nil {
			logger.Error("failed to ensure default tenant in postgres", "error", err)
			demoTenantID = ids.MustNew()
		}
	} else {
		demoTenantID = ids.MustNew()
	}

	existingUser, _, err := userStore.FindByEmail(ctx, "demo@iotp.local")
	if err == nil && existingUser != nil && existingUser.TenantID != nil {
		demoTenantID = *existingUser.TenantID
		logger.Info("loaded existing demo tenant and users from database", "tenant_id", demoTenantID)
	} else {
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
		if err := idSvc.Register(ctx, demoUser1, "demo1234"); err != nil {
			logger.Error("failed to seed demo user demo@iotp.local", "error", err)
		}

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
		if err := idSvc.Register(ctx, demoUser2, "Secret123!"); err != nil {
			logger.Error("failed to seed demo user tenant@example.com", "error", err)
		}
		logger.Info("seeded demo users", "user1", "demo@iotp.local", "user2", "tenant@example.com", "tenant_id", demoTenantID)
	}

	// 3. Core Device Manager
	devMgr := core.NewDeviceManager(devStore, []byte(cfg.Pepper))

	// 4. Extended Platform Services: DevState, Rule Engine, RPC, Integrations, Edge
	tracker := devstate.NewEngine(60*time.Second, attrStore, alarmSvc)
	tracker.Start(ctx)
	ruleServices := &rule.Services{
		Telemetry:  tsStore,
		Attributes: attrStore,
		Alarms:     alarmSvc,
		DevState:   tracker,
		HTTPClient: http.DefaultClient,
	}
	ruleEngine := rule.NewEngineWithStore(ruleStore, ruleServices, logger)
	var rpcSvc rpc.Service
	var otaSvc ota.Service
	var integrationSvc integration.Service
	if pool != nil {
		rpcStore := rpc.NewPostgresStore(pool)
		rpcSvc = rpc.NewDefaultService(rpcStore, logger)
		otaStore := ota.NewPostgresStore(pool)
		otaSvc = ota.NewService(otaStore, attrStore, logger)
		integStore := integration.NewPostgresStore(pool)
		integrationSvc = integration.NewService(integStore, devMgr, tsStore, attrStore, tracker, ruleEngine)
		logger.Info("durable PostgreSQL stores initialized for Downlink RPC, OTA Management, and Integrations & Converters")
	} else {
		rpcSvc = rpc.NewMemoryService()
		otaSvc = ota.NewService(nil, attrStore, logger)
		integrationSvc = integration.NewMemoryService(devMgr, tsStore, attrStore, tracker, ruleEngine)
	}
	edgeHub := edge.NewMemoryHub(tsStore, alarmSvc)

	// Seed demo SCADA process tags for Water Treatment HMI
	_ = scadaSvc.RegisterTag(ctx, &scada.Tag{
		TenantID:   demoTenantID,
		Path:       "WaterTreatment.Tank101.Level",
		DataType:   scada.DataTypeAnalog,
		EngUnit:    "%",
		Deadband:   0.5,
		EUMin:      0.0,
		EUMax:      100.0,
		ScanRateMs: 1000,
		Value:      76.4,
		Quality:    scada.QualityGood,
	})
	_ = scadaSvc.RegisterTag(ctx, &scada.Tag{
		TenantID:   demoTenantID,
		Path:       "WaterTreatment.Pump101.Status",
		DataType:   scada.DataTypeDiscrete,
		EngUnit:    "STATE",
		ScanRateMs: 1000,
		Value:      1,
		Quality:    scada.QualityGood,
	})
	_ = scadaSvc.RegisterTag(ctx, &scada.Tag{
		TenantID:   demoTenantID,
		Path:       "WaterTreatment.Pump101.Flow",
		DataType:   scada.DataTypeAnalog,
		EngUnit:    "GPM",
		Deadband:   1.0,
		EUMin:      0.0,
		EUMax:      500.0,
		ScanRateMs: 1000,
		Value:      342.8,
		Quality:    scada.QualityGood,
	})
	_ = scadaSvc.RegisterTag(ctx, &scada.Tag{
		TenantID:   demoTenantID,
		Path:       "WaterTreatment.Discharge.Pressure",
		DataType:   scada.DataTypeAnalog,
		EngUnit:    "PSI",
		Deadband:   0.5,
		EUMin:      0.0,
		EUMax:      150.0,
		ScanRateMs: 1000,
		Value:      62.5,
		Quality:    scada.QualityGood,
	})
	_ = scadaSvc.RegisterTag(ctx, &scada.Tag{
		TenantID:   demoTenantID,
		Path:       "WaterTreatment.Valve101.Position",
		DataType:   scada.DataTypeAnalog,
		EngUnit:    "%",
		Deadband:   1.0,
		EUMin:      0.0,
		EUMax:      100.0,
		ScanRateMs: 1000,
		Value:      100.0,
		Quality:    scada.QualityGood,
	})
	logger.Info("demo SCADA tags registered for water treatment synoptic HMI")

	// Seed sample device
	sampleDev, _, _ := devMgr.CreateDevice(ctx, demoTenantID, "weather-station-alpha", "Building A Rooftop Sensor", "DEV_ALPHA_TOKEN_123")
	if sampleDev == nil {
		devs, _, _ := devMgr.ListDevices(ctx, demoTenantID, 0, 10)
		for _, d := range devs {
			if d.Name == "weather-station-alpha" {
				sampleDev = d
				break
			}
		}
	}
	if sampleDev != nil {
		logger.Info("sample device active", "name", sampleDev.Name, "id", sampleDev.ID)
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
		tracker.RegisterDevice(ctx, demoTenantID, sampleDev.ID, 60000)
		tracker.RecordActivity(ctx, sampleDev.ID, time.Now().UnixMilli())

		// Seed a default Live Dashboard if none exist
		existingDashboards, totalDash, _ := dashboardSvc.ListDashboards(ctx, demoTenantID, 0, 1)
		if totalDash == 0 && len(existingDashboards) == 0 {
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

		// Seed demo firmware package for sample device
		sampleFw, _ := otaSvc.CreatePackage(ctx, demoTenantID, model.OtaTypeFirmware, "ESP32 Sensor Firmware", "v2.2.0", "stable", "")
		if sampleFw != nil {
			sampleFwData := []byte("DEMO_FIRMWARE_BINARY_V2.2.0_HASH_VERIFIED_OK")
			_, _ = otaSvc.UploadPackageBinary(ctx, demoTenantID, sampleFw.ID, "firmware-v2.2.0.bin", "application/octet-stream", sampleFwData)
			_, _ = otaSvc.AssignPackage(ctx, demoTenantID, "FIRMWARE", sampleFw.ID, "DEVICE", sampleDev.ID)
			logger.Info("seeded demo OTA firmware package and assigned to sample device", "version", "v2.2.0")
		}
	}

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

	// 5. Message Bus & Direct-Save Worker with Rule Engine, Alarm Evaluator & Activity Hook
	b := mem.New(8)
	alarmSvc.SetEventBus(b)
	_ = notifSvc.Start(ctx, b)

	rtSvc := realtime.NewService(idSvc, logger)
	_ = rtSvc.Start(ctx, b)

	worker := telemetry.NewDirectSaveWorker(b, tsStore, logger)
	worker.AddHook(func(ctx context.Context, tenantID, deviceID ids.ID, points []*model.TelemetryDataPoint) {
		tracker.RecordActivity(ctx, deviceID, time.Now().UnixMilli())
		ruleEngine.EvaluateTelemetry(ctx, tenantID, deviceID, points)

		vals := make(map[string]any)
		var latestTs int64
		for _, p := range points {
			vals[p.Key] = p.Value()
			if p.Timestamp > latestTs {
				latestTs = p.Timestamp
			}
		}
		if len(vals) > 0 {
			_, _ = alarmSvc.EvaluateTelemetry(ctx, tenantID, deviceID, latestTs, vals)
			rtSvc.BroadcastTelemetry(tenantID, deviceID, latestTs, vals)
		}
	})

	if err := worker.Start(ctx); err != nil {
		logger.Error("failed to start telemetry worker", "error", err)
		os.Exit(1)
	}

	// Seed Stage 5 Alarm Rule & Notifications
	_, _ = alarmSvc.SaveAlarmRule(ctx, &model.AlarmRule{
		TenantID:  demoTenantID,
		Name:      "High Temperature Critical Threshold",
		Scope:     "DEVICE_PROFILE",
		AlarmType: "High Temperature",
		Enabled:   true,
		CreateRules: map[model.AlarmSeverity]model.AlarmRuleCreateDef{
			model.SeverityCritical: {
				Condition: model.AlarmCondition{
					Type: model.ConditionSimple,
					Filters: []model.AlarmFilter{
						{Key: "temperature", ValueType: model.ValueTypeNumeric, Operator: model.OpGreater, Value: 40.0},
					},
				},
				Details: "Temperature exceeded 40°C threshold",
			},
		},
		ClearRule: &model.AlarmRuleCreateDef{
			Condition: model.AlarmCondition{
				Type: model.ConditionSimple,
				Filters: []model.AlarmFilter{
					{Key: "temperature", ValueType: model.ValueTypeNumeric, Operator: model.OpLessOrEqual, Value: 30.0},
				},
			},
		},
	})

	mailpitTarget, _ := notifSvc.SaveTarget(ctx, &model.NotificationTarget{
		TenantID: demoTenantID,
		Name:     "Maintenance Mailpit Email",
		Type:     model.TargetEmail,
		Config:   json.RawMessage(`{"emails":["maintenance@iotp.local","admin@iotp.local"]}`),
	})
	inboxTarget, _ := notifSvc.SaveTarget(ctx, &model.NotificationTarget{
		TenantID: demoTenantID,
		Name:     "Platform Web Inbox",
		Type:     model.TargetPlatformUsers,
		Config:   json.RawMessage(`{}`),
	})
	alertTemplate, _ := notifSvc.SaveTemplate(ctx, &model.NotificationTemplate{
		TenantID: demoTenantID,
		Name:     "Standard Alarm Notification",
		Type:     "ALARM",
		Config:   json.RawMessage(`{"subject":"[${alarm.severity}] ${alarm.type} Alert on Device ${device.id}","body":"An urgent alarm was triggered:\nType: ${alarm.type}\nSeverity: ${alarm.severity}\nTime: ${time}"}`),
	})
	if mailpitTarget != nil && inboxTarget != nil && alertTemplate != nil {
		_, _ = notifSvc.SaveRule(ctx, &model.NotificationRule{
			TenantID:    demoTenantID,
			Name:        "Critical Alarm Multi-Channel Delivery",
			Enabled:     true,
			TemplateID:  alertTemplate.ID,
			TriggerType: "ALARM",
			RecipientsConfig: []model.EscalationStep{
				{Step: 1, DelaySeconds: 0, TargetIDs: []ids.ID{mailpitTarget.ID, inboxTarget.ID}},
			},
		})
	}
	logger.Info("Stage 5 Alarm Rules & Multi-Channel Notification targets seeded for demo tenant")

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
	mqttServer.WithTracker(tracker).
		WithRpcService(rpcSvc).
		WithOtaService(otaSvc)
	rpcSvc.SetDispatcher(mqttServer)

	// 7. REST API Router with full service attachments
	apiSvc := api.NewAPI(idSvc, devMgr, tsStore, b).
		WithAttributes(attrStore).
		WithTracker(tracker).
		WithAlarms(alarmSvc).
		WithNotifications(notifSvc).
		WithDashboards(dashboardSvc).
		WithRPC(rpcSvc).
		WithOTA(otaSvc).
		WithIntegrations(integrationSvc).
		WithEdge(edgeHub).
		WithScada(scadaSvc).
		WithRuleEngine(ruleEngine).
		WithRealtime(rtSvc)

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

	// Background task: RFC 7252 CoAP UDP server
	coapServer := transportcoap.NewServer(":5683", devMgr, tsStore, attrStore, tracker, ruleEngine)
	if err := coapServer.Start(ctx); err != nil {
		logger.Warn("failed to start CoAP UDP transport (port 5683 might be in use or restricted)", "error", err)
	} else {
		logger.Info("started lightweight RFC 7252 CoAP UDP transport on :5683")
		s.AddTask(func(ctx context.Context) error {
			<-ctx.Done()
			coapServer.Stop()
			return nil
		})
	}

	if err := s.Run(ctx); err != nil {
		logger.Error("service terminated with error", "error", err)
		os.Exit(1)
	}
}

func ensureDefaultTenant(ctx context.Context, pool *pgxpool.Pool) (ids.ID, error) {
	var tenantID ids.ID
	err := pool.QueryRow(ctx, `SELECT id FROM tenant LIMIT 1`).Scan(&tenantID)
	if err == nil {
		return tenantID, nil
	}

	var profileID ids.ID
	err = pool.QueryRow(ctx, `SELECT id FROM tenant_profile WHERE is_default = true LIMIT 1`).Scan(&profileID)
	if err != nil {
		profileID = ids.MustNew()
		_, err = pool.Exec(ctx, `
			INSERT INTO tenant_profile (id, created_time, name, description, is_default, isolation_level, profile_data, schema_version, version)
			VALUES ($1, $2, 'Default Tenant Profile', 'System default profile', true, 1, '{}'::jsonb, 1, 1)
			ON CONFLICT (id) DO NOTHING
		`, profileID, time.Now().UnixMilli())
		if err != nil {
			return ids.Nil, fmt.Errorf("failed to create default tenant profile: %w", err)
		}
	}

	tenantID = ids.MustNew()
	_, err = pool.Exec(ctx, `
		INSERT INTO tenant (id, created_time, tenant_profile_id, title, version)
		VALUES ($1, $2, $3, 'Default Organization', 1)
		ON CONFLICT (id) DO NOTHING
	`, tenantID, time.Now().UnixMilli(), profileID)
	if err != nil {
		return ids.Nil, fmt.Errorf("failed to create default tenant: %w", err)
	}

	return tenantID, nil
}
