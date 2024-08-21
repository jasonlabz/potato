package idgen

import (
	"fmt"
	"testing"
)

func TestGenUID(t *testing.T) {
	for i := 0; i < 1000; i++ {
		fmt.Println(GenUID())
	}
}
