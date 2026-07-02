package cli

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// CreateClientToken prints a local client-token config snippet.
func CreateClientToken(name string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("client name is required")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	snippet := map[string]any{
		"clients": map[string]any{
			name: map[string]any{
				"token":             token,
				"allowed_endpoints": []string{"models", "chat.completions", "responses"},
			},
		},
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(snippet)
}
