package codex

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// jwtExp extracts the "exp" (expiry, Unix seconds) claim from a JWT payload
// without verifying the signature. ok is false when the token is malformed,
// the payload is not valid base64url JSON, or there is no exp claim.
func jwtExp(token string) (exp int64, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return 0, false
	}
	// JWT segments are unpadded base64url; tolerate any stray padding.
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return 0, false
	}
	// exp is a JSON numeric date; decode as float64 to tolerate non-integers.
	var claims struct {
		Exp *float64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == nil {
		return 0, false
	}
	return int64(*claims.Exp), true
}
