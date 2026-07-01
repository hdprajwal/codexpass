package codex

import (
	"fmt"
	"time"
)

// CodexBaseURL is the OpenAI base URL that a borrowed chatgpt-mode token must be
// used against.
const CodexBaseURL = "https://chatgpt.com/backend-api/codex"

// refreshSkewSeconds is how long before expiry a token is treated as stale.
const refreshSkewSeconds = 30

// Credential is a borrowed OpenAI credential.
type Credential struct {
	APIKey    string // access token (chatgpt) or real API key (apikey)
	AccountID string // ChatGPT account id; chatgpt mode only
	Mode      string // "chatgpt" or "apikey"
}

// BaseURL returns the OpenAI base URL the credential must target, or "" when the
// default api.openai.com applies (apikey mode).
func (c Credential) BaseURL() string {
	if c.Mode == "chatgpt" {
		return CodexBaseURL
	}
	return ""
}

// Borrow reads the Codex credential from the default auth.json location,
// refreshing an expired chatgpt-mode token if necessary.
func Borrow() (Credential, error) {
	return borrowFrom(AuthPath(), time.Now)
}

// borrowFrom is the testable core of Borrow: it takes an explicit auth.json path
// and a clock.
func borrowFrom(path string, now func() time.Time) (Credential, error) {
	a, err := readAuth(path)
	if err != nil {
		return Credential{}, err
	}

	switch a.mode() {
	case "apikey":
		key := a.directAPIKey()
		if key == "" {
			return Credential{}, fmt.Errorf("%w: OPENAI_API_KEY missing in apikey mode. Run `codex login` first", ErrNoTokens)
		}
		return Credential{APIKey: key, Mode: "apikey"}, nil

	case "chatgpt":
		return borrowChatGPT(a, now)

	default:
		return Credential{}, fmt.Errorf("%w %q; expected \"chatgpt\" or \"apikey\"", ErrWrongMode, a.mode())
	}
}

// borrowChatGPT returns the access token when still valid. Refreshing an expired
// token is added in a later slice.
func borrowChatGPT(a *auth, now func() time.Time) (Credential, error) {
	access := a.tokenString("access_token")
	if access == "" {
		return Credential{}, fmt.Errorf("%w in auth.json. Run `codex login` first", ErrNoTokens)
	}
	accountID := a.tokenString("account_id")

	exp, ok := jwtExp(access)
	if ok && now().Unix() < exp-refreshSkewSeconds {
		// Still valid: use as-is. (Matches upstream: unknown expiry -> refresh.)
		return Credential{APIKey: access, AccountID: accountID, Mode: "chatgpt"}, nil
	}

	// Expired or near-expiry: refresh via the OAuth endpoint and persist.
	refreshToken := a.tokenString("refresh_token")
	if refreshToken == "" {
		return Credential{}, fmt.Errorf("%w. Run `codex login` to re-authenticate", ErrNoRefreshToken)
	}

	tokens, err := refreshTokens(refreshToken)
	if err != nil {
		return Credential{}, err
	}
	if tokens.AccessToken != "" {
		a.setToken("access_token", tokens.AccessToken)
	}
	if tokens.IDToken != "" {
		a.setToken("id_token", tokens.IDToken)
	}
	if tokens.RefreshToken != "" {
		a.setToken("refresh_token", tokens.RefreshToken)
	}
	a.setLastRefresh(now().UTC().Format("2006-01-02T15:04:05+00:00"))

	if err := a.save(); err != nil {
		return Credential{}, fmt.Errorf("writing refreshed tokens to %s: %w", a.path, err)
	}

	return Credential{APIKey: a.tokenString("access_token"), AccountID: accountID, Mode: "chatgpt"}, nil
}
