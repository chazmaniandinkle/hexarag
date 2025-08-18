package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
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
			Port:        8080,
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
			Path:           "./data/hexarag.db",
			MigrationsPath: "./internal/adapters/storage/sqlite/migrations",
		},
		LLM: LLMConfig{
			Provider:    "openai-compatible",
			BaseURL:     "http://localhost:11434/v1",
			Model:       "llama2",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		Tools: ToolsConfig{
			MCPTimeServer: MCPTimeServerConfig{
				Enabled:   true,
				Timezones: []string{"UTC", "America/New_York", "Europe/London"},
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load loads configuration from files and environment variables
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()

	// Set config file path
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./deployments/config")
		v.AddConfigPath("./config")
		v.AddConfigPath(".")
	}

	// Environment variable support
	v.SetEnvPrefix("HEXARAG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is okay, we'll use defaults + env vars
	}

	// Unmarshal into struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	if c.LLM.BaseURL == "" {
		return fmt.Errorf("LLM base URL cannot be empty")
	}

	if c.LLM.Model == "" {
		return fmt.Errorf("LLM model cannot be empty")
	}

	if c.NATS.URL == "" {
		return fmt.Errorf("NATS URL cannot be empty")
	}

	return nil
}
