package configx

import (
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	for in, want := range map[string]time.Duration{"3000": 3 * time.Second, "5s": 5 * time.Second} {
		if got, err := parseTimeout(in); err != nil || got != want {
			t.Fatalf("parseTimeout(%q) = %v, %v, want %v", in, got, err, want)
		}
	}
}
