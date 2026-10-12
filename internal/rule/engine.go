package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

type defaultEnv struct {
	services *Services
	logger   *slog.Logger
	tenantID ids.ID
}

func (e *defaultEnv) Services() *Services   { return e.services }
func (e *defaultEnv) Logger() *slog.Logger  { return e.logger }
func (e *defaultEnv) TenantID() ids.ID      { return e.tenantID }

// Engine evaluates rule chains and executes directed acyclic graph node pipelines.
type Engine struct {
	store    Store
	services *Services
	compiler *Compiler
	catalog  *Catalog
	alarms   alarm.Service
	logger   *slog.Logger

	mu     sync.RWMutex
	chains map[ids.ID]*CompiledChain
}

// NewEngine creates a Rule Engine instance with in-memory store by default.
func NewEngine(alarmSvc alarm.Service, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	memStore := NewMemoryStore()
	services := &Services{
		Alarms: alarmSvc,
	}
	return &Engine{
		store:    memStore,
		services: services,
		compiler: NewCompiler(DefaultCatalog),
		catalog:  DefaultCatalog,
		alarms:   alarmSvc,
		logger:   logger.With("component", "rule_engine"),
		chains:   make(map[ids.ID]*CompiledChain),
	}
}

// NewEngineWithStore creates a Rule Engine instance backed by a specific Store and platform Services.
func NewEngineWithStore(store Store, services *Services, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	if services == nil {
		services = &Services{}
	}
	return &Engine{
		store:    store,
		services: services,
		compiler: NewCompiler(DefaultCatalog),
		catalog:  DefaultCatalog,
		alarms:   services.Alarms,
		logger:   logger.With("component", "rule_engine"),
		chains:   make(map[ids.ID]*CompiledChain),
	}
}

// SetStore sets the persistent or in-memory storage provider.
func (e *Engine) SetStore(store Store) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.store = store
	e.chains = make(map[ids.ID]*CompiledChain)
}

// SetServices updates available platform services.
func (e *Engine) SetServices(services *Services) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.services = services
	if services != nil && services.Alarms != nil {
		e.alarms = services.Alarms
	}
}

// Store returns the underlying Rule Chain store.
func (e *Engine) Store() Store {
	return e.store
}

// Catalog returns the node descriptor catalog.
func (e *Engine) Catalog() *Catalog {
	return e.catalog
}

// InvalidateChain flushes a compiled chain from the execution cache.
func (e *Engine) InvalidateChain(chainID ids.ID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.chains, chainID)
}

// EnsureRootRuleChain guarantees that a tenant has a Root Rule Chain seeded.
func (e *Engine) EnsureRootRuleChain(ctx context.Context, tenantID ids.ID) (*model.RuleChain, error) {
	root, err := e.store.GetRootRuleChain(ctx, tenantID)
	if err == nil && root != nil {
		return root, nil
	}

	// Seed default ThingsBoard Root Rule Chain
	chainID := ids.MustNew()
	newChain := &model.RuleChain{
		ID:                chainID,
		CreatedTime:       time.Now().UnixMilli(),
		TenantID:          tenantID,
		Name:              "Root Rule Chain",
		Type:              "CORE",
		IsRoot:            true,
		PublishedRevision: 1,
		AdditionalInfo:    json.RawMessage(`{"description":"Default platform root processing chain"}`),
		Version:           1,
	}

	if err := e.store.SaveRuleChain(ctx, newChain); err != nil {
		return nil, fmt.Errorf("failed to save default root rule chain: %w", err)
	}

	rev := &model.RuleChainRevision{
		RuleChainID: chainID,
		Revision:    1,
		Status:      "PUBLISHED",
		Definition:  NewDefaultRootChainDefinition(),
		Comment:     "Initial root chain revision",
		CreatedTime: time.Now().UnixMilli(),
	}

	if err := e.store.SaveRevision(ctx, rev); err != nil {
		return nil, fmt.Errorf("failed to save root rule chain revision: %w", err)
	}

	e.logger.Info("seeded default root rule chain for tenant", "tenant_id", tenantID, "chain_id", chainID)
	return newChain, nil
}

// GetOrCompileChain fetches or compiles a rule chain by ID.
func (e *Engine) GetOrCompileChain(ctx context.Context, tenantID, chainID ids.ID) (*CompiledChain, error) {
	e.mu.RLock()
	compiled, ok := e.chains[chainID]
	e.mu.RUnlock()
	if ok {
		return compiled, nil
	}

	rev, err := e.store.GetLatestRevision(ctx, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest revision for chain %s: %w", chainID, err)
	}

	env := &defaultEnv{
		services: e.services,
		logger:   e.logger,
		tenantID: tenantID,
	}

	newCompiled, _, err := e.compiler.Compile(&rev.Definition, env)
	if err != nil {
		return nil, fmt.Errorf("failed to compile rule chain %s: %w", chainID, err)
	}

	e.mu.Lock()
	e.chains[chainID] = newCompiled
	e.mu.Unlock()

	return newCompiled, nil
}

// Execute runs a message through the appropriate rule chain (either specified ChainID or tenant's root chain).
func (e *Engine) Execute(ctx context.Context, msg *Message) error {
	if msg.ID == ids.Nil {
		msg.ID = ids.MustNew()
	}
	if msg.Ts == 0 {
		msg.Ts = time.Now().UnixMilli()
	}

	var chainID ids.ID
	if msg.ChainID != ids.Nil {
		chainID = msg.ChainID
	} else {
		root, err := e.EnsureRootRuleChain(ctx, msg.TenantID)
		if err != nil {
			return err
		}
		chainID = root.ID
		msg.ChainID = chainID
	}

	compiled, err := e.GetOrCompileChain(ctx, msg.TenantID, chainID)
	if err != nil {
		return err
	}

	return compiled.Execute(ctx, msg, e.logger)
}

// ExecuteTrace runs a message through a chain and collects step-by-step debug traces.
func (e *Engine) ExecuteTrace(ctx context.Context, chainID ids.ID, msg *Message) (*ExecutionTrace, error) {
	if msg.ID == ids.Nil {
		msg.ID = ids.MustNew()
	}
	if msg.Ts == 0 {
		msg.Ts = time.Now().UnixMilli()
	}
	msg.ChainID = chainID

	compiled, err := e.GetOrCompileChain(ctx, msg.TenantID, chainID)
	if err != nil {
		return nil, err
	}

	return compiled.ExecuteWithTrace(ctx, msg, e.logger)
}

// ExecuteTestDefinition compiles an arbitrary chain definition on the fly and executes a test message.
func (e *Engine) ExecuteTestDefinition(ctx context.Context, tenantID ids.ID, def *model.ChainDefinition, msg *Message) (*ExecutionTrace, error) {
	if msg.ID == ids.Nil {
		msg.ID = ids.MustNew()
	}
	if msg.Ts == 0 {
		msg.Ts = time.Now().UnixMilli()
	}

	env := &defaultEnv{
		services: e.services,
		logger:   e.logger,
		tenantID: tenantID,
	}

	compiled, errs, err := e.compiler.Compile(def, env)
	if err != nil {
		return &ExecutionTrace{
			MessageID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("Compilation failed: %v", err),
		}, err
	}
	_ = errs

	return compiled.ExecuteWithTrace(ctx, msg, e.logger)
}

// EvaluateTelemetry evaluates rules on ingested telemetry data points (maintains full backwards compatibility).
func (e *Engine) EvaluateTelemetry(ctx context.Context, tenantID, deviceID ids.ID, points []*model.TelemetryDataPoint) {
	// 1. Direct threshold evaluation for alarm rules (preserves existing unit test expectations)
	if e.alarms != nil {
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

	// 2. Dispatches POST_TELEMETRY_REQUEST message through the Root Rule Chain
	dataMap := make(map[string]any)
	for _, pt := range points {
		if pt.Value() != nil {
			dataMap[pt.Key] = pt.Value()
		}
	}
	payloadBytes, _ := json.Marshal(dataMap)

	msg := &Message{
		ID:       ids.MustNew(),
		TenantID: tenantID,
		Ts:       time.Now().UnixMilli(),
		Type:     TypePostTelemetry,
		Originator: model.EntityRef{
			EntityType: "DEVICE",
			ID:         deviceID,
		},
		Metadata: map[string]string{
			"deviceName": deviceID.String(),
			"deviceType": "default",
			"ts":         fmt.Sprintf("%d", time.Now().UnixMilli()),
		},
		DataType: "JSON",
		Data:     payloadBytes,
	}

	_ = e.Execute(ctx, msg)
}
