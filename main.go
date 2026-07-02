// Command codexpass borrows the OpenAI credential stored by the Codex CLI
// (~/.codex/auth.json) and hands it to you as shell export lines or a raw token.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hdprajwal/codexpass/internal/cli"
)

// version is the CLI version string, overridable at build time with
// -ldflags "-X main.version=...".
var version = "0.1.0-dev"

const usage = `codexpass - borrow the OpenAI credential from your Codex login

Usage:
  codexpass export [--no-base-url]   Print eval-able shell export lines
  codexpass token                    Print the bare token to stdout
  codexpass doctor [--json] [--live] Inspect local Codex auth state
  codexpass serve [--port N]              Run a local OpenAI-compatible proxy
  codexpass --version                Print version
  codexpass --help                   Print this help

Typical use:
  eval "$(codexpass export)"         Inject the key into the current shell
  KEY=$(codexpass token)             Capture the token for scripts or code
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
	case "token":
		if err := cli.Token(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "codexpass: %v\n", err)
			return 1
		}
		return 0
	case "doctor":
		if err := cli.Doctor(args[1:], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "codexpass: %v\n", err)
			return 1
		}
		return 0
	case "export":
		fs := flag.NewFlagSet("export", flag.ContinueOnError)
		noBaseURL := fs.Bool("no-base-url", false, "omit the OPENAI_BASE_URL line")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := cli.Export(os.Stdout, os.Stderr, cli.ExportOptions{NoBaseURL: *noBaseURL}); err != nil {
			fmt.Fprintf(os.Stderr, "codexpass: %v\n", err)
			return 1
		}
		return 0
	case "serve":
		if err := cli.Serve(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "codexpass: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "codexpass: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
