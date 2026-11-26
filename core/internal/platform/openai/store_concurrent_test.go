package openai

import (
	"fmt"
	"sync"
	"testing"
)

// TestStoreConcurrentUse hammers Put/List/get/Delete from several goroutines.
// It asserts nothing itself: it exists so `go test -race` covers the Store mutex.
func TestStoreConcurrentUse(t *testing.T) {
	s := NewStore(t.TempDir())
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				name := fmt.Sprintf("ep%d", (g+i)%5)
				k := "k"
				_, _ = s.Put(Endpoint{Name: name, URL: "http://127.0.0.1:1/v1"}, &k)
				_, _ = s.List()
				_, _, _ = s.get(name)
				if i%7 == 0 {
					_ = s.Delete(name)
				}
			}
		}(g)
	}
	wg.Wait()
}
