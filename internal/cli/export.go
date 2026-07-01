package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/hdprajwal/codex2key/internal/codex"
)

// ExportOptions controls the export command.
type ExportOptions struct {
	// NoBaseURL omits the OPENAI_BASE_URL line even in chatgpt mode.
	NoBaseURL bool
}

// Export borrows the credential and writes eval-able shell export lines to out.
// Human-facing notes go to notes (stderr) so that `eval "$(codex2key export)"`
// never evaluates them.
func Export(out, notes io.Writer, opts ExportOptions) error {
	cred, err := codex.Borrow()
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "export OPENAI_API_KEY=%s\n", shellSingleQuote(cred.APIKey))

	if base := cred.BaseURL(); base != "" && !opts.NoBaseURL {
		fmt.Fprintf(out, "export OPENAI_BASE_URL=%s\n", shellSingleQuote(base))
	}

	if cred.Mode == "chatgpt" {
		acct := cred.AccountID
		if acct == "" {
			acct = "unknown"
		}
		fmt.Fprintf(notes, "note: this Codex token is scoped to the Codex backend and also expects a "+
			"ChatGPT-Account-ID header (%s), which cannot be passed through an environment variable.\n", acct)
	}

	return nil
}

// shellSingleQuote wraps s in single quotes for safe shell eval, escaping any
// embedded single quote the POSIX way. Tokens and API keys never contain
// quotes, but this keeps the output robust regardless of input.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
