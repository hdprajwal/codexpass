package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hdprajwal/codexpass/internal/codex"
)

// DoctorOptions controls the doctor command.
type DoctorOptions struct {
	JSON bool
	Live bool
}

type doctorReport struct {
	Auth codex.AuthReport `json:"auth"`
	Live *liveReport      `json:"live,omitempty"`
	OK   bool             `json:"ok"`
}

type liveReport struct {
	Checked bool   `json:"checked"`
	OK      bool   `json:"ok"`
	Status  string `json:"status,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Doctor parses args and writes a redacted diagnostics report.
func Doctor(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit diagnostics as JSON")
	live := fs.Bool("live", false, "probe the upstream models endpoint")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return RunDoctor(context.Background(), out, DoctorOptions{JSON: *jsonOut, Live: *live})
}

// RunDoctor runs the doctor command.
func RunDoctor(ctx context.Context, out io.Writer, opts DoctorOptions) error {
	auth, authErr := codex.InspectDefault(time.Now())
	report := doctorReport{Auth: auth, OK: authErr == nil}
	if opts.Live {
		live := probeLiveModels(ctx)
		report.Live = &live
		report.OK = report.OK && live.OK
	}

	if opts.JSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		writeDoctorText(out, report)
	}
	if !report.OK {
		if authErr != nil {
			return authErr
		}
		return fmt.Errorf("live probe failed")
	}
	return nil
}

func writeDoctorText(w io.Writer, r doctorReport) {
	status := "ok"
	if !r.OK {
		status = "problem"
	}
	fmt.Fprintf(w, "codexpass doctor: %s\n", status)
	fmt.Fprintf(w, "auth path: %s\n", r.Auth.Path)
	fmt.Fprintf(w, "auth file: %t\n", r.Auth.Exists)
	if r.Auth.FileMode != "" {
		fmt.Fprintf(w, "file mode: %s\n", r.Auth.FileMode)
	}
	if r.Auth.Mode != "" {
		fmt.Fprintf(w, "auth mode: %s\n", r.Auth.Mode)
	}
	if r.Auth.BaseURL != "" {
		fmt.Fprintf(w, "base url: %s\n", r.Auth.BaseURL)
	}
	if r.Auth.AccessTokenExpiry != nil {
		fmt.Fprintf(w, "access token expiry: %s\n", r.Auth.AccessTokenExpiry.Format(time.RFC3339))
	} else if r.Auth.AccessTokenUnknown {
		fmt.Fprintln(w, "access token expiry: unknown")
	}
	fmt.Fprintf(w, "has access token: %t\n", r.Auth.HasAccessToken)
	fmt.Fprintf(w, "has refresh token: %t\n", r.Auth.HasRefreshToken)
	fmt.Fprintf(w, "has account id: %t\n", r.Auth.HasAccountID)
	fmt.Fprintf(w, "has api key: %t\n", r.Auth.HasAPIKey)
	if r.Live != nil {
		fmt.Fprintf(w, "live probe: %t", r.Live.OK)
		if r.Live.Status != "" {
			fmt.Fprintf(w, " (%s)", r.Live.Status)
		}
		if r.Live.Error != "" {
			fmt.Fprintf(w, " - %s", r.Live.Error)
		}
		fmt.Fprintln(w)
	}
	for _, p := range r.Auth.Problems {
		fmt.Fprintf(w, "problem: %s\n", p)
	}
}

func probeLiveModels(ctx context.Context) liveReport {
	cred, err := codex.Borrow()
	if err != nil {
		return liveReport{Checked: true, OK: false, Error: err.Error()}
	}
	base := cred.BaseURL()
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	url := base + "/models"
	if cred.BaseURL() != "" {
		url += "?client_version=" + codex.CodexClientVersion
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return liveReport{Checked: true, OK: false, Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+cred.APIKey)
	if cred.BaseURL() != "" {
		req.Header.Set("Originator", codex.CodexOriginator)
		req.Header.Set("User-Agent", codex.CodexUserAgent)
	}
	if cred.AccountID != "" {
		req.Header.Set("ChatGPT-Account-ID", cred.AccountID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return liveReport{Checked: true, OK: false, Error: err.Error()}
	}
	defer resp.Body.Close()
	return liveReport{Checked: true, OK: resp.StatusCode >= 200 && resp.StatusCode < 300, Status: resp.Status}
}
