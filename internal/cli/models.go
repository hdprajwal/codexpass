package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/hdprajwal/codexpass/internal/codex"
	"github.com/hdprajwal/codexpass/internal/config"
	"github.com/hdprajwal/codexpass/internal/proxy"
)

// Models handles model registry commands.
func Models(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: codexpass models list|refresh [--config PATH]")
	}
	switch args[0] {
	case "list", "refresh":
		return listModels(args[1:], out)
	default:
		return fmt.Errorf("unknown models command %q", args[0])
	}
}

func listModels(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to codexpass config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := *configPath
	if path == "" {
		path = config.Path()
	}
	cfg, err := config.LoadFrom(path)
	if err != nil {
		return err
	}
	cred, err := codex.Borrow()
	if err != nil {
		return err
	}
	models, err := proxy.FetchModels(context.Background(), cred)
	if err != nil {
		return err
	}
	models = proxy.NewModelRegistry(cfg.Models.Aliases).Expose(models)
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	for _, m := range models {
		fmt.Fprintln(out, m.ID)
	}
	return nil
}
