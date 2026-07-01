package codex

import (
	"encoding/base64"
	"testing"
)

// makeToken builds a fake JWT whose payload is the given JSON. Only the payload
// segment matters to jwtExp; header and signature are arbitrary.
func makeToken(payloadJSON string) string {
	seg := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	return "header." + seg + ".signature"
}

func TestJWTExp(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantExp int64
		wantOK  bool
	}{
		{"integer exp", makeToken(`{"exp":1700000000}`), 1700000000, true},
		{"float exp", makeToken(`{"exp":1700000000.0}`), 1700000000, true},
		{"exp with other claims", makeToken(`{"sub":"abc","exp":42}`), 42, true},
		{"no exp claim", makeToken(`{"sub":"abc"}`), 0, false},
		{"empty payload object", makeToken(`{}`), 0, false},
		{"malformed base64 payload", "header.!!!not-base64!!!.sig", 0, false},
		{"payload not json", makeToken(`not json`), 0, false},
		{"too few segments", "onlyonesegment", 0, false},
		{"empty string", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, ok := jwtExp(tt.token)
			if ok != tt.wantOK {
				t.Fatalf("jwtExp ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && exp != tt.wantExp {
				t.Fatalf("jwtExp exp = %d, want %d", exp, tt.wantExp)
			}
		})
	}
}
