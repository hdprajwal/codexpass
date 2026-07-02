package codex

import (
	"fmt"
	"os"
	"time"
)

// AuthReport is a redacted, read-only summary of the Codex auth file.
type AuthReport struct {
	Path               string     `json:"path"`
	Exists             bool       `json:"exists"`
	Mode               string     `json:"mode,omitempty"`
	FileMode           string     `json:"file_mode,omitempty"`
	FilePermissionRisk bool       `json:"file_permission_risk"`
	HasAccessToken     bool       `json:"has_access_token"`
	HasRefreshToken    bool       `json:"has_refresh_token"`
	HasIDToken         bool       `json:"has_id_token"`
	HasAccountID       bool       `json:"has_account_id"`
	HasAPIKey          bool       `json:"has_api_key"`
	AccessTokenExpiry  *time.Time `json:"access_token_expiry,omitempty"`
	AccessTokenExpired bool       `json:"access_token_expired"`
	AccessTokenUnknown bool       `json:"access_token_expiry_unknown"`
	BaseURL            string     `json:"base_url,omitempty"`
	Problems           []string   `json:"problems,omitempty"`
}

// Inspect reads an auth.json file without refreshing or exposing secrets.
func Inspect(path string, now time.Time) (AuthReport, error) {
	report := AuthReport{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			report.Problems = append(report.Problems, "auth file not found; run `codex login` first")
			return report, ErrNoAuthFile
		}
		report.Problems = append(report.Problems, fmt.Sprintf("could not stat auth file: %v", err))
		return report, err
	}
	report.Exists = true
	report.FileMode = info.Mode().Perm().String()
	if info.Mode().Perm()&0o077 != 0 {
		report.FilePermissionRisk = true
		report.Problems = append(report.Problems, "auth file is readable by group or others")
	}

	a, err := readAuth(path)
	if err != nil {
		report.Problems = append(report.Problems, err.Error())
		return report, err
	}
	report.Mode = a.mode()
	switch report.Mode {
	case "apikey":
		report.HasAPIKey = a.directAPIKey() != ""
		if !report.HasAPIKey {
			report.Problems = append(report.Problems, "OPENAI_API_KEY missing in apikey mode")
			return report, ErrNoTokens
		}
	case "chatgpt":
		access := a.tokenString("access_token")
		report.HasAccessToken = access != ""
		report.HasRefreshToken = a.tokenString("refresh_token") != ""
		report.HasIDToken = a.tokenString("id_token") != ""
		report.HasAccountID = a.tokenString("account_id") != ""
		report.BaseURL = CodexBaseURL
		if !report.HasAccessToken {
			report.Problems = append(report.Problems, "access_token missing in chatgpt mode")
			return report, ErrNoTokens
		}
		if exp, ok := jwtExp(access); ok {
			t := time.Unix(exp, 0).UTC()
			report.AccessTokenExpiry = &t
			report.AccessTokenExpired = now.Unix() >= exp-refreshSkewSeconds
		} else {
			report.AccessTokenUnknown = true
			report.AccessTokenExpired = true
		}
		if report.AccessTokenExpired && !report.HasRefreshToken {
			report.Problems = append(report.Problems, "access token is expired or unknown and refresh_token is missing")
			return report, ErrNoRefreshToken
		}
		if !report.HasAccountID {
			report.Problems = append(report.Problems, "account_id missing; Codex backend may reject requests")
		}
	default:
		report.Problems = append(report.Problems, fmt.Sprintf("unsupported auth_mode %q", report.Mode))
		return report, ErrWrongMode
	}
	return report, nil
}

// InspectDefault reads the default Codex auth path.
func InspectDefault(now time.Time) (AuthReport, error) {
	return Inspect(AuthPath(), now)
}
