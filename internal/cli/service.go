package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hdprajwal/codexpass/internal/service"
)

// Service handles background service commands.
func Service(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: codexpass service install|uninstall|status")
	}
	switch args[0] {
	case "install":
		return serviceInstall(args[1:], out)
	case "uninstall":
		return serviceUninstall(args[1:], out)
	case "status":
		return serviceStatus(args[1:], out)
	default:
		return fmt.Errorf("unknown service command %q", args[0])
	}
}

func serviceInstall(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("service install", flag.ContinueOnError)
	host := fs.String("host", "127.0.0.1", "host/interface to bind")
	port := fs.Int("port", 8080, "port to listen on")
	configPath := fs.String("config", "", "path to codexpass config file")
	dryRun := fs.Bool("dry-run", false, "print the generated service without writing it")
	allowNetwork := fs.Bool("allow-network", false, "allow non-loopback service binding")
	platform := fs.String("platform", "", "override platform for rendering/testing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	program, err := os.Executable()
	if err != nil {
		return err
	}
	result, err := service.Install(context.Background(), service.Spec{
		Program:    program,
		Host:       *host,
		Port:       *port,
		ConfigPath: *configPath,
	}, service.InstallOptions{Platform: *platform, DryRun: *dryRun, AllowNonLoopback: *allowNetwork})
	if *dryRun && result.Content != "" {
		fmt.Fprint(out, result.Content)
	}
	if err != nil {
		return err
	}
	if !*dryRun {
		fmt.Fprintf(out, "installed service at %s\n", result.Decision.Path)
	}
	return nil
}

func serviceUninstall(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("service uninstall", flag.ContinueOnError)
	platform := fs.String("platform", "", "override platform for rendering/testing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := service.Uninstall(context.Background(), service.Spec{Program: "codexpass"}, *platform); err != nil {
		return err
	}
	fmt.Fprintln(out, "uninstalled service")
	return nil
}

func serviceStatus(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("service status", flag.ContinueOnError)
	platform := fs.String("platform", "", "override platform for rendering/testing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	status, err := service.CheckStatus(context.Background(), service.Spec{Program: "codexpass"}, *platform)
	if status.Active {
		fmt.Fprintln(out, "active")
	} else {
		fmt.Fprintln(out, "inactive")
	}
	if status.Output != "" {
		fmt.Fprintln(out, status.Output)
	}
	return err
}
