// Package config provides configuration management for the high-performance proxy server.
// It handles loading and parsing of YAML configuration files using the viper library,
// and provides strongly typed configuration structures for all proxy settings.
package config

import (
	"log/slog"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete proxy server configuration.
// It contains all settings necessary for running the proxy server,
// including server parameters, keep-alive settings, backend configurations,
// connection pooling options, and logging preferences.
type Config struct {
	Server         ServerConfig  `mapstructure:"server"`
	BackendSockets []string      `mapstructure:"backends"`
	Logging        LoggingConfig `mapstructure:"logging"`
	PreWarm        PreWarmConfig `mapstructure:"pre_warm"`
}

// ServerConfig defines the core server settings including address binding,
// timeouts, and connection limits.
type ServerConfig struct {
	ListenAddress    string        `mapstructure:"listen_address"`
	KeepAliveTimeout time.Duration `mapstructure:"keep_alive_timeout"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
}

// PreWarmConfig defines the settings for pre-warming connections to backends.
// It includes whether pre-warming is enabled and the number of requests to send
// to each backend during pre-warming.
type PreWarmConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	RequestsPerBackend int  `mapstructure:"requests_per_backend"`
}

// LoggingConfig contains settings for controlling the proxy's logging behavior,
// including enabling/disabling logging and setting the log level.
type LoggingConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Level   string `mapstructure:"level"`
}

// GetLevel converts the string log level from the configuration
// into a slog.Level value. If an invalid level is specified,
// it defaults to slog.LevelInfo.
func (l *LoggingConfig) GetLevel() slog.Level {
	switch l.Level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LoadConfig reads and parses the configuration file at the specified path.
// It returns a pointer to a Config struct containing all configuration settings.
// If the file cannot be read or parsed, it returns an error.
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
