// Package service renders and manages per-user background service definitions.
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

const (
	defaultName        = "codexpass"
	defaultDescription = "codexpass local OpenAI-compatible proxy"
	defaultHost        = "127.0.0.1"
	defaultPort        = 8080
)

var (
	// ErrNonLoopback is returned when an install would persist a service that
	// listens on a non-loopback interface without explicit approval.
	ErrNonLoopback = errors.New("refusing to install service bound to non-loopback host")
)

// Spec describes a codexpass user service.
type Spec struct {
	// Name is used for filenames and, unless Label is set, service identifiers.
	Name string
	// Label is the launchd label. Systemd units use Name.
	Label       string
	Description string

	// Program is the absolute or PATH-resolved executable to run.
	Program string
	// Args are appended after the generated "serve" arguments.
	Args []string

	Host       string
	Port       int
	ConfigPath string

	WorkingDir string
	Env        map[string]string

	StandardOutPath string
	StandardErrPath string
}

// InstallOptions controls installation behavior.
type InstallOptions struct {
	Platform         string
	DryRun           bool
	AllowNonLoopback bool
}

// Decision describes whether an install operation should proceed.
type Decision struct {
	Allowed bool
	DryRun  bool
	Reason  string
	Path    string
}

// InstallResult contains rendered service data and the intended target path.
type InstallResult struct {
	Decision Decision
	Content  string
}

// Status reports a platform service manager's raw status output.
type Status struct {
	Active bool
	Output string
}

// Normalize returns a copy of s with defaults filled in.
func (s Spec) Normalize() Spec {
	if s.Name == "" {
		s.Name = defaultName
	}
	if s.Label == "" {
		s.Label = "com.hdprajwal." + s.Name
	}
	if s.Description == "" {
		s.Description = defaultDescription
	}
	if s.Host == "" {
		s.Host = defaultHost
	}
	if s.Port == 0 {
		s.Port = defaultPort
	}
	return s
}

// Command returns the argv used by the service definition.
func (s Spec) Command() []string {
	s = s.Normalize()
	args := []string{s.Program, "serve", "--host", s.Host, "--port", strconv.Itoa(s.Port)}
	if s.ConfigPath != "" {
		args = append(args, "--config", s.ConfigPath)
	}
	args = append(args, s.Args...)
	return args
}

// RenderSystemdUser renders a Linux systemd user unit.
func RenderSystemdUser(s Spec) (string, error) {
	s = s.Normalize()
	if err := validateSpec(s); err != nil {
		return "", err
	}
	var b bytes.Buffer
	data := struct {
		Spec      Spec
		ExecStart string
		Env       []string
	}{
		Spec:      s,
		ExecStart: systemdExecStart(s.Command()),
		Env:       systemdEnv(s.Env),
	}
	if err := systemdTemplate.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// RenderLaunchdPlist renders a macOS launchd plist.
func RenderLaunchdPlist(s Spec) (string, error) {
	s = s.Normalize()
	if err := validateSpec(s); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(xmlHeader)
	writePlistString(&b, "Label", s.Label)
	writePlistArray(&b, "ProgramArguments", s.Command())
	if s.WorkingDir != "" {
		writePlistString(&b, "WorkingDirectory", s.WorkingDir)
	}
	if len(s.Env) > 0 {
		writePlistEnv(&b, s.Env)
	}
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>KeepAlive</key>\n\t<true/>\n")
	if s.StandardOutPath != "" {
		writePlistString(&b, "StandardOutPath", s.StandardOutPath)
	}
	if s.StandardErrPath != "" {
		writePlistString(&b, "StandardErrorPath", s.StandardErrPath)
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String(), nil
}

// PlanInstall validates install policy and computes the platform service path.
func PlanInstall(s Spec, opts InstallOptions) (Decision, error) {
	s = s.Normalize()
	if err := validateSpec(s); err != nil {
		return Decision{}, err
	}
	platform := opts.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	path, err := ServicePath(s, platform)
	if err != nil {
		return Decision{}, err
	}
	d := Decision{Allowed: true, DryRun: opts.DryRun, Path: path}
	if !opts.AllowNonLoopback && !IsLoopbackHost(s.Host) {
		d.Allowed = opts.DryRun
		d.Reason = ErrNonLoopback.Error()
		if !opts.DryRun {
			return d, fmt.Errorf("%w: %s", ErrNonLoopback, s.Host)
		}
	}
	return d, nil
}

// ServicePath returns the per-user service definition path for platform.
func ServicePath(s Spec, platform string) (string, error) {
	s = s.Normalize()
	switch platform {
	case "linux":
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "systemd", "user", s.Name+".service"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "LaunchAgents", s.Label+".plist"), nil
	default:
		return "", fmt.Errorf("unsupported service platform %q", platform)
	}
}

// Install writes and starts the user service. DryRun only renders and plans.
func Install(ctx context.Context, s Spec, opts InstallOptions) (InstallResult, error) {
	decision, err := PlanInstall(s, opts)
	if err != nil {
		return InstallResult{Decision: decision}, err
	}
	content, err := renderForPlatform(s, platformOrDefault(opts.Platform))
	if err != nil {
		return InstallResult{Decision: decision}, err
	}
	result := InstallResult{Decision: decision, Content: content}
	if opts.DryRun {
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(decision.Path), 0o755); err != nil {
		return result, err
	}
	if err := os.WriteFile(decision.Path, []byte(content), 0o644); err != nil {
		return result, err
	}
	return result, enableAndStart(ctx, s.Normalize(), platformOrDefault(opts.Platform), decision.Path)
}

// Uninstall stops and removes the user service.
func Uninstall(ctx context.Context, s Spec, platform string) error {
	s = s.Normalize()
	path, err := ServicePath(s, platformOrDefault(platform))
	if err != nil {
		return err
	}
	if err := disableAndStop(ctx, s, platformOrDefault(platform), path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// CheckStatus asks the platform service manager for status.
func CheckStatus(ctx context.Context, s Spec, platform string) (Status, error) {
	s = s.Normalize()
	switch platformOrDefault(platform) {
	case "linux":
		out, err := exec.CommandContext(ctx, "systemctl", "--user", "is-active", s.Name+".service").CombinedOutput()
		text := strings.TrimSpace(string(out))
		return Status{Active: strings.TrimSpace(text) == "active", Output: text}, err
	case "darwin":
		u, _ := user.Current()
		domain := fmt.Sprintf("gui/%d", os.Getuid())
		if u != nil && u.Uid != "" {
			domain = "gui/" + u.Uid
		}
		out, err := exec.CommandContext(ctx, "launchctl", "print", domain+"/"+s.Label).CombinedOutput()
		text := strings.TrimSpace(string(out))
		return Status{Active: err == nil, Output: text}, err
	default:
		return Status{}, fmt.Errorf("unsupported service platform %q", platformOrDefault(platform))
	}
}

// IsLoopbackHost reports whether host resolves to a loopback-only listener.
func IsLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err == nil {
		return ip.IsLoopback()
	}
	addrs, err := net.LookupIP(host)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, addr := range addrs {
		if !addr.IsLoopback() {
			return false
		}
	}
	return true
}

func validateSpec(s Spec) error {
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("service name is required")
	}
	if strings.ContainsAny(s.Name, `/\:`) {
		return fmt.Errorf("invalid service name %q", s.Name)
	}
	if strings.TrimSpace(s.Program) == "" {
		return errors.New("program is required")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("invalid port %d", s.Port)
	}
	return nil
}

func renderForPlatform(s Spec, platform string) (string, error) {
	switch platform {
	case "linux":
		return RenderSystemdUser(s)
	case "darwin":
		return RenderLaunchdPlist(s)
	default:
		return "", fmt.Errorf("unsupported service platform %q", platform)
	}
}

func platformOrDefault(platform string) string {
	if platform == "" {
		return runtime.GOOS
	}
	return platform
}

func enableAndStart(ctx context.Context, s Spec, platform, path string) error {
	switch platform {
	case "linux":
		if out, err := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl daemon-reload: %w: %s", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.CommandContext(ctx, "systemctl", "--user", "enable", "--now", filepath.Base(path)).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl enable --now: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	case "darwin":
		domain := fmt.Sprintf("gui/%d", os.Getuid())
		_ = exec.CommandContext(ctx, "launchctl", "bootout", domain, path).Run()
		if out, err := exec.CommandContext(ctx, "launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.CommandContext(ctx, "launchctl", "enable", domain+"/"+s.Label).CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl enable: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	default:
		return fmt.Errorf("unsupported service platform %q", platform)
	}
}

func disableAndStop(ctx context.Context, s Spec, platform, path string) error {
	switch platform {
	case "linux":
		unit := filepath.Base(path)
		if out, err := exec.CommandContext(ctx, "systemctl", "--user", "disable", "--now", unit).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl disable --now: %w: %s", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl daemon-reload: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	case "darwin":
		domain := fmt.Sprintf("gui/%d", os.Getuid())
		if out, err := exec.CommandContext(ctx, "launchctl", "bootout", domain, path).CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl bootout: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	default:
		return fmt.Errorf("unsupported service platform %q", platform)
	}
}

var systemdTemplate = template.Must(template.New("systemd").Parse(`[Unit]
Description={{.Spec.Description}}
After=network-online.target

[Service]
Type=simple
ExecStart={{.ExecStart}}
Restart=on-failure
RestartSec=5s
{{if .Spec.WorkingDir}}
WorkingDirectory={{.Spec.WorkingDir}}
{{end}}
{{range .Env}}
Environment={{.}}
{{end}}
{{if .Spec.StandardOutPath}}
StandardOutput=append:{{.Spec.StandardOutPath}}
{{end}}
{{if .Spec.StandardErrPath}}
StandardError=append:{{.Spec.StandardErrPath}}
{{end}}

[Install]
WantedBy=default.target
`))

func systemdExecStart(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, systemdQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func systemdEnv(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, systemdQuote(k+"="+env[k]))
	}
	return out
}

func systemdQuote(s string) string {
	if s == "" {
		return `""`
	}
	needsQuote := strings.ContainsAny(s, " \t\n\"'\\;$")
	if !needsQuote {
		return s
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`

func writePlistString(b *strings.Builder, key, value string) {
	b.WriteString("\t<key>")
	b.WriteString(html.EscapeString(key))
	b.WriteString("</key>\n\t<string>")
	b.WriteString(html.EscapeString(value))
	b.WriteString("</string>\n")
}

func writePlistArray(b *strings.Builder, key string, values []string) {
	b.WriteString("\t<key>")
	b.WriteString(html.EscapeString(key))
	b.WriteString("</key>\n\t<array>\n")
	for _, value := range values {
		b.WriteString("\t\t<string>")
		b.WriteString(html.EscapeString(value))
		b.WriteString("</string>\n")
	}
	b.WriteString("\t</array>\n")
}

func writePlistEnv(b *strings.Builder, env map[string]string) {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
	for _, k := range keys {
		b.WriteString("\t\t<key>")
		b.WriteString(html.EscapeString(k))
		b.WriteString("</key>\n\t\t<string>")
		b.WriteString(html.EscapeString(env[k]))
		b.WriteString("</string>\n")
	}
	b.WriteString("\t</dict>\n")
}
