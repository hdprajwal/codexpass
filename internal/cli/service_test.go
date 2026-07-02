package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestServiceInstallDryRun(t *testing.T) {
	var out bytes.Buffer
	err := Service([]string{"install", "--dry-run", "--platform", "linux", "--host", "127.0.0.1", "--port", "8123"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "ExecStart=") || !strings.Contains(got, "--port 8123") {
		t.Fatalf("dry-run output = %s", got)
	}
}

func TestServiceInstallRefusesNetworkBinding(t *testing.T) {
	var out bytes.Buffer
	err := Service([]string{"install", "--dry-run=false", "--platform", "linux", "--host", "0.0.0.0"}, &out)
	if err == nil {
		t.Fatal("expected non-loopback refusal")
	}
}
