package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// codexHome returns the Codex home directory, honoring $CODEX_HOME and falling
// back to ~/.codex, mirroring the Codex CLI.
func codexHome() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(home, ".codex")
}

// AuthPath returns the resolved path to the Codex auth.json file.
func AuthPath() string {
	return filepath.Join(codexHome(), "auth.json")
}

// auth wraps the parsed auth.json. The full document is kept as a generic map
// so that unknown fields survive a refresh write-back unchanged.
type auth struct {
	path string
	raw  map[string]any
}

// readAuth reads and parses auth.json at path. A missing file yields
// ErrNoAuthFile with guidance to run `codex login`.
func readAuth(path string) (*auth, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w at %s. Run `codex login` first", ErrNoAuthFile, path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &auth{path: path, raw: raw}, nil
}

// mode returns the auth_mode field ("chatgpt", "apikey", or "").
func (a *auth) mode() string {
	s, _ := a.raw["auth_mode"].(string)
	return s
}

// directAPIKey returns the OPENAI_API_KEY field, used in apikey mode.
func (a *auth) directAPIKey() string {
	s, _ := a.raw["OPENAI_API_KEY"].(string)
	return s
}

// tokenString returns tokens[key] as a string, or "" if absent.
func (a *auth) tokenString(key string) string {
	toks, _ := a.raw["tokens"].(map[string]any)
	s, _ := toks[key].(string)
	return s
}

// setToken sets tokens[key], creating the tokens object if needed.
func (a *auth) setToken(key, value string) {
	toks, ok := a.raw["tokens"].(map[string]any)
	if !ok {
		toks = map[string]any{}
		a.raw["tokens"] = toks
	}
	toks[key] = value
}

// setLastRefresh stamps the last_refresh field.
func (a *auth) setLastRefresh(ts string) {
	a.raw["last_refresh"] = ts
}

// save atomically writes auth.json back to disk with 0600 permissions,
// preserving all fields that were read.
func (a *auth) save() error {
	data, err := json.MarshalIndent(a.raw, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(a.path)
	tmp, err := os.CreateTemp(dir, ".auth-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed away

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, a.path)
}
