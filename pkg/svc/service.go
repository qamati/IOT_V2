package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/qamati/IOT_V2/pkg/obs"
	"golang.org/x/sync/errgroup"
)

// ReadinessCheck defines a check function that returns nil when ready or an error when not.
type ReadinessCheck func(ctx context.Context) error

// Config holds service runtime configuration.
type Config struct {
	Name            string        `koanf:"name"`
	HTTPAddr        string        `koanf:"http_addr"`
	HTTPPort        int           `koanf:"http_port"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
}

// Service manages the lifecycle of an iotp microservice or modular monolith component.
type Service struct {
	name            string
	cfg             Config
	logger          *slog.Logger
	metrics         *obs.Metrics
	mux             *http.ServeMux
	server          *http.Server
	listener        net.Listener
	mu              sync.RWMutex
	readyChecks     map[string]ReadinessCheck
	additionalTasks []func(ctx context.Context) error
}

// New creates and initializes a new Service.
func New(name string, cfg Config, logger *slog.Logger) *Service {
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8080
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if logger == nil {
		logger = obs.NewLogger(os.Stdout, slog.LevelInfo, name)
	}

	metrics := obs.NewMetrics()
	mux := http.NewServeMux()

	s := &Service{
		name:        name,
		cfg:         cfg,
		logger:      logger,
		metrics:     metrics,
		mux:         mux,
		readyChecks: make(map[string]ReadinessCheck),
	}

	s.setupInternalRoutes()
	return s
}

func (s *Service) setupInternalRoutes() {
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "UP",
			"service": s.name,
		})
	})

	s.mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		ctx := r.Context()
		failures := make(map[string]string)
		for checkName, checkFn := range s.readyChecks {
			if err := checkFn(ctx); err != nil {
				failures[checkName] = err.Error()
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if len(failures) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":   "NOT_READY",
				"service":  s.name,
				"failures": failures,
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "READY",
			"service": s.name,
		})
	})

	s.mux.Handle("/metrics", s.metrics.Handler())
}

// RegisterReadinessCheck registers a named readiness check.
func (s *Service) RegisterReadinessCheck(name string, check ReadinessCheck) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readyChecks[name] = check
}

// AddTask adds a background task function that runs alongside the HTTP server and stops on context cancellation.
func (s *Service) AddTask(task func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.additionalTasks = append(s.additionalTasks, task)
}

// Mux returns the underlying HTTP ServeMux for registering custom application routes.
func (s *Service) Mux() *http.ServeMux {
	return s.mux
}

// Logger returns the service logger.
func (s *Service) Logger() *slog.Logger {
	return s.logger
}

// Metrics returns the service metrics registry.
func (s *Service) Metrics() *obs.Metrics {
	return s.metrics
}

// SetListener allows injecting a custom net.Listener (useful in unit tests).
func (s *Service) SetListener(ln net.Listener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listener = ln
}

// Run executes the service with signal trapping and graceful shutdown.
func (s *Service) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := s.cfg.HTTPAddr
	if addr == "" {
		addr = fmt.Sprintf(":%d", s.cfg.HTTPPort)
	}

	s.server = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	g, gCtx := errgroup.WithContext(ctx)

	// Run HTTP server
	g.Go(func() error {
		s.logger.Info("starting HTTP server", slog.String("addr", addr), slog.String("service", s.name))
		var err error
		if s.listener != nil {
			err = s.server.Serve(s.listener)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
		return nil
	})

	// Run additional background tasks
	for _, task := range s.additionalTasks {
		t := task
		g.Go(func() error {
			return t(gCtx)
		})
	}

	// Listen for shutdown signal
	g.Go(func() error {
		<-gCtx.Done()
		s.logger.Info("shutting down service", slog.String("service", s.name))

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()

		return s.server.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	s.logger.Info("service stopped gracefully", slog.String("service", s.name))
	return nil
}
