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
				Name:     "core",
				HTTPPort: 8083,
			},
		},
		EnvPrefix: "IOTP_CORE_",
	})
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := obs.NewLogger(os.Stdout, slog.LevelInfo, cfg.Service.Name)
	logger.Info("starting core entity service")

	s := svc.New(cfg.Service.Name, cfg.Service, logger)
	if err := s.Run(context.Background()); err != nil {
		logger.Error("service terminated with error", "error", err)
		os.Exit(1)
	}
}
