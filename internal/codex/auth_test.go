package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeAuthFile writes content to a temp auth.json and returns its path.
func writeAuthFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing auth.json: %v", err)
	}
	return path
}

func TestReadAuthChatGPTMode(t *testing.T) {
	path := writeAuthFile(t, `{
		"auth_mode": "chatgpt",
		"tokens": {
			"access_token": "at-123",
			"refresh_token": "rt-456",
			"account_id": "acc-789"
		}
	}`)

	a, err := readAuth(path)
	if err != nil {
		t.Fatalf("readAuth: %v", err)
	}
	if a.mode() != "chatgpt" {
		t.Errorf("mode = %q, want chatgpt", a.mode())
	}
	if got := a.tokenString("access_token"); got != "at-123" {
		t.Errorf("access_token = %q, want at-123", got)
	}
	if got := a.tokenString("account_id"); got != "acc-789" {
		t.Errorf("account_id = %q, want acc-789", got)
	}
}

func TestReadAuthAPIKeyMode(t *testing.T) {
	path := writeAuthFile(t, `{"auth_mode": "apikey", "OPENAI_API_KEY": "sk-real"}`)

	a, err := readAuth(path)
	if err != nil {
		t.Fatalf("readAuth: %v", err)
	}
	if a.mode() != "apikey" {
		t.Errorf("mode = %q, want apikey", a.mode())
	}
	if got := a.directAPIKey(); got != "sk-real" {
		t.Errorf("directAPIKey = %q, want sk-real", got)
	}
}

func TestReadAuthMissingFile(t *testing.T) {
	_, err := readAuth(filepath.Join(t.TempDir(), "nope.json"))
	if !errors.Is(err, ErrNoAuthFile) {
		t.Fatalf("err = %v, want ErrNoAuthFile", err)
	}
}

func TestReadAuthInvalidJSON(t *testing.T) {
	path := writeAuthFile(t, `{not valid json`)
	if _, err := readAuth(path); err == nil {
		t.Fatal("expected error for invalid json, got nil")
	}
}

func TestSaveRoundTripPreservesFieldsAndPerms(t *testing.T) {
	path := writeAuthFile(t, `{
		"auth_mode": "chatgpt",
		"custom_field": "keep-me",
		"tokens": {"access_token": "old", "refresh_token": "rt", "extra": "stay"}
	}`)

	a, err := readAuth(path)
	if err != nil {
		t.Fatalf("readAuth: %v", err)
	}
	a.setToken("access_token", "new")
	a.setLastRefresh("2026-07-01T00:00:00+00:00")
	if err := a.save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Permissions must be 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}

	// Re-read and confirm the update plus preservation of unknown fields.
	raw := map[string]any{}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if raw["custom_field"] != "keep-me" {
		t.Errorf("custom_field lost: %v", raw["custom_field"])
	}
	if raw["last_refresh"] != "2026-07-01T00:00:00+00:00" {
		t.Errorf("last_refresh = %v", raw["last_refresh"])
	}
	toks := raw["tokens"].(map[string]any)
	if toks["access_token"] != "new" {
		t.Errorf("access_token = %v, want new", toks["access_token"])
	}
	if toks["extra"] != "stay" {
		t.Errorf("nested unknown token field lost: %v", toks["extra"])
	}
}
