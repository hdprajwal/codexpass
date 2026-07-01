package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	refreshURL = "https://auth.openai.com/oauth/token"
	// clientID is the public Codex OAuth client id (same value the Codex CLI
	// and the upstream Python plugin use).
	clientID = "app_EMoamEEZ73f0CkXaXp7hrann"
)

// Overridable for tests.
var (
	oauthEndpoint = refreshURL
	httpClient    = &http.Client{Timeout: 30 * time.Second}
)

// refreshResponse is the subset of the OAuth token response we consume.
type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

// refreshTokens exchanges a refresh_token for a fresh set of tokens.
func refreshTokens(refreshToken string) (refreshResponse, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     clientID,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	})
	if err != nil {
		return refreshResponse{}, fmt.Errorf("%w: %v", ErrRefreshFailed, err)
	}

	req, err := http.NewRequest(http.MethodPost, oauthEndpoint, bytes.NewReader(body))
	if err != nil {
		return refreshResponse{}, fmt.Errorf("%w: %v", ErrRefreshFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return refreshResponse{}, fmt.Errorf("%w (network error): %v", ErrRefreshFailed, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		if code := parseOAuthError(data); isRefreshInvalid(code) {
			return refreshResponse{}, fmt.Errorf("%w (%s). Run `codex login` to re-authenticate", ErrRefreshExpired, code)
		}
		return refreshResponse{}, fmt.Errorf("%w (HTTP %d): %s", ErrRefreshFailed, resp.StatusCode, string(data))
	}

	var out refreshResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return refreshResponse{}, fmt.Errorf("%w: invalid response body: %v", ErrRefreshFailed, err)
	}
	return out, nil
}

// parseOAuthError extracts the "error" code from an OAuth error body, if any.
func parseOAuthError(body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	return e.Error
}

// isRefreshInvalid reports whether an OAuth error code means the refresh token
// itself is dead and the user must log in again.
func isRefreshInvalid(code string) bool {
	switch code {
	case "refresh_token_expired", "refresh_token_reused", "refresh_token_invalidated":
		return true
	default:
		return false
	}
}
