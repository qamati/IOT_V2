package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/qamati/IOT_V2/pkg/config"
	"github.com/qamati/IOT_V2/pkg/obs"
	"github.com/qamati/IOT_V2/pkg/svc"
)

type Config struct {
	Service svc.Config `koanf:"service"`
}

func main() {
	cfg, err := config.Load(config.Options[Config]{
		Default: Config{
			Service: svc.Config{
				Name:     "iotp-lite",
				HTTPPort: 8080,
			},
		},
		EnvPrefix: "IOTP_",
	})
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := obs.NewLogger(os.Stdout, slog.LevelInfo, cfg.Service.Name)
	logger.Info("bootstrapping iotp-lite modular monolith")

	s := svc.New(cfg.Service.Name, cfg.Service, logger)

	if err := s.Run(context.Background()); err != nil {
		logger.Error("service terminated with error", "error", err)
		os.Exit(1)
	}
}
