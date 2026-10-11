package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/qamati/IOT_V2/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ServerConfig struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
}

type AppConfig struct {
	Environment string       `koanf:"environment"`
	Server      ServerConfig `koanf:"server"`
}

func TestConfigDefaults(t *testing.T) {
	cfg, err := config.Load(config.Options[AppConfig]{
		Default: AppConfig{
			Environment: "development",
			Server: ServerConfig{
				Host: "127.0.0.1",
				Port: 8080,
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestConfigEnvOverride(t *testing.T) {
	t.Setenv("IOTP_ENVIRONMENT", "production")
	t.Setenv("IOTP_SERVER_PORT", "9090")

	cfg, err := config.Load(config.Options[AppConfig]{
		Default: AppConfig{
			Environment: "development",
			Server: ServerConfig{
				Host: "127.0.0.1",
				Port: 8080,
			},
		},
		EnvPrefix: "IOTP_",
	})
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
}

func TestConfigFileOverride(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, "config.yaml")
	content := `
environment: staging
server:
  host: 0.0.0.0
  port: 8888
`
	require.NoError(t, os.WriteFile(yamlFile, []byte(content), 0644))

	cfg, err := config.Load(config.Options[AppConfig]{
		Default: AppConfig{
			Environment: "development",
			Server: ServerConfig{
				Host: "127.0.0.1",
				Port: 8080,
			},
		},
		ConfigFile: yamlFile,
	})
	require.NoError(t, err)
	assert.Equal(t, "staging", cfg.Environment)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8888, cfg.Server.Port)
}
