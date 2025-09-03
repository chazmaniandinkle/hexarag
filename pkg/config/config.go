package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"hexarag/internal/pkg/configutil"
	"hexarag/internal/pkg/constants"
	"hexarag/internal/pkg/logutil"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	NATS     NATSConfig     `mapstructure:"nats"`
	Database DatabaseConfig `mapstructure:"database"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Tools    ToolsConfig    `mapstructure:"tools"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	Host        string `mapstructure:"host"`
	CORSEnabled bool   `mapstructure:"cors_enabled"`
}

// NATSConfig holds NATS configuration
type NATSConfig struct {
	URL       string          `mapstructure:"url"`
	JetStream JetStreamConfig `mapstructure:"jetstream"`
}

// JetStreamConfig holds JetStream-specific configuration
type JetStreamConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	RetentionDays int  `mapstructure:"retention_days"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Path           string `mapstructure:"path"`
	MigrationsPath string `mapstructure:"migrations_path"`
}

// LLMConfig holds Language Model configuration
type LLMConfig struct {
	Provider    string  `mapstructure:"provider"`
	BaseURL     string  `mapstructure:"base_url"`
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

// ToolsConfig holds tool configuration
type ToolsConfig struct {
	MCPTimeServer MCPTimeServerConfig `mapstructure:"mcp_time_server"`
}

// MCPTimeServerConfig holds MCP time server configuration
type MCPTimeServerConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	Timezones []string `mapstructure:"timezones"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:        8080, // Could use constants.DefaultServerPort if defined
			Host:        "0.0.0.0",
			CORSEnabled: true,
		},
		NATS: NATSConfig{
			URL: "nats://localhost:4222",
			JetStream: JetStreamConfig{
				Enabled:       true,
				RetentionDays: 7,
			},
		},
		Database: DatabaseConfig{
			Path:           constants.DefaultDBPath,
			MigrationsPath: constants.DefaultMigrationsPath,
		},
		LLM: LLMConfig{
			Provider:    "openai-compatible",
			BaseURL:     "http://localhost:11434/v1",
			Model:       constants.DefaultModel,
			MaxTokens:   constants.DefaultMaxTokens,
			Temperature: constants.DefaultTemperature,
		},
		Tools: ToolsConfig{
			MCPTimeServer: MCPTimeServerConfig{
				Enabled:   true,
				Timezones: []string{"UTC", "America/New_York", "Europe/London"},
			},
		},
		Logging: LoggingConfig{
			Level:  constants.LogLevelInfo,
			Format: constants.LogFormatJSON,
		},
	}
}

// Load loads configuration from files and environment variables
func Load(configPath string) (*Config, error) {
	logger := logutil.NewDefaultLogger().WithFields(logutil.Fields{
		"component": "config_loader",
	})

	logger.Info("Loading configuration")

	cfg := DefaultConfig()

	v := viper.New()

	// Set config file path
	if configPath != "" {
		v.SetConfigFile(configPath)
		logger.Debug("Using specified config file", logutil.Fields{
			"path": configPath,
		})
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./deployments/config")
		v.AddConfigPath("./config")
		v.AddConfigPath(".")
		logger.Debug("Searching for config file in default paths")
	}

	// Environment variable support using constants
	v.SetEnvPrefix(strings.TrimPrefix(constants.EnvPort, "HEXARAG_")) // Extract prefix
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific environment variables using constants
	v.BindEnv("server.port", constants.EnvPort)
	v.BindEnv("server.host", constants.EnvHost)
	v.BindEnv("logging.level", constants.EnvLogLevel)
	v.BindEnv("logging.format", constants.EnvLogFormat)
	v.BindEnv("database.path", constants.EnvDBPath)
	v.BindEnv("nats.url", constants.EnvNATSURL)
	v.BindEnv("llm.provider", constants.EnvLLMProvider)
	v.BindEnv("llm.base_url", constants.EnvLLMBaseURL)
	v.BindEnv("llm.api_key", constants.EnvLLMAPIKey)
	v.BindEnv("llm.model", constants.EnvLLMModel)
	v.BindEnv("debug_mode", constants.EnvDebugMode)
	v.BindEnv("cors_enabled", constants.EnvCORSEnabled)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			logger.Error("Failed to read config file", logutil.Fields{
				"error": err.Error(),
			})
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is okay, we'll use defaults + env vars
		logger.Info("Config file not found, using defaults and environment variables")
	} else {
		logger.Info("Config file loaded successfully", logutil.Fields{
			"file": v.ConfigFileUsed(),
		})
	}

	// Unmarshal into struct
	if err := v.Unmarshal(cfg); err != nil {
		logger.Error("Failed to unmarshal config", logutil.Fields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	logger.Info("Configuration loaded successfully", logutil.Fields{
		"server_port":  cfg.Server.Port,
		"llm_provider": cfg.LLM.Provider,
		"log_level":    cfg.Logging.Level,
	})

	return cfg, nil
}

// Validate checks if the configuration is valid using configutil
func (c *Config) Validate() error {
	return c.ValidateWithDetails()
}

// ValidateWithDetails provides detailed validation with custom rules
func (c *Config) ValidateWithDetails() error {
	validator := configutil.NewValidator()

	// Server validation
	validator.RequiredString("server.host", c.Server.Host)
	validator.IntRange("server.port", c.Server.Port, 1, 65535)

	// Database validation
	validator.RequiredString("database.path", c.Database.Path)
	validator.RequiredString("database.migrations_path", c.Database.MigrationsPath)
	validator.ValidateFilePath("database.migrations_path", c.Database.MigrationsPath)

	// NATS validation
	validator.RequiredString("nats.url", c.NATS.URL)
	validator.ValidateURL("nats.url", c.NATS.URL)
	validator.IntRange("nats.jetstream.retention_days", c.NATS.JetStream.RetentionDays, 1, 365)

	// LLM validation
	validator.RequiredString("llm.provider", c.LLM.Provider)
	validator.RequiredString("llm.base_url", c.LLM.BaseURL)
	validator.ValidateURL("llm.base_url", c.LLM.BaseURL)
	validator.RequiredString("llm.model", c.LLM.Model)
	validator.IntRange("llm.max_tokens", c.LLM.MaxTokens, 1, 100000)

	// Logging validation
	validator.OneOf("logging.level", c.Logging.Level, []string{"debug", "info", "warn", "error"})
	validator.OneOf("logging.format", c.Logging.Format, []string{"text", "json"})

	return validator.Result()
}
