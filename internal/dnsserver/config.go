package dnsserver

import (
	"encoding/json"
	"fmt"
	"net/netip"
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
	AllowedClients   []netip.Prefix
	RateLimit        float64
	RateBurst        int
	MaxResponseBytes int
	ZoneFile         string
	ShutdownTimeout  time.Duration
}

func DefaultConfig() Config {
	return Config{
		ListenAddr:       "127.0.0.1:8503",
		RedisAddr:        "localhost:6379",
		DefaultTTL:       300,
		LogLevel:         "info",
		ShutdownTimeout:  5 * time.Second,
		AllowedClients:   []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")},
		RateLimit:        50,
		RateBurst:        100,
		MaxResponseBytes: 1232,
	}
}

func LoadConfig(getenv func(string) string) (Config, error) {
	config := DefaultConfig()
	config.ListenAddr = envOr(getenv, "DNS_LISTEN_ADDR", config.ListenAddr)
	config.RedisAddr = envOr(getenv, "DNS_REDIS_ADDR", config.RedisAddr)
	config.RedisPassword = getenv("DNS_REDIS_PASSWORD")
	config.ZoneFile = getenv("DNS_ZONE_FILE")
	config.LogLevel = strings.ToLower(envOr(getenv, "DNS_LOG_LEVEL", config.LogLevel))
	if value := getenv("DNS_ALLOWED_CLIENTS"); value != "" {
		allowed, parseErr := parsePrefixes(value)
		if parseErr != nil {
			return Config{}, parseErr
		}
		config.AllowedClients = allowed
	}

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
	if value := getenv("DNS_RATE_LIMIT"); value != "" {
		config.RateLimit, err = strconv.ParseFloat(value, 64)
		if err != nil || config.RateLimit <= 0 {
			return Config{}, fmt.Errorf("DNS_RATE_LIMIT must be a positive number")
		}
	}
	if value := getenv("DNS_RATE_BURST"); value != "" {
		config.RateBurst, err = strconv.Atoi(value)
		if err != nil || config.RateBurst <= 0 {
			return Config{}, fmt.Errorf("DNS_RATE_BURST must be a positive integer")
		}
	}
	if value := getenv("DNS_MAX_RESPONSE_BYTES"); value != "" {
		config.MaxResponseBytes, err = strconv.Atoi(value)
		if err != nil || config.MaxResponseBytes < 512 {
			return Config{}, fmt.Errorf("DNS_MAX_RESPONSE_BYTES must be at least 512")
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

func parsePrefixes(value string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, item := range strings.Split(value, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(item))
		if err != nil {
			return nil, fmt.Errorf("DNS_ALLOWED_CLIENTS contains invalid CIDR %q: %w", item, err)
		}
		prefixes = append(prefixes, prefix)
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("DNS_ALLOWED_CLIENTS must not be empty")
	}
	return prefixes, nil
}

func LoadRecords(path string, defaultTTL uint32) ([]Record, error) {
	if path == "" {
		records := DefaultRecords()
		applyDefaultTTL(records, defaultTTL)
		if err := validateRecords(records); err != nil {
			return nil, err
		}
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
	if err := validateRecords(records); err != nil {
		return nil, fmt.Errorf("validate zone file %q: %w", path, err)
	}
	return records, nil
}

func validateRecords(records []Record) error {
	for _, record := range records {
		if err := ValidateRecord(record); err != nil {
			return fmt.Errorf("record %q: %w", record.Name, err)
		}
	}
	return nil
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
