package cli

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/hdprajwal/codex2key/internal/proxy"
)

// Serve parses flags and runs the proxy server until interrupted.
func Serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	host := fs.String("host", "127.0.0.1", "host/interface to bind")
	port := fs.Int("port", 8080, "port to listen on")
	token := fs.String("token", "", "require this bearer token from clients (optional)")
	verbose := fs.Bool("verbose", false, "log request metadata (never secrets or content)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := proxy.New(proxy.Config{Host: *host, Port: *port, Token: *token, Verbose: *verbose})
	return s.ListenAndServe(ctx)
}
