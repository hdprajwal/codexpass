package codex

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// withOAuthServer points the package OAuth endpoint at a test server for the
// duration of the test.
func withOAuthServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	prev := oauthEndpoint
	oauthEndpoint = srv.URL
	t.Cleanup(func() {
		oauthEndpoint = prev
		srv.Close()
	})
	return srv
}

func TestBorrowRefreshesExpiredTokenAndWritesBack(t *testing.T) {
	expired := makeToken(`{"exp":100}`) // long past relative to clock @ 1000
	path := writeAuthFile(t, `{
		"auth_mode": "chatgpt",
		"tokens": {"access_token": "`+expired+`", "refresh_token": "rt-old", "account_id": "acc-1"}
	}`)

	var gotBody map[string]string
	withOAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"access_token":"at-new","id_token":"id-new","refresh_token":"rt-new"}`)
	})

	cred, err := borrowFrom(path, fixedClock(1000))
	if err != nil {
		t.Fatalf("borrowFrom: %v", err)
	}
	if cred.APIKey != "at-new" {
		t.Errorf("APIKey = %q, want at-new", cred.APIKey)
	}
	if cred.AccountID != "acc-1" {
		t.Errorf("AccountID = %q, want acc-1", cred.AccountID)
	}

	// Request carried the client id, grant type, and old refresh token.
	if gotBody["client_id"] != clientID {
		t.Errorf("client_id = %q, want %q", gotBody["client_id"], clientID)
	}
	if gotBody["grant_type"] != "refresh_token" {
		t.Errorf("grant_type = %q, want refresh_token", gotBody["grant_type"])
	}
	if gotBody["refresh_token"] != "rt-old" {
		t.Errorf("refresh_token = %q, want rt-old", gotBody["refresh_token"])
	}

	// auth.json now holds the new tokens and a last_refresh stamp.
	raw := map[string]any{}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	toks := raw["tokens"].(map[string]any)
	if toks["access_token"] != "at-new" {
		t.Errorf("persisted access_token = %v, want at-new", toks["access_token"])
	}
	if toks["refresh_token"] != "rt-new" {
		t.Errorf("persisted refresh_token = %v, want rt-new", toks["refresh_token"])
	}
	if toks["id_token"] != "id-new" {
		t.Errorf("persisted id_token = %v, want id-new", toks["id_token"])
	}
	if raw["last_refresh"] == nil || raw["last_refresh"] == "" {
		t.Errorf("last_refresh not stamped: %v", raw["last_refresh"])
	}
}

func TestBorrowRefreshExpiredRefreshToken(t *testing.T) {
	expired := makeToken(`{"exp":100}`)
	path := writeAuthFile(t, `{
		"auth_mode": "chatgpt",
		"tokens": {"access_token": "`+expired+`", "refresh_token": "rt-dead"}
	}`)

	withOAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":"refresh_token_expired"}`)
	})

	_, err := borrowFrom(path, fixedClock(1000))
	if !errors.Is(err, ErrRefreshExpired) {
		t.Fatalf("err = %v, want ErrRefreshExpired", err)
	}
}

func TestBorrowRefreshServerError(t *testing.T) {
	expired := makeToken(`{"exp":100}`)
	path := writeAuthFile(t, `{
		"auth_mode": "chatgpt",
		"tokens": {"access_token": "`+expired+`", "refresh_token": "rt"}
	}`)

	withOAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "boom")
	})

	_, err := borrowFrom(path, fixedClock(1000))
	if !errors.Is(err, ErrRefreshFailed) {
		t.Fatalf("err = %v, want ErrRefreshFailed", err)
	}
}

func TestBorrowExpiredNoRefreshToken(t *testing.T) {
	expired := makeToken(`{"exp":100}`)
	path := writeAuthFile(t, `{
		"auth_mode": "chatgpt",
		"tokens": {"access_token": "`+expired+`"}
	}`)

	// No server needed: refresh must not be attempted without a refresh token.
	_, err := borrowFrom(path, fixedClock(1000))
	if !errors.Is(err, ErrNoRefreshToken) {
		t.Fatalf("err = %v, want ErrNoRefreshToken", err)
	}
}
