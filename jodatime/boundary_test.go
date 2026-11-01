package jodatime

import (
	"testing"
	"time"
)

func TestFormatBoundaries(t *testing.T) {
	noon := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)
	if got := Format("a", noon); got != "PM" {
		t.Fatalf("Format(a, 12:30) = %q, want PM", got)
	}
	ist := time.Date(2024, 1, 1, 0, 0, 0, 0, time.FixedZone("IST", 5*3600+30*60))
	if got := Format("Z", ist); got != "+0530" {
		t.Fatalf("Format(Z) = %q, want +0530", got)
	}
	if got := Format("ZZ", ist); got != "+05:30" {
		t.Fatalf("Format(ZZ) = %q, want +05:30", got)
	}
	if got := Format("ZZZ", ist); got != "IST" {
		t.Fatalf("Format(ZZZ) = %q, want IST", got)
	}
	// 末尾单引号不应越界 panic
	_ = Format("HH'", noon)
	_ = GetLayout("HH'")
}
