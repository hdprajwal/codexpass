package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCreateClientTokenPrintsConfigSnippet(t *testing.T) {
	var out bytes.Buffer
	if err := CreateClientToken("zed", &out); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Clients map[string]struct {
			Token string `json:"token"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Clients["zed"].Token == "" {
		t.Fatalf("payload = %s", out.String())
	}
}
