package poolx

import (
	"fmt"
	"github.com/panjf2000/ants/v2"
	"testing"
	"time"
)

func TestName(t *testing.T) {
	goPool, err := ants.NewPool(100)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 100; i++ {
		test(goPool)
		time.Sleep(3 * time.Second)
	}
}

// test
func test(pool *ants.Pool) {
	fmt.Println(pool.Cap())
	fmt.Println(pool.Running())
	fmt.Println(pool.IsClosed())
}
