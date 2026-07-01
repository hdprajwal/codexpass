package codex

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// fixedClock returns a clock stuck at the given Unix second.
func fixedClock(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0) }
}

func TestBorrowChatGPTValidToken(t *testing.T) {
	// Token expires at 100000; clock at 1000 -> well within validity.
	access := makeToken(`{"exp":100000}`)
	path := writeAuthFile(t, `{
		"auth_mode": "chatgpt",
		"tokens": {"access_token": "`+access+`", "account_id": "acc-1"}
	}`)

	cred, err := borrowFrom(path, fixedClock(1000))
	if err != nil {
		t.Fatalf("borrowFrom: %v", err)
	}
	if cred.APIKey != access {
		t.Errorf("APIKey = %q, want the access token", cred.APIKey)
	}
	if cred.AccountID != "acc-1" {
		t.Errorf("AccountID = %q, want acc-1", cred.AccountID)
	}
	if cred.Mode != "chatgpt" {
		t.Errorf("Mode = %q, want chatgpt", cred.Mode)
	}
	if cred.BaseURL() != CodexBaseURL {
		t.Errorf("BaseURL = %q, want %q", cred.BaseURL(), CodexBaseURL)
	}
}

func TestBorrowAPIKeyMode(t *testing.T) {
	path := writeAuthFile(t, `{"auth_mode": "apikey", "OPENAI_API_KEY": "sk-real"}`)

	cred, err := borrowFrom(path, fixedClock(1000))
	if err != nil {
		t.Fatalf("borrowFrom: %v", err)
	}
	if cred.APIKey != "sk-real" {
		t.Errorf("APIKey = %q, want sk-real", cred.APIKey)
	}
	if cred.Mode != "apikey" {
		t.Errorf("Mode = %q, want apikey", cred.Mode)
	}
	if cred.BaseURL() != "" {
		t.Errorf("BaseURL = %q, want empty for apikey mode", cred.BaseURL())
	}
}

func TestBorrowAPIKeyModeMissingKey(t *testing.T) {
	path := writeAuthFile(t, `{"auth_mode": "apikey", "OPENAI_API_KEY": ""}`)
	_, err := borrowFrom(path, fixedClock(1000))
	if !errors.Is(err, ErrNoTokens) {
		t.Fatalf("err = %v, want ErrNoTokens", err)
	}
}

func TestBorrowChatGPTMissingAccessToken(t *testing.T) {
	path := writeAuthFile(t, `{"auth_mode": "chatgpt", "tokens": {"refresh_token": "rt"}}`)
	_, err := borrowFrom(path, fixedClock(1000))
	if !errors.Is(err, ErrNoTokens) {
		t.Fatalf("err = %v, want ErrNoTokens", err)
	}
}

func TestBorrowWrongMode(t *testing.T) {
	path := writeAuthFile(t, `{"auth_mode": "carrier-pigeon"}`)
	_, err := borrowFrom(path, fixedClock(1000))
	if !errors.Is(err, ErrWrongMode) {
		t.Fatalf("err = %v, want ErrWrongMode", err)
	}
}

func TestBorrowMissingFile(t *testing.T) {
	_, err := borrowFrom(filepath.Join(t.TempDir(), "nope.json"), fixedClock(1000))
	if !errors.Is(err, ErrNoAuthFile) {
		t.Fatalf("err = %v, want ErrNoAuthFile", err)
	}
}
