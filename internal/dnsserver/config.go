package dnsserver

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr       string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	DefaultTTL       uint32
	LogLevel         string
	RecursionEnabled bool
	ZoneFile         string
	ShutdownTimeout  time.Duration
}

func DefaultConfig() Config {
	return Config{
		ListenAddr:      "127.0.0.1:8503",
		RedisAddr:       "localhost:6379",
		DefaultTTL:      300,
		LogLevel:        "info",
		ShutdownTimeout: 5 * time.Second,
	}
}

func LoadConfig(getenv func(string) string) (Config, error) {
	config := DefaultConfig()
	config.ListenAddr = envOr(getenv, "DNS_LISTEN_ADDR", config.ListenAddr)
	config.RedisAddr = envOr(getenv, "DNS_REDIS_ADDR", config.RedisAddr)
	config.RedisPassword = getenv("DNS_REDIS_PASSWORD")
	config.ZoneFile = getenv("DNS_ZONE_FILE")
	config.LogLevel = strings.ToLower(envOr(getenv, "DNS_LOG_LEVEL", config.LogLevel))

	var err error
	if config.RedisDB, err = envInt(getenv, "DNS_REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if value := getenv("DNS_DEFAULT_TTL"); value != "" {
		ttl, parseErr := strconv.ParseUint(value, 10, 32)
		if parseErr != nil {
			return Config{}, fmt.Errorf("DNS_DEFAULT_TTL must be an unsigned integer: %w", parseErr)
		}
		config.DefaultTTL = uint32(ttl)
	}
	if value := getenv("DNS_RECURSION_ENABLED"); value != "" {
		config.RecursionEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("DNS_RECURSION_ENABLED must be true or false: %w", err)
		}
	}
	if value := getenv("DNS_SHUTDOWN_TIMEOUT"); value != "" {
		config.ShutdownTimeout, err = time.ParseDuration(value)
		if err != nil || config.ShutdownTimeout <= 0 {
			return Config{}, fmt.Errorf("DNS_SHUTDOWN_TIMEOUT must be a positive duration")
		}
	}
	switch config.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("DNS_LOG_LEVEL must be one of debug, info, warn, or error")
	}
	if config.RedisDB < 0 {
		return Config{}, fmt.Errorf("DNS_REDIS_DB must not be negative")
	}
	return config, nil
}

func LoadRecords(path string, defaultTTL uint32) ([]Record, error) {
	if path == "" {
		records := DefaultRecords()
		applyDefaultTTL(records, defaultTTL)
		return records, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read zone file %q: %w", path, err)
	}
	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parse zone file %q as JSON: %w", path, err)
	}
	applyDefaultTTL(records, defaultTTL)
	return records, nil
}

func applyDefaultTTL(records []Record, defaultTTL uint32) {
	for index := range records {
		if records[index].TTL == 0 {
			records[index].TTL = defaultTTL
		}
	}
}

func envOr(getenv func(string) string, key, fallback string) string {
	if value := getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(getenv func(string) string, key string, fallback int) (int, error) {
	value := getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}
