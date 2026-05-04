package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App   AppConfig
	HTTP  HTTPConfig
	MySQL MySQLConfig
	Redis RedisConfig
	AI    AIConfig
	Auth  AuthConfig
}

type AppConfig struct {
	Name     string
	Env      string
	LogLevel string
}

type HTTPConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type MySQLConfig struct {
	Enabled     bool
	Driver      string
	DSN         string
	PingTimeout time.Duration
	AutoMigrate bool
}

type RedisConfig struct {
	Addr        string
	PingTimeout time.Duration
}

type AIConfig struct {
	HTTPBaseURL         string
	HTTPTimeout         time.Duration
	IndexTimeout        time.Duration
	GRPCTarget          string
	PingTimeout         time.Duration
	RuntimeSharedSecret string
}

type AuthConfig struct {
	JWTSecret      string
	TokenExpiresIn time.Duration
}

func Load() Config {
	return Config{
		App: AppConfig{
			Name:     getEnv("APP_NAME", "go-api"),
			Env:      getEnv("APP_ENV", "dev"),
			LogLevel: getEnv("LOG_LEVEL", "INFO"),
		},
		HTTP: HTTPConfig{
			Host:         getEnv("HTTP_HOST", "0.0.0.0"),
			Port:         getEnv("HTTP_PORT", "8080"),
			ReadTimeout:  getDuration("HTTP_READ_TIMEOUT_SECONDS", 10*time.Second),
			WriteTimeout: getDuration("HTTP_WRITE_TIMEOUT_SECONDS", 15*time.Second),
			IdleTimeout:  getDuration("HTTP_IDLE_TIMEOUT_SECONDS", 30*time.Second),
		},
		MySQL: MySQLConfig{
			Enabled:     getBool("MYSQL_ENABLED", true),
			Driver:      getEnv("MYSQL_DRIVER", "mysql"),
			DSN:         getEnv("MYSQL_DSN", "oncall:oncall@tcp(127.0.0.1:3306)/oncall?charset=utf8mb4&parseTime=True&loc=Local"),
			PingTimeout: getDuration("MYSQL_PING_TIMEOUT_SECONDS", 2*time.Second),
			AutoMigrate: getBool("MYSQL_AUTO_MIGRATE", true),
		},
		Redis: RedisConfig{
			Addr:        getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			PingTimeout: getDuration("REDIS_PING_TIMEOUT_SECONDS", 2*time.Second),
		},
		AI: AIConfig{
			HTTPBaseURL:         getEnv("AI_HTTP_BASE_URL", "http://127.0.0.1:8000"),
			HTTPTimeout:         getDuration("AI_HTTP_TIMEOUT_SECONDS", 8*time.Second),
			IndexTimeout:        getDuration("AI_INDEX_TIMEOUT_SECONDS", 25*time.Second),
			GRPCTarget:          getEnv("GRPC_AI_TARGET", "127.0.0.1:50051"),
			PingTimeout:         getDuration("AI_GRPC_PING_TIMEOUT_SECONDS", 2*time.Second),
			RuntimeSharedSecret: getEnv("RUNTIME_SHARED_SECRET", "oncall-runtime-secret"),
		},
		Auth: AuthConfig{
			JWTSecret:      getEnv("JWT_SECRET", "oncall-dev-secret"),
			TokenExpiresIn: getDuration("JWT_EXPIRES_IN_SECONDS", 24*time.Hour),
		},
	}
}

func (c Config) HTTPAddress() string {
	return c.HTTP.Host + ":" + c.HTTP.Port
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
