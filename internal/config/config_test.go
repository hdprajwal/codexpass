package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromMissingFileIsEmpty(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Host != "" || len(cfg.Models.Aliases) != 0 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadFromParsesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
	  "server": {"host":"127.0.0.1","port":8123,"token":"local","verbose":true},
	  "models": {"cache_ttl_seconds":60,"aliases":{"gpt-local":"gpt-5.4"}}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8123 || cfg.Server.Token != "local" || cfg.Models.Aliases["gpt-local"] != "gpt-5.4" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.Models.CacheTTL() != time.Minute {
		t.Fatalf("ttl = %s", cfg.Models.CacheTTL())
	}
}

func TestPathUsesEnv(t *testing.T) {
	t.Setenv("CODEXPASS_CONFIG", "/tmp/codexpass-test.json")
	if Path() != "/tmp/codexpass-test.json" {
		t.Fatalf("path = %q", Path())
	}
}
