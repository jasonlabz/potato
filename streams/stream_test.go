package streams

import (
	"cmp"
	"testing"
)

func TestMaxMin(t *testing.T) {
	if v, ok := Of(3, 9, 1, 5).Max(cmp.Compare[int]).Get(); !ok || v != 9 {
		t.Fatalf("Max() = %d, %v", v, ok)
	}
	if v, ok := Of(3, 9, 1, 5).Min(cmp.Compare[int]).Get(); !ok || v != 1 {
		t.Fatalf("Min() = %d, %v", v, ok)
	}
	if Of[int]().Max(cmp.Compare[int]).IsPresent() {
		t.Fatal("Max() of empty stream should be absent")
	}
}

func TestCallLimit(t *testing.T) {
	n := Call(func(source chan<- int) {
		for i := 0; i < 100; i++ {
			source <- i
		}
	}).Limit(2).Count()
	if n != 2 {
		t.Fatalf("Limit(2).Count() = %d", n)
	}
}
