package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	// Server settings
	ServerPort string `mapstructure:"SERVER_PORT"`

	// LLM Provider settings
	LLMProvider string `mapstructure:"LLM_PROVIDER"`
	LLMModel    string `mapstructure:"LLM_MODEL"`

	// API Keys mapped from env
	OpenAIAPIKey    string `mapstructure:"OPENAI_API_KEY"`
	AnthropicAPIKey string `mapstructure:"ANTHROPIC_API_KEY"`
	GeminiAPIKey    string `mapstructure:"GEMINI_API_KEY"`

	// APIKey is populated dynamically based on LLMProvider
	APIKey      string

	// Database settings
	DatabaseURL string `mapstructure:"DATABASE_URL"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	viper.AutomaticEnv()

	// Try loading from .env in current directory, fallback to parent directory
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		viper.SetConfigFile("../.env")
		_ = viper.ReadInConfig() // ignore error if neither exists (rely on OS env vars)
	}

	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("LLM_PROVIDER", "openai")

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.LLMProvider = strings.ToLower(cfg.LLMProvider)

	switch cfg.LLMProvider {
	case "openai":
		if cfg.LLMModel == "" {
			cfg.LLMModel = "gpt-4o"
		}
		cfg.APIKey = cfg.OpenAIAPIKey
	case "anthropic":
		if cfg.LLMModel == "" {
			cfg.LLMModel = "claude-sonnet-4-20250514"
		}
		cfg.APIKey = cfg.AnthropicAPIKey
	case "gemini":
		if cfg.LLMModel == "" {
			cfg.LLMModel = "gemini-2.5-pro"
		}
		cfg.APIKey = cfg.GeminiAPIKey
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, anthropic, gemini)", cfg.LLMProvider)
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("missing DATABASE_URL environment variable")
	}

	return &cfg, nil
}
