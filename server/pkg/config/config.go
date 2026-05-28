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
	WebPort    string `mapstructure:"WEB_PORT"`

	// LLM Provider settings
	LLMProvider string `mapstructure:"LLM_PROVIDER"`
	LLMModel    string `mapstructure:"LLM_MODEL"`

	LLMFastModel              string  `mapstructure:"LLM_FAST_MODEL"`
	LLMBalancedModel          string  `mapstructure:"LLM_BALANCED_MODEL"`
	LLMPowerfulModel          string  `mapstructure:"LLM_POWERFUL_MODEL"`
	LLMAnthropicBalancedModel string  `mapstructure:"LLM_ANTHROPIC_BALANCED_MODEL"`
	LLMAnthropicPowerfulModel string  `mapstructure:"LLM_ANTHROPIC_POWERFUL_MODEL"`
	LLMGeminiFastModel        string  `mapstructure:"LLM_GEMINI_FAST_MODEL"`
	LLMGeminiBalancedModel    string  `mapstructure:"LLM_GEMINI_BALANCED_MODEL"`
	LLMCircuitMaxTokens       int     `mapstructure:"LLM_CIRCUIT_MAX_TOKENS"`
	LLMCircuitMaxCostUSD      float64 `mapstructure:"LLM_CIRCUIT_MAX_COST_USD"`
	LLMDefaultOutputTokens    int     `mapstructure:"LLM_DEFAULT_OUTPUT_TOKENS"`

	// API Keys mapped from env
	OpenAIAPIKey    string `mapstructure:"OPENAI_API_KEY"`
	AnthropicAPIKey string `mapstructure:"ANTHROPIC_API_KEY"`
	GeminiAPIKey    string `mapstructure:"GEMINI_API_KEY"`

	// APIKey is populated dynamically based on LLMProvider
	APIKey string

	// Database settings
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// Auth settings
	JWTSecret string `mapstructure:"JWT_SECRET"`

	// Sandbox settings
	SandboxRuntime       string `mapstructure:"SANDBOX_RUNTIME"`
	SandboxImage         string `mapstructure:"SANDBOX_IMAGE"`
	SandboxWorkspaceRoot string `mapstructure:"SANDBOX_WORKSPACE_ROOT"`
	SandboxMemoryMB      int64  `mapstructure:"SANDBOX_MEMORY_MB"`
	SandboxNanoCPUs      int64  `mapstructure:"SANDBOX_NANO_CPUS"`
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

	viper.SetDefault("SERVER_PORT", "32080")
	viper.SetDefault("WEB_PORT", "32300")
	viper.SetDefault("LLM_PROVIDER", "openai")
	viper.SetDefault("LLM_FAST_MODEL", "gpt-4o-mini")
	viper.SetDefault("LLM_BALANCED_MODEL", "gpt-4o")
	viper.SetDefault("LLM_POWERFUL_MODEL", "gpt-4o")
	viper.SetDefault("LLM_ANTHROPIC_BALANCED_MODEL", "claude-sonnet-4-20250514")
	viper.SetDefault("LLM_ANTHROPIC_POWERFUL_MODEL", "claude-opus-4-20250514")
	viper.SetDefault("LLM_GEMINI_FAST_MODEL", "gemini-2.5-flash")
	viper.SetDefault("LLM_GEMINI_BALANCED_MODEL", "gemini-2.5-pro")
	viper.SetDefault("LLM_CIRCUIT_MAX_TOKENS", 120000)
	viper.SetDefault("LLM_CIRCUIT_MAX_COST_USD", 2.50)
	viper.SetDefault("LLM_DEFAULT_OUTPUT_TOKENS", 2048)
	viper.SetDefault("SANDBOX_RUNTIME", "stub")
	viper.SetDefault("SANDBOX_IMAGE", "auto-code-os-sandbox:latest")
	viper.SetDefault("SANDBOX_WORKSPACE_ROOT", "/tmp/auto-code-os/workspaces")
	viper.SetDefault("SANDBOX_MEMORY_MB", 1024)
	viper.SetDefault("SANDBOX_NANO_CPUS", 1000000000)

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.LLMProvider = strings.ToLower(cfg.LLMProvider)
	cfg.SandboxRuntime = strings.ToLower(cfg.SandboxRuntime)

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
	case "gateway":
		if cfg.OpenAIAPIKey == "" && cfg.AnthropicAPIKey == "" && cfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("LLM_PROVIDER=gateway requires at least one provider API key")
		}
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, anthropic, gemini, gateway)", cfg.LLMProvider)
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("missing DATABASE_URL environment variable")
	}

	return &cfg, nil
}
