package cli

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/hdprajwal/codexpass/internal/config"
	"github.com/hdprajwal/codexpass/internal/proxy"
)

// Serve parses flags and runs the proxy server until interrupted.
func Serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to codexpass config file")
	host := fs.String("host", "127.0.0.1", "host/interface to bind")
	port := fs.Int("port", 8080, "port to listen on")
	token := fs.String("token", "", "require this bearer token from clients (optional)")
	verbose := fs.Bool("verbose", false, "log request metadata (never secrets or content)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = config.Path()
	}
	cfg, err := config.LoadFrom(cfgPath)
	if err != nil {
		return err
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	if !visited["host"] && cfg.Server.Host != "" {
		*host = cfg.Server.Host
	}
	if !visited["port"] && cfg.Server.Port != 0 {
		*port = cfg.Server.Port
	}
	if !visited["token"] && cfg.Server.Token != "" {
		*token = cfg.Server.Token
	}
	if !visited["verbose"] {
		*verbose = cfg.Server.Verbose
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := proxy.New(proxy.Config{
		Host:          *host,
		Port:          *port,
		Token:         *token,
		Verbose:       *verbose,
		ModelAliases:  cfg.Models.Aliases,
		ModelCacheTTL: cfg.Models.CacheTTL(),
	})
	return s.ListenAndServe(ctx)
}
