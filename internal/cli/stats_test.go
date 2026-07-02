package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatsSummarizesUsageLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	data := `{"client":"zed","model":"gpt-5.4","input_tokens":3,"output_tokens":4}
{"client":"zed","model":"gpt-5.4","input_tokens":5,"output_tokens":6}
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Stats([]string{"--path", path}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "zed/gpt-5.4 requests=2 input_tokens=8 output_tokens=10 total_tokens=18") {
		t.Fatalf("stats = %s", got)
	}
}
