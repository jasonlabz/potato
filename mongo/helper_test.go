package mongo

import (
	"strings"
	"testing"
)

func TestMaskURIHidesShortPassword(t *testing.T) {
	for _, uri := range []string{
		"mongodb://u:pass@10.0.0.1:27017",
		"mongodb://user:pass@" + strings.Repeat("h", 60) + ":27017",
	} {
		if got := maskURI(uri); strings.Contains(got, "pass") {
			t.Fatalf("maskURI(%q) = %q, leaks password", uri, got)
		}
	}
	if got := maskURI("redis://x"); got != "redis://x" {
		t.Fatalf("maskURI(short non-mongo) = %q", got)
	}
}
