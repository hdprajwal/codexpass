// Command codex2key borrows the OpenAI credential stored by the Codex CLI
// (~/.codex/auth.json) and hands it to you as shell export lines or a raw token.
package main

import (
	"fmt"
	"os"
)

// version is the CLI version string, overridable at build time with
// -ldflags "-X main.version=...".
var version = "0.1.0-dev"

const usage = `codex2key - borrow the OpenAI credential from your Codex login

Usage:
  codex2key export [--no-base-url]   Print eval-able shell export lines
  codex2key token                    Print the bare token to stdout
  codex2key --version                Print version
  codex2key --help                   Print this help

Typical use:
  eval "$(codex2key export)"         Inject the key into the current shell
  KEY=$(codex2key token)             Capture the token for scripts or code
`

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches a subcommand and returns the process exit code.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
		return 0
	case "--version", "version":
		fmt.Fprintln(os.Stdout, version)
		return 0
	case "export", "token":
		fmt.Fprintf(os.Stderr, "codex2key: %q is not implemented yet\n", args[0])
		return 1
	default:
		fmt.Fprintf(os.Stderr, "codex2key: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
