package dnsserver

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	values := map[string]string{
		"DNS_LISTEN_ADDR":       "0.0.0.0:5353",
		"DNS_REDIS_ADDR":        "redis:6379",
		"DNS_REDIS_PASSWORD":    "secret",
		"DNS_REDIS_DB":          "2",
		"DNS_DEFAULT_TTL":       "120",
		"DNS_LOG_LEVEL":         "WARN",
		"DNS_RECURSION_ENABLED": "true",
		"DNS_SHUTDOWN_TIMEOUT":  "10s",
		"DNS_ZONE_FILE":         "records.json",
	}
	config, err := LoadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.ListenAddr != "0.0.0.0:5353" || config.RedisDB != 2 ||
		config.DefaultTTL != 120 || config.LogLevel != "warn" ||
		!config.RecursionEnabled || config.ZoneFile != "records.json" ||
		config.ShutdownTimeout != 10*time.Second {
		t.Fatalf("LoadConfig() = %+v", config)
	}
}

func TestLoadRecordsAppliesDefaultTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	if err := os.WriteFile(path, []byte(`[{"Name":"example.com.","Type":1,"Value":"192.0.2.1"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	records, err := LoadRecords(path, 90)
	if err != nil {
		t.Fatalf("LoadRecords() error = %v", err)
	}
	if len(records) != 1 || records[0].TTL != 90 {
		t.Fatalf("LoadRecords() = %+v", records)
	}
}
