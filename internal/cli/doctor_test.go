package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorJSONRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	auth := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(auth, []byte(`{"auth_mode":"apikey","OPENAI_API_KEY":"sk-secret-value"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Doctor([]string{"--json"}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "sk-secret-value") {
		t.Fatalf("doctor output leaked secret: %s", out.String())
	}
	var payload struct {
		OK   bool `json:"ok"`
		Auth struct {
			Mode      string `json:"mode"`
			HasAPIKey bool   `json:"has_api_key"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.Auth.Mode != "apikey" || !payload.Auth.HasAPIKey {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestDoctorMissingAuthReturnsError(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	var out bytes.Buffer
	err := Doctor(nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out.String(), "auth file: false") {
		t.Fatalf("output = %q", out.String())
	}
}
