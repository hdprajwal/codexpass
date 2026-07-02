package codex

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestInspectChatGPTRedacted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	exp := time.Now().Add(time.Hour).Unix()
	access := testJWT(exp)
	data := `{
  "auth_mode": "chatgpt",
  "tokens": {
    "access_token": "` + access + `",
    "refresh_token": "refresh-secret",
    "id_token": "id-secret",
    "account_id": "acct_123"
  }
}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := Inspect(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != "chatgpt" || !report.HasAccessToken || !report.HasRefreshToken || !report.HasAccountID {
		t.Fatalf("report = %+v", report)
	}
	if report.AccessTokenExpiry == nil || report.AccessTokenExpired {
		t.Fatalf("expiry = %v expired=%t", report.AccessTokenExpiry, report.AccessTokenExpired)
	}
	if strings.Contains(strings.Join(report.Problems, " "), "refresh-secret") {
		t.Fatal("report leaked refresh token")
	}
}

func TestInspectMissingRefreshForExpiredToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	data := `{"auth_mode":"chatgpt","tokens":{"access_token":"` + testJWT(time.Now().Add(-time.Hour).Unix()) + `"}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := Inspect(path, time.Now())
	if err != ErrNoRefreshToken {
		t.Fatalf("err = %v, want ErrNoRefreshToken", err)
	}
	if !report.AccessTokenExpired || len(report.Problems) == 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestInspectAPIKeyMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte(`{"auth_mode":"apikey","OPENAI_API_KEY":"sk-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := Inspect(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != "apikey" || !report.HasAPIKey || report.BaseURL != "" {
		t.Fatalf("report = %+v", report)
	}
}

func testJWT(exp int64) string {
	payload := `{"exp":` + strconv.FormatInt(exp, 10) + `}`
	return "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}
