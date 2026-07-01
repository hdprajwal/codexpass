package cli

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureCodexHome writes an auth.json into a temp CODEX_HOME and points the
// environment at it for the duration of the test.
func fixtureCodexHome(t *testing.T, authJSON string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(authJSON), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	t.Setenv("CODEX_HOME", dir)
}

// futureToken builds a JWT whose exp is far in the future so Borrow uses it
// as-is without attempting a network refresh.
func futureToken() string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":9999999999}`))
	return "h." + payload + ".s"
}

func TestExportChatGPTMode(t *testing.T) {
	tok := futureToken()
	fixtureCodexHome(t, `{"auth_mode":"chatgpt","tokens":{"access_token":"`+tok+`","account_id":"acc-1"}}`)

	var out, notes bytes.Buffer
	if err := Export(&out, &notes, ExportOptions{}); err != nil {
		t.Fatalf("Export: %v", err)
	}

	stdout := out.String()
	if !strings.Contains(stdout, "export OPENAI_API_KEY='"+tok+"'\n") {
		t.Errorf("missing api key line, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "export OPENAI_BASE_URL='https://chatgpt.com/backend-api/codex'\n") {
		t.Errorf("missing base url line, got:\n%s", stdout)
	}
	// The account-id caveat must be on notes (stderr), never stdout.
	if strings.Contains(stdout, "ChatGPT-Account-ID") {
		t.Errorf("caveat leaked into stdout:\n%s", stdout)
	}
	if !strings.Contains(notes.String(), "acc-1") {
		t.Errorf("expected account id in notes, got: %q", notes.String())
	}
}

func TestExportChatGPTModeNoBaseURL(t *testing.T) {
	tok := futureToken()
	fixtureCodexHome(t, `{"auth_mode":"chatgpt","tokens":{"access_token":"`+tok+`","account_id":"acc-1"}}`)

	var out, notes bytes.Buffer
	if err := Export(&out, &notes, ExportOptions{NoBaseURL: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if strings.Contains(out.String(), "OPENAI_BASE_URL") {
		t.Errorf("--no-base-url still emitted base url:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "OPENAI_API_KEY") {
		t.Errorf("missing api key line:\n%s", out.String())
	}
}

func TestExportAPIKeyMode(t *testing.T) {
	fixtureCodexHome(t, `{"auth_mode":"apikey","OPENAI_API_KEY":"sk-real"}`)

	var out, notes bytes.Buffer
	if err := Export(&out, &notes, ExportOptions{}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.Contains(out.String(), "export OPENAI_API_KEY='sk-real'\n") {
		t.Errorf("missing api key line:\n%s", out.String())
	}
	if strings.Contains(out.String(), "OPENAI_BASE_URL") {
		t.Errorf("apikey mode must not override base url:\n%s", out.String())
	}
	if notes.Len() != 0 {
		t.Errorf("apikey mode should emit no notes, got: %q", notes.String())
	}
}

func TestShellSingleQuote(t *testing.T) {
	tests := map[string]string{
		"plain":       "'plain'",
		"sk-abc123":   "'sk-abc123'",
		"has'quote":   `'has'\''quote'`,
		"a.b_c-d/e+f": "'a.b_c-d/e+f'",
	}
	for in, want := range tests {
		if got := shellSingleQuote(in); got != want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
