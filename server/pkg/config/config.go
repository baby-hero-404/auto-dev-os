package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Port        string `mapstructure:"SERVER_PORT"`
	WebPort     string `mapstructure:"WEB_PORT"`
	CORSOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`
}

type DatabaseConfig struct {
	URL string `mapstructure:"DATABASE_URL"`
}

type AuthConfig struct {
	JWTSecret string `mapstructure:"JWT_SECRET"`
}

type LLMConfig struct {
	Provider                  string  `mapstructure:"LLM_PROVIDER"`
	Model                     string  `mapstructure:"LLM_MODEL"`
	FastModel                 string  `mapstructure:"LLM_FAST_MODEL"`
	BalancedModel             string  `mapstructure:"LLM_BALANCED_MODEL"`
	PowerfulModel             string  `mapstructure:"LLM_POWERFUL_MODEL"`
	AnthropicBalancedModel    string  `mapstructure:"LLM_ANTHROPIC_BALANCED_MODEL"`
	AnthropicPowerfulModel    string  `mapstructure:"LLM_ANTHROPIC_POWERFUL_MODEL"`
	GeminiFastModel           string  `mapstructure:"LLM_GEMINI_FAST_MODEL"`
	GeminiBalancedModel       string  `mapstructure:"LLM_GEMINI_BALANCED_MODEL"`
	CircuitMaxTokens          int     `mapstructure:"LLM_CIRCUIT_MAX_TOKENS"`
	CircuitMaxCostUSD         float64 `mapstructure:"LLM_CIRCUIT_MAX_COST_USD"`
	DefaultOutputTokens       int     `mapstructure:"LLM_DEFAULT_OUTPUT_TOKENS"`
	MaxRetries                int     `mapstructure:"LLM_MAX_RETRIES"`
	OpenAIAPIKey              string  `mapstructure:"OPENAI_API_KEY"`
	AnthropicAPIKey           string  `mapstructure:"ANTHROPIC_API_KEY"`
	GeminiAPIKey              string  `mapstructure:"GEMINI_API_KEY"`
	BaseURL                   string  `mapstructure:"LLM_BASE_URL"`
	LLMAPIKey                 string  `mapstructure:"LLM_API_KEY"`
	APIKey                    string  // Dynamically populated based on provider
}

type SandboxConfig struct {
	Runtime       string `mapstructure:"SANDBOX_RUNTIME"`
	Image         string `mapstructure:"SANDBOX_IMAGE"`
	WorkspaceRoot string `mapstructure:"SANDBOX_WORKSPACE_ROOT"`
	MemoryMB      int64  `mapstructure:"SANDBOX_MEMORY_MB"`
	NanoCPUs      int64  `mapstructure:"SANDBOX_NANO_CPUS"`
}

type WorkerConfig struct {
	Enabled     bool `mapstructure:"QUEUE_WORKER_ENABLED"`
	IntervalMS  int  `mapstructure:"QUEUE_WORKER_INTERVAL_MS"`
	Concurrency int  `mapstructure:"QUEUE_WORKER_CONCURRENCY"`
}

type TelemetryConfig struct {
	OTLPTraceEndpoint string `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// Config holds all configuration for the application.
type Config struct {
	Server    ServerConfig    `mapstructure:",squash"`
	Database  DatabaseConfig  `mapstructure:",squash"`
	Auth      AuthConfig      `mapstructure:",squash"`
	LLM       LLMConfig       `mapstructure:",squash"`
	Sandbox   SandboxConfig   `mapstructure:",squash"`
	Worker    WorkerConfig    `mapstructure:",squash"`
	Telemetry TelemetryConfig `mapstructure:",squash"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()

	// In test mode, skip loading the config file to avoid test pollution from local .env files
	inTest := false
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") || strings.HasSuffix(os.Args[0], ".test") {
			inTest = true
			break
		}
	}

	if !inTest {
		// Try loading from .env in current directory, fallback to parent directory
		v.SetConfigFile(".env")
		if err := v.ReadInConfig(); err != nil {
			v.SetConfigFile("../.env")
			_ = v.ReadInConfig() // ignore error if neither exists (rely on OS env vars)
		}
	}

	v.SetDefault("SERVER_PORT", "32080")
	v.SetDefault("WEB_PORT", "32300")
	v.SetDefault("CORS_ALLOWED_ORIGINS", "")
	v.SetDefault("LLM_PROVIDER", "openai")
	v.SetDefault("LLM_BASE_URL", "")
	v.SetDefault("LLM_FAST_MODEL", "gpt-4o-mini")
	v.SetDefault("LLM_BALANCED_MODEL", "gpt-4o")
	v.SetDefault("LLM_POWERFUL_MODEL", "gpt-4o")
	v.SetDefault("LLM_ANTHROPIC_BALANCED_MODEL", "claude-sonnet-4-20250514")
	v.SetDefault("LLM_ANTHROPIC_POWERFUL_MODEL", "claude-opus-4-20250514")
	v.SetDefault("LLM_GEMINI_FAST_MODEL", "gemini-2.5-flash")
	v.SetDefault("LLM_GEMINI_BALANCED_MODEL", "gemini-2.5-pro")
	v.SetDefault("LLM_CIRCUIT_MAX_TOKENS", 120000)
	v.SetDefault("LLM_CIRCUIT_MAX_COST_USD", 2.50)
	v.SetDefault("LLM_DEFAULT_OUTPUT_TOKENS", 2048)
	v.SetDefault("LLM_MAX_RETRIES", 2)
	v.SetDefault("SANDBOX_RUNTIME", "stub")
	v.SetDefault("SANDBOX_IMAGE", "auto-code-os-sandbox:latest")
	v.SetDefault("SANDBOX_WORKSPACE_ROOT", "/tmp/auto-code-os/workspaces")
	v.SetDefault("SANDBOX_MEMORY_MB", 1024)
	v.SetDefault("SANDBOX_NANO_CPUS", 1000000000)
	v.SetDefault("QUEUE_WORKER_ENABLED", true)
	v.SetDefault("QUEUE_WORKER_INTERVAL_MS", 2000)
	v.SetDefault("QUEUE_WORKER_CONCURRENCY", 1)
	v.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	for _, key := range []string{
		"SERVER_PORT",
		"WEB_PORT",
		"CORS_ALLOWED_ORIGINS",
		"LLM_PROVIDER",
		"LLM_MODEL",
		"LLM_FAST_MODEL",
		"LLM_BALANCED_MODEL",
		"LLM_POWERFUL_MODEL",
		"LLM_ANTHROPIC_BALANCED_MODEL",
		"LLM_ANTHROPIC_POWERFUL_MODEL",
		"LLM_GEMINI_FAST_MODEL",
		"LLM_GEMINI_BALANCED_MODEL",
		"LLM_CIRCUIT_MAX_TOKENS",
		"LLM_CIRCUIT_MAX_COST_USD",
		"LLM_DEFAULT_OUTPUT_TOKENS",
		"LLM_MAX_RETRIES",
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GEMINI_API_KEY",
		"LLM_BASE_URL",
		"LLM_API_KEY",
		"DATABASE_URL",
		"JWT_SECRET",
		"SANDBOX_RUNTIME",
		"SANDBOX_IMAGE",
		"SANDBOX_WORKSPACE_ROOT",
		"SANDBOX_MEMORY_MB",
		"SANDBOX_NANO_CPUS",
		"QUEUE_WORKER_ENABLED",
		"QUEUE_WORKER_INTERVAL_MS",
		"QUEUE_WORKER_CONCURRENCY",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.LLM.Provider = strings.ToLower(cfg.LLM.Provider)
	cfg.Sandbox.Runtime = strings.ToLower(cfg.Sandbox.Runtime)

	switch cfg.LLM.Provider {
	case "openai":
		if cfg.LLM.Model == "" {
			cfg.LLM.Model = "gpt-4o"
		}
		cfg.LLM.APIKey = cfg.LLM.OpenAIAPIKey
	case "anthropic":
		if cfg.LLM.Model == "" {
			cfg.LLM.Model = "claude-sonnet-4-20250514"
		}
		cfg.LLM.APIKey = cfg.LLM.AnthropicAPIKey
	case "gemini":
		if cfg.LLM.Model == "" {
			cfg.LLM.Model = "gemini-2.5-pro"
		}
		cfg.LLM.APIKey = cfg.LLM.GeminiAPIKey
	case "9router":
		if cfg.LLM.Model == "" {
			cfg.LLM.Model = "balanced"
		}
		if cfg.LLM.BaseURL == "" {
			cfg.LLM.BaseURL = "http://localhost:20128/v1"
		}
		cfg.LLM.APIKey = cfg.LLM.LLMAPIKey
	case "gateway":
		if cfg.LLM.OpenAIAPIKey == "" && cfg.LLM.AnthropicAPIKey == "" && cfg.LLM.GeminiAPIKey == "" {
			return nil, fmt.Errorf("LLM_PROVIDER=gateway requires at least one provider API key")
		}
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, anthropic, gemini, 9router, gateway)", cfg.LLM.Provider)
	}

	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("missing DATABASE_URL environment variable")
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, fmt.Errorf("missing JWT_SECRET environment variable")
	}
	if cfg.Sandbox.MemoryMB <= 0 {
		return nil, fmt.Errorf("SANDBOX_MEMORY_MB must be greater than zero")
	}
	if cfg.Sandbox.NanoCPUs <= 0 {
		return nil, fmt.Errorf("SANDBOX_NANO_CPUS must be greater than zero")
	}
	if cfg.Worker.IntervalMS <= 0 {
		return nil, fmt.Errorf("QUEUE_WORKER_INTERVAL_MS must be greater than zero")
	}
	if cfg.Worker.Concurrency <= 0 {
		return nil, fmt.Errorf("QUEUE_WORKER_CONCURRENCY must be greater than zero")
	}

	return &cfg, nil
}
