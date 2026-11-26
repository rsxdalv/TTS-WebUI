package history

import (
	"fmt"
	"sync"
	"testing"
)

// TestStoreConcurrentUse hammers CreateCollection/Collections/Items from several
// goroutines. It asserts nothing itself: it exists so `go test -race` covers the
// Store mutex.
func TestStoreConcurrentUse(t *testing.T) {
	s := New(Options{Root: t.TempDir(), Token: "t"})
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				_ = s.CreateCollection(fmt.Sprintf("c%d", (g+i)%6))
				_, _ = s.Collections()
				_, _ = s.Items("outputs")
				_, _ = s.Items("collections/c1")
			}
		}(g)
	}
	wg.Wait()
}
