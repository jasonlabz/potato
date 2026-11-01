package bloom

import (
	"strconv"
	"sync"
	"testing"
)

func TestBloomFilterConcurrentAddContains(t *testing.T) {
	bf := NewBloomFilter()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				v := strconv.Itoa(i*1000 + j)
				bf.Add(v)
				if !bf.Contains(v) {
					t.Errorf("Contains(%q) = false after Add", v)
				}
			}
		}(i)
	}
	wg.Wait()
}
