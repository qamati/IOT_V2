package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Options configures how settings are loaded.
type Options[T any] struct {
	Default    T
	ConfigFile string
	EnvPrefix  string
}

// Load reads and parses configuration from defaults, an optional file, and environment variables into target type T.
func Load[T any](opts Options[T]) (*T, error) {
	k := koanf.New(".")

	// 1. Load struct defaults
	if err := k.Load(structs.Provider(opts.Default, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("config: failed to load struct defaults: %w", err)
	}

	// 2. Load config file if specified and exists
	filePath := opts.ConfigFile
	if filePath == "" {
		filePath = os.Getenv("IOTP_CONFIG_FILE")
	}
	if filePath != "" {
		if _, err := os.Stat(filePath); err == nil {
			if err := k.Load(file.Provider(filePath), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("config: failed to load config file %q: %w", filePath, err)
			}
		}
	}

	// 3. Load environment variables
	prefix := opts.EnvPrefix
	if prefix == "" {
		prefix = "IOTP_"
	}
	err := k.Load(env.Provider(prefix, ".", func(s string) string {
		// Strip prefix and transform IOTP_SERVER_PORT or IOTP_SERVER__PORT to server.port
		trimmed := strings.TrimPrefix(s, prefix)
		// Lowercase and replace underscores with dots for nested keys
		lower := strings.ToLower(trimmed)
		return strings.ReplaceAll(lower, "_", ".")
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("config: failed to load environment variables: %w", err)
	}

	// 4. Unmarshal into target struct
	var target T
	if err := k.UnmarshalWithConf("", &target, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("config: failed to unmarshal configuration: %w", err)
	}

	return &target, nil
}
