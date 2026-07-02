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
	logFormat := fs.String("log-format", "", "request log format: text or json")
	metrics := fs.Bool("metrics", false, "enable /metrics endpoint")
	statsPath := fs.String("stats-path", "", "write redacted usage JSONL to this path")
	retryAttempts := fs.Int("retry-attempts", 1, "maximum upstream attempts for retryable failures")
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
	if !visited["log-format"] && cfg.Server.LogFormat != "" {
		*logFormat = cfg.Server.LogFormat
	}
	if !visited["metrics"] {
		*metrics = cfg.Server.Metrics
	}
	if !visited["stats-path"] && cfg.Server.StatsPath != "" {
		*statsPath = cfg.Server.StatsPath
	}
	if !visited["retry-attempts"] && cfg.Server.RetryAttempts != 0 {
		*retryAttempts = cfg.Server.RetryAttempts
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := proxy.New(proxy.Config{
		Host:          *host,
		Port:          *port,
		Token:         *token,
		Verbose:       *verbose,
		LogFormat:     *logFormat,
		Metrics:       *metrics,
		StatsPath:     *statsPath,
		RetryAttempts: *retryAttempts,
		ModelAliases:  cfg.Models.Aliases,
		ModelCacheTTL: cfg.Models.CacheTTL(),
		Clients:       proxyClients(cfg.Clients),
	})
	return s.ListenAndServe(ctx)
}

func proxyClients(clients config.ClientsConfig) []proxy.ClientPolicy {
	out := make([]proxy.ClientPolicy, 0, len(clients))
	for name, c := range clients {
		out = append(out, proxy.ClientPolicy{
			Name:               name,
			Token:              c.Token,
			AllowedEndpoints:   c.AllowedEndpoints,
			AllowedModels:      c.AllowedModels,
			MaxBodyBytes:       c.MaxBodyBytes,
			RateLimitPerMinute: c.RateLimitPerMinute,
			AllowFallback:      c.AllowFallback,
			Disabled:           c.Disabled,
		})
	}
	return out
}
