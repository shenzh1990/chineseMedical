package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AppName  string
	Env      string
	LogLevel slog.Level
	Path     string `yaml:"-"`
	HTTP     HTTPConfig
	Database DatabaseConfig
	Redis    RedisConfig
	AI       AIConfig
}

type HTTPConfig struct {
	Host            string
	Port            string
	Addr            string `yaml:"-"`
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	ConnectTimeout  time.Duration
	HealthCheckTime time.Duration
}

type RedisConfig struct {
	Addr     string
	Username string
	Password string
	DB       int
}

type AIConfig struct {
	BaseURL              string
	EndpointPath         string
	APIKey               string
	APIKeyEnv            string
	Model                string
	ImageCount           int
	Size                 string
	Quality              string
	OutputFormat         string
	OutputDir            string
	Timeout              time.Duration
	ResearchBaseURL      string
	ResearchEndpointPath string
	ResearchAPIKey       string
	ResearchAPIKeyEnv    string
	ResearchModel        string
	ResearchToolType     string
	ResearchContextSize  string
	ResearchTimeout      time.Duration
}

type rawConfig struct {
	AppName  string            `yaml:"app_name"`
	Env      string            `yaml:"env"`
	LogLevel string            `yaml:"log_level"`
	HTTP     rawHTTPConfig     `yaml:"http"`
	Database rawDatabaseConfig `yaml:"database"`
	Redis    RedisConfig       `yaml:"redis"`
	AI       rawAIConfig       `yaml:"ai"`
}

type rawHTTPConfig struct {
	Host            string `yaml:"host"`
	Port            string `yaml:"port"`
	ShutdownTimeout string `yaml:"shutdown_timeout"`
}

type rawDatabaseConfig struct {
	URL             string `yaml:"url"`
	MaxConns        int32  `yaml:"max_conns"`
	MinConns        int32  `yaml:"min_conns"`
	ConnectTimeout  string `yaml:"connect_timeout"`
	HealthCheckTime string `yaml:"health_check_time"`
}

type rawAIConfig struct {
	BaseURL              string `yaml:"base_url"`
	EndpointPath         string `yaml:"endpoint_path"`
	APIKey               string `yaml:"api_key"`
	APIKeyEnv            string `yaml:"api_key_env"`
	Model                string `yaml:"model"`
	ImageCount           int    `yaml:"image_count"`
	Size                 string `yaml:"size"`
	Quality              string `yaml:"quality"`
	OutputFormat         string `yaml:"output_format"`
	OutputDir            string `yaml:"output_dir"`
	Timeout              string `yaml:"timeout"`
	ResearchBaseURL      string `yaml:"research_base_url"`
	ResearchEndpointPath string `yaml:"research_endpoint_path"`
	ResearchAPIKey       string `yaml:"research_api_key"`
	ResearchAPIKeyEnv    string `yaml:"research_api_key_env"`
	ResearchModel        string `yaml:"research_model"`
	ResearchToolType     string `yaml:"research_tool_type"`
	ResearchContextSize  string `yaml:"research_context_size"`
	ResearchTimeout      string `yaml:"research_timeout"`
}

func Load() (Config, error) {
	path := strings.TrimSpace(os.Getenv("CONFIG_FILE"))
	if path == "" {
		path = "configs/config.yaml"
	}

	return LoadFile(path)
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	raw := defaultRawConfig()
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config file %q: %w", path, err)
	}

	cfg, err := normalize(raw)
	if err != nil {
		return Config{}, fmt.Errorf("load config file %q: %w", path, err)
	}
	cfg.Path = path

	return cfg, nil
}

func WriteAIConfig(path string, ai AIConfig) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	raw := defaultRawConfig()
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}

	raw.AI = rawAIConfig{
		BaseURL:              ai.BaseURL,
		EndpointPath:         ai.EndpointPath,
		APIKey:               ai.APIKey,
		APIKeyEnv:            ai.APIKeyEnv,
		Model:                ai.Model,
		ImageCount:           ai.ImageCount,
		Size:                 ai.Size,
		Quality:              ai.Quality,
		OutputFormat:         ai.OutputFormat,
		OutputDir:            ai.OutputDir,
		Timeout:              ai.Timeout.String(),
		ResearchBaseURL:      ai.ResearchBaseURL,
		ResearchEndpointPath: ai.ResearchEndpointPath,
		ResearchAPIKey:       ai.ResearchAPIKey,
		ResearchAPIKeyEnv:    ai.ResearchAPIKeyEnv,
		ResearchModel:        ai.ResearchModel,
		ResearchToolType:     ai.ResearchToolType,
		ResearchContextSize:  ai.ResearchContextSize,
		ResearchTimeout:      ai.ResearchTimeout.String(),
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal config file %q: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}
	return nil
}

func defaultRawConfig() rawConfig {
	return rawConfig{
		AppName:  "chinese-medical",
		Env:      "development",
		LogLevel: "info",
		HTTP: rawHTTPConfig{
			Host:            "0.0.0.0",
			Port:            "8080",
			ShutdownTimeout: "10s",
		},
		Database: rawDatabaseConfig{
			URL:             "postgres://postgres:postgres@localhost:5432/chinese_medical?sslmode=disable",
			MaxConns:        10,
			MinConns:        1,
			ConnectTimeout:  "5s",
			HealthCheckTime: "30s",
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
			DB:   0,
		},
		AI: rawAIConfig{
			BaseURL:              "https://api.openai.com/v1",
			EndpointPath:         "/images/generations",
			APIKeyEnv:            "OPENAI_API_KEY",
			Model:                "gpt-image-1",
			ImageCount:           4,
			Size:                 "720x1280",
			Quality:              "medium",
			OutputFormat:         "png",
			OutputDir:            "generated",
			Timeout:              "240s",
			ResearchEndpointPath: "/responses",
			ResearchModel:        "gpt-5.5",
			ResearchToolType:     "web_search",
			ResearchContextSize:  "medium",
			ResearchTimeout:      "90s",
		},
	}
}

func normalize(raw rawConfig) (Config, error) {
	shutdownTimeout, err := parseDuration("http.shutdown_timeout", raw.HTTP.ShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	connectTimeout, err := parseDuration("database.connect_timeout", raw.Database.ConnectTimeout)
	if err != nil {
		return Config{}, err
	}

	healthCheckTime, err := parseDuration("database.health_check_time", raw.Database.HealthCheckTime)
	if err != nil {
		return Config{}, err
	}

	aiTimeout, err := parseDuration("ai.timeout", raw.AI.Timeout)
	if err != nil {
		return Config{}, err
	}
	researchTimeout, err := parseDuration("ai.research_timeout", fallback(raw.AI.ResearchTimeout, "90s"))
	if err != nil {
		return Config{}, err
	}

	host := strings.TrimSpace(raw.HTTP.Host)
	port := strings.TrimSpace(raw.HTTP.Port)
	if host == "" {
		host = "0.0.0.0"
	}
	if port == "" {
		port = "8080"
	}

	databaseURL := strings.TrimSpace(raw.Database.URL)
	if databaseURL == "" {
		return Config{}, fmt.Errorf("database.url is required")
	}

	redisAddr := strings.TrimSpace(raw.Redis.Addr)
	if redisAddr == "" {
		return Config{}, fmt.Errorf("redis.addr is required")
	}

	return Config{
		AppName:  fallback(raw.AppName, "chinese-medical"),
		Env:      fallback(raw.Env, "development"),
		LogLevel: parseLogLevel(raw.LogLevel),
		HTTP: HTTPConfig{
			Host:            host,
			Port:            port,
			Addr:            net.JoinHostPort(host, port),
			ShutdownTimeout: shutdownTimeout,
		},
		Database: DatabaseConfig{
			URL:             databaseURL,
			MaxConns:        positiveInt32(raw.Database.MaxConns, 10),
			MinConns:        positiveInt32(raw.Database.MinConns, 1),
			ConnectTimeout:  connectTimeout,
			HealthCheckTime: healthCheckTime,
		},
		Redis: RedisConfig{
			Addr:     redisAddr,
			Username: raw.Redis.Username,
			Password: raw.Redis.Password,
			DB:       raw.Redis.DB,
		},
		AI: AIConfig{
			BaseURL:              strings.TrimRight(fallback(raw.AI.BaseURL, "https://api.openai.com/v1"), "/"),
			EndpointPath:         normalizeEndpointPath(raw.AI.EndpointPath),
			APIKey:               strings.TrimSpace(raw.AI.APIKey),
			APIKeyEnv:            strings.TrimSpace(raw.AI.APIKeyEnv),
			Model:                fallback(raw.AI.Model, "gpt-image-1"),
			ImageCount:           positiveInt(raw.AI.ImageCount, 4),
			Size:                 fallback(raw.AI.Size, "720x1280"),
			Quality:              fallback(raw.AI.Quality, "medium"),
			OutputFormat:         fallback(raw.AI.OutputFormat, "png"),
			OutputDir:            fallback(raw.AI.OutputDir, "generated"),
			Timeout:              aiTimeout,
			ResearchBaseURL:      strings.TrimRight(fallback(raw.AI.ResearchBaseURL, fallback(raw.AI.BaseURL, "https://api.openai.com/v1")), "/"),
			ResearchEndpointPath: normalizeEndpointPath(fallback(raw.AI.ResearchEndpointPath, "/responses")),
			ResearchAPIKey:       strings.TrimSpace(raw.AI.ResearchAPIKey),
			ResearchAPIKeyEnv:    strings.TrimSpace(raw.AI.ResearchAPIKeyEnv),
			ResearchModel:        fallback(raw.AI.ResearchModel, "gpt-5.5"),
			ResearchToolType:     fallback(raw.AI.ResearchToolType, "web_search"),
			ResearchContextSize:  fallback(raw.AI.ResearchContextSize, "medium"),
			ResearchTimeout:      researchTimeout,
		},
	}, nil
}

func normalizeEndpointPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/images/generations"
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}

func fallback(value string, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}

func positiveInt32(value, defaultValue int32) int32 {
	if value <= 0 {
		return defaultValue
	}
	return value
}

func positiveInt(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}

func parseDuration(name, value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("%s is required", name)
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
