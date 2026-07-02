package service

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderSystemdUser(t *testing.T) {
	unit, err := RenderSystemdUser(Spec{
		Name:            "codexpass",
		Program:         "/usr/local/bin/codexpass",
		Host:            "127.0.0.1",
		Port:            9090,
		ConfigPath:      "/home/me/.config/codexpass/config.json",
		Args:            []string{"--metrics", "--stats-path", "/tmp/codexpass stats.jsonl"},
		WorkingDir:      "/home/me",
		Env:             map[string]string{"CODEXPASS_LOG": "debug mode"},
		StandardOutPath: "/tmp/codexpass.out",
		StandardErrPath: "/tmp/codexpass.err",
	})
	if err != nil {
		t.Fatalf("RenderSystemdUser returned error: %v", err)
	}

	want := []string{
		"Description=codexpass local OpenAI-compatible proxy",
		`ExecStart=/usr/local/bin/codexpass serve --host 127.0.0.1 --port 9090 --config /home/me/.config/codexpass/config.json --metrics --stats-path "/tmp/codexpass stats.jsonl"`,
		"\nRestart=on-failure\n",
		"\nWorkingDirectory=/home/me\n",
		"\nEnvironment=\"CODEXPASS_LOG=debug mode\"\n",
		"\nStandardOutput=append:/tmp/codexpass.out\n",
		"\nStandardError=append:/tmp/codexpass.err\n",
		"WantedBy=default.target",
	}
	for _, s := range want {
		if !strings.Contains(unit, s) {
			t.Fatalf("systemd unit missing %q\n%s", s, unit)
		}
	}
}

func TestRenderLaunchdPlist(t *testing.T) {
	plist, err := RenderLaunchdPlist(Spec{
		Name:       "codexpass",
		Label:      "com.example.codexpass",
		Program:    "/opt/homebrew/bin/codexpass",
		Host:       "::1",
		Port:       8081,
		ConfigPath: "/Users/me/Library/Application Support/codexpass/config.json",
		Env:        map[string]string{"CODEXPASS_MODE": "local&safe"},
	})
	if err != nil {
		t.Fatalf("RenderLaunchdPlist returned error: %v", err)
	}

	want := []string{
		"<key>Label</key>",
		"<string>com.example.codexpass</string>",
		"<key>ProgramArguments</key>",
		"<string>/opt/homebrew/bin/codexpass</string>",
		"<string>serve</string>",
		"<string>--host</string>",
		"<string>::1</string>",
		"<string>--port</string>",
		"<string>8081</string>",
		"<string>/Users/me/Library/Application Support/codexpass/config.json</string>",
		"<key>EnvironmentVariables</key>",
		"<string>local&amp;safe</string>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"<key>KeepAlive</key>",
	}
	for _, s := range want {
		if !strings.Contains(plist, s) {
			t.Fatalf("launchd plist missing %q\n%s", s, plist)
		}
	}
}

func TestPlanInstallRefusesNonLoopbackUnlessDryRunOrAllowed(t *testing.T) {
	spec := Spec{Program: "/usr/bin/codexpass", Host: "0.0.0.0"}

	decision, err := PlanInstall(spec, InstallOptions{Platform: "linux"})
	if !errors.Is(err, ErrNonLoopback) {
		t.Fatalf("expected ErrNonLoopback, got decision=%+v err=%v", decision, err)
	}
	if decision.Allowed {
		t.Fatalf("non-dry-run non-loopback install should not be allowed: %+v", decision)
	}

	decision, err = PlanInstall(spec, InstallOptions{Platform: "linux", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run plan returned error: %v", err)
	}
	if !decision.Allowed || !decision.DryRun || decision.Reason == "" {
		t.Fatalf("dry-run plan should be allowed with refusal reason recorded: %+v", decision)
	}

	decision, err = PlanInstall(spec, InstallOptions{Platform: "linux", AllowNonLoopback: true})
	if err != nil {
		t.Fatalf("explicitly allowed non-loopback plan returned error: %v", err)
	}
	if !decision.Allowed || decision.Reason != "" {
		t.Fatalf("allowed non-loopback plan has wrong decision: %+v", decision)
	}
}

func TestPlanInstallAllowsLoopback(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "localhost"} {
		t.Run(host, func(t *testing.T) {
			decision, err := PlanInstall(Spec{Program: "/usr/bin/codexpass", Host: host}, InstallOptions{Platform: "linux"})
			if err != nil {
				t.Fatalf("PlanInstall returned error: %v", err)
			}
			if !decision.Allowed || decision.Path == "" {
				t.Fatalf("loopback install should be allowed: %+v", decision)
			}
		})
	}
}
