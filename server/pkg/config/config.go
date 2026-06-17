package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Port        string `mapstructure:"port"`
	WebPort     string `mapstructure:"web_port"`
	CORSOrigins string `mapstructure:"cors_allowed_origins"`
}

type DatabaseConfig struct {
	URL                    string `mapstructure:"url"`
	MaxOpenConns           int    `mapstructure:"max_open_conns"`
	MaxIdleConns           int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `mapstructure:"conn_max_lifetime_seconds"`
	ConnMaxIdleTimeSeconds int    `mapstructure:"conn_max_idle_time_seconds"`
}

type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"`
}

type LLMConfig struct {
	Provider               string  `mapstructure:"provider"`
	Model                  string  `mapstructure:"model"`
	FastModel              string  `mapstructure:"fast_model"`
	BalancedModel          string  `mapstructure:"balanced_model"`
	PowerfulModel          string  `mapstructure:"powerful_model"`
	AnthropicBalancedModel string  `mapstructure:"anthropic_balanced_model"`
	AnthropicPowerfulModel string  `mapstructure:"anthropic_powerful_model"`
	GeminiFastModel        string  `mapstructure:"gemini_fast_model"`
	GeminiBalancedModel    string  `mapstructure:"gemini_balanced_model"`
	CircuitMaxTokens       int     `mapstructure:"circuit_max_tokens"`
	CircuitMaxCostUSD      float64 `mapstructure:"circuit_max_cost_usd"`
	DefaultOutputTokens    int     `mapstructure:"default_output_tokens"`
	MaxRetries             int     `mapstructure:"max_retries"`
	OpenAIAPIKey           string  `mapstructure:"openai_api_key"`
	AnthropicAPIKey        string  `mapstructure:"anthropic_api_key"`
	GeminiAPIKey           string  `mapstructure:"gemini_api_key"`
	BaseURL                string  `mapstructure:"base_url"`
	LLMAPIKey              string  `mapstructure:"api_key"`
	EmbeddingModel         string  `mapstructure:"embedding_model"`
	APIKey                 string  // Dynamically populated based on provider
}

type SandboxConfig struct {
	Runtime                         string `mapstructure:"runtime"`
	Image                           string `mapstructure:"image"`
	WorkspaceRoot                   string `mapstructure:"workspace_root"`
	SkillsRoot                      string `mapstructure:"skills_root"`
	WorkspaceRetentionHours         int    `mapstructure:"workspace_retention_hours"`
	WorkspaceCleanupIntervalMinutes int    `mapstructure:"workspace_cleanup_interval_minutes"`
	MemoryMB                        int64  `mapstructure:"memory_mb"`
	NanoCPUs                        int64  `mapstructure:"nano_cpus"`
}

type WorkerConfig struct {
	Enabled     bool `mapstructure:"enabled"`
	IntervalMS  int  `mapstructure:"interval_ms"`
	Concurrency int  `mapstructure:"concurrency"`
}

type TelemetryConfig struct {
	OTLPTraceEndpoint string `mapstructure:"otlp_endpoint"`
}

type LoggingConfig struct {
	LocalRetentionDays int    `mapstructure:"local_retention_days"`
	FileRoot           string `mapstructure:"file_root"`
}

// Config holds all configuration for the application.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Auth      AuthConfig      `mapstructure:"auth"`
	LLM       LLMConfig       `mapstructure:"llm"`
	Sandbox   SandboxConfig   `mapstructure:"sandbox"`
	Worker    WorkerConfig    `mapstructure:"worker"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	Logging   LoggingConfig   `mapstructure:"logging"`
}

//go:embed config.yaml
var defaultConfig []byte

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()

	if err := configure(v); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := normalize(&cfg); err != nil {
		return nil, err
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func configure(v *viper.Viper) error {
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind legacy environment variables to new nested structure
	v.BindEnv("server.port", "SERVER_PORT")
	v.BindEnv("server.web_port", "WEB_PORT")
	v.BindEnv("server.cors_allowed_origins", "CORS_ALLOWED_ORIGINS")
	v.BindEnv("llm.openai_api_key", "OPENAI_API_KEY")
	v.BindEnv("llm.anthropic_api_key", "ANTHROPIC_API_KEY")
	v.BindEnv("llm.gemini_api_key", "GEMINI_API_KEY")
	v.BindEnv("database.max_open_conns", "DB_MAX_OPEN_CONNS")
	v.BindEnv("database.max_idle_conns", "DB_MAX_IDLE_CONNS")
	v.BindEnv("database.conn_max_lifetime_seconds", "DB_CONN_MAX_LIFETIME_SECONDS")
	v.BindEnv("database.conn_max_idle_time_seconds", "DB_CONN_MAX_IDLE_TIME_SECONDS")
	v.BindEnv("auth.jwt_secret", "JWT_SECRET")
	v.BindEnv("worker.enabled", "QUEUE_WORKER_ENABLED")
	v.BindEnv("worker.interval_ms", "QUEUE_WORKER_INTERVAL_MS")
	v.BindEnv("worker.concurrency", "QUEUE_WORKER_CONCURRENCY")
	v.BindEnv("telemetry.otlp_endpoint", "OTEL_EXPORTER_OTLP_ENDPOINT")
	v.BindEnv("logging.local_retention_days", "LOG_LOCAL_RETENTION_DAYS")
	v.BindEnv("logging.file_root", "LOG_FILE_ROOT")

	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(defaultConfig)); err != nil {
		return fmt.Errorf("read default config: %w", err)
	}

	if !isTestProcess() {
		readConfigFile(v)
	}

	return nil
}

func isTestProcess() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") || strings.HasSuffix(os.Args[0], ".test") {
			return true
		}
	}
	return false
}

func readConfigFile(v *viper.Viper) {
	v.SetConfigFile(".env")
	if err := v.MergeInConfig(); err != nil {
		v.SetConfigFile("../.env")
		_ = v.MergeInConfig()
	}
}

func normalize(cfg *Config) error {
	cfg.LLM.Provider = strings.ToLower(cfg.LLM.Provider)
	cfg.Sandbox.Runtime = strings.ToLower(cfg.Sandbox.Runtime)

	if err := configureLLM(&cfg.LLM); err != nil {
		return err
	}
	return nil
}

func configureLLM(cfg *LLMConfig) error {
	switch cfg.Provider {
	case "openai":
		if cfg.Model == "" {
			cfg.Model = "gpt-5.5"
		}
		cfg.APIKey = cfg.OpenAIAPIKey
	case "anthropic":
		if cfg.Model == "" {
			cfg.Model = "claude-sonnet-4-6"
		}
		cfg.APIKey = cfg.AnthropicAPIKey
	case "gemini":
		if cfg.Model == "" {
			cfg.Model = "gemini-3.1-pro-preview"
		}
		cfg.APIKey = cfg.GeminiAPIKey
	case "9router":
		if cfg.Model == "" {
			cfg.Model = "balanced"
		}
		if cfg.BaseURL == "" {
			cfg.BaseURL = "http://localhost:20128/v1"
		}
		cfg.APIKey = cfg.LLMAPIKey
	case "gateway":
		// Gateway can resolve provider credentials from the database at runtime.
		// Environment keys remain supported as a fallback but are not required.
	default:
		return fmt.Errorf("unsupported LLM provider: %s (supported: openai, anthropic, gemini, 9router, gateway)", cfg.Provider)
	}
	return nil
}

func validate(cfg *Config) error {
	if cfg.Database.URL == "" {
		return fmt.Errorf("missing DATABASE_URL environment variable")
	}
	if err := validatePositive("DB_MAX_OPEN_CONNS", cfg.Database.MaxOpenConns); err != nil {
		return err
	}
	if err := validatePositive("DB_MAX_IDLE_CONNS", cfg.Database.MaxIdleConns); err != nil {
		return err
	}
	if cfg.Database.MaxIdleConns > cfg.Database.MaxOpenConns {
		return fmt.Errorf("DB_MAX_IDLE_CONNS cannot exceed DB_MAX_OPEN_CONNS")
	}
	if err := validatePositive("DB_CONN_MAX_LIFETIME_SECONDS", cfg.Database.ConnMaxLifetimeSeconds); err != nil {
		return err
	}
	if err := validatePositive("DB_CONN_MAX_IDLE_TIME_SECONDS", cfg.Database.ConnMaxIdleTimeSeconds); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return fmt.Errorf("missing JWT_SECRET environment variable")
	}
	if err := validatePositiveInt64("SANDBOX_MEMORY_MB", cfg.Sandbox.MemoryMB); err != nil {
		return err
	}
	if err := validatePositiveInt64("SANDBOX_NANO_CPUS", cfg.Sandbox.NanoCPUs); err != nil {
		return err
	}
	if cfg.Sandbox.WorkspaceRetentionHours < 0 {
		return fmt.Errorf("SANDBOX_WORKSPACE_RETENTION_HOURS cannot be negative")
	}
	if err := validatePositive("SANDBOX_WORKSPACE_CLEANUP_INTERVAL_MINUTES", cfg.Sandbox.WorkspaceCleanupIntervalMinutes); err != nil {
		return err
	}
	if err := validatePositive("QUEUE_WORKER_INTERVAL_MS", cfg.Worker.IntervalMS); err != nil {
		return err
	}
	if err := validatePositive("QUEUE_WORKER_CONCURRENCY", cfg.Worker.Concurrency); err != nil {
		return err
	}
	if cfg.Logging.LocalRetentionDays < 0 {
		return fmt.Errorf("LOG_LOCAL_RETENTION_DAYS cannot be negative")
	}
	return nil
}

func validatePositive(name string, value int) error {
	if value <= 0 {
		return fmt.Errorf("%s must be greater than zero", name)
	}
	return nil
}

func validatePositiveInt64(name string, value int64) error {
	if value <= 0 {
		return fmt.Errorf("%s must be greater than zero", name)
	}
	return nil
}
