package cursor

import (
	"encoding/base64"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	token, err := Encode(12.5, 1_723_000_000_000, 42)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	got, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.Score != 12.5 || got.CreatedAtUnixMs != 1_723_000_000_000 || got.PostID != 42 {
		t.Fatalf("decoded cursor = %+v", got)
	}
}

func TestRejectsMalformedAndInvalidCursors(t *testing.T) {
	unknownField := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"s":1,"t":2,"i":3,"x":4}`))
	badVersion := base64.RawURLEncoding.EncodeToString([]byte(`{"v":2,"s":1,"t":2,"i":3}`))
	nonPositive := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"s":1,"t":2,"i":0}`))
	for _, token := range []string{"", "%%%", unknownField, badVersion, nonPositive} {
		if _, err := Decode(token); err == nil {
			t.Errorf("Decode(%q) error = nil", token)
		}
	}
}
