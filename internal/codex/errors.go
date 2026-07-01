package codex

import "errors"

// Sentinel errors returned by this package. They are wrapped with additional
// context (paths, HTTP status, guidance) via fmt.Errorf("%w: ...", ...), so
// callers should match them with errors.Is rather than by string.
var (
	// ErrNoAuthFile means auth.json does not exist (Codex not logged in).
	ErrNoAuthFile = errors.New("codex auth file not found")
	// ErrWrongMode means auth_mode is neither "chatgpt" nor "apikey".
	ErrWrongMode = errors.New("unsupported auth_mode")
	// ErrNoTokens means chatgpt mode but no access token is present.
	ErrNoTokens = errors.New("no ChatGPT tokens found")
	// ErrNoRefreshToken means the token is expired and cannot be refreshed.
	ErrNoRefreshToken = errors.New("no refresh token available")
	// ErrRefreshExpired means the refresh token itself is no longer valid.
	ErrRefreshExpired = errors.New("refresh token is no longer valid")
	// ErrRefreshFailed means the OAuth refresh request failed (HTTP/network).
	ErrRefreshFailed = errors.New("token refresh failed")
)
