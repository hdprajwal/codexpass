package cli

import (
	"fmt"
	"io"

	"github.com/hdprajwal/codex2key/internal/codex"
)

// Token borrows the credential and prints the bare token to out, followed by a
// newline. Suitable for `KEY=$(codex2key token)`.
func Token(out io.Writer) error {
	cred, err := codex.Borrow()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, cred.APIKey)
	return nil
}
