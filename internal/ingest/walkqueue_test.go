package ingest

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewWalkQueue_Empty(t *testing.T) {
	q := NewWalkQueue()
	if n := q.Len(); n != 0 {
		t.Errorf("Len() = %d, want 0", n)
	}
}

func TestPushPop_Single(t *testing.T) {
	q := NewWalkQueue()
	want := WalkResult{AbsPath: "/a/b.go", RelPath: "b.go"}
	q.Push(want)
	got, ok := q.Pop()
	if !ok {
		t.Fatal("Pop returned ok=false, want true")
	}
	if got != want {
		t.Errorf("Pop = %+v, want %+v", got, want)
	}
	if n := q.Len(); n != 0 {
		t.Errorf("Len() = %d, want 0 after drain", n)
	}
}

func TestPushPop_FIFOOrder(t *testing.T) {
	q := NewWalkQueue()
	items := []WalkResult{
		{RelPath: "a.go"},
		{RelPath: "b.go"},
		{RelPath: "c.go"},
	}
	for _, it := range items {
		q.Push(it)
	}
	for i, want := range items {
		got, ok := q.Pop()
		if !ok {
			t.Fatalf("Pop %d returned ok=false", i)
		}
		if got != want {
			t.Errorf("Pop %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestPop_BlocksWhenEmpty(t *testing.T) {
	q := NewWalkQueue()
	done := make(chan WalkResult, 1)
	go func() {
		item, _ := q.Pop()
		done <- item
	}()
	select {
	case <-done:
		t.Fatal("Pop returned before Push")
	case <-time.After(50 * time.Millisecond):
	}
	want := WalkResult{RelPath: "late.go"}
	q.Push(want)
	select {
	case got := <-done:
		if got != want {
			t.Errorf("Pop = %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop did not unblock after Push")
	}
}

func TestPop_ReturnsFalseAfterClose(t *testing.T) {
	q := NewWalkQueue()
	q.Push(WalkResult{RelPath: "a.go"})
	q.Close()
	item, ok := q.Pop()
	if !ok {
		t.Fatal("first Pop after Close should return ok=true (item still queued)")
	}
	if item.RelPath != "a.go" {
		t.Errorf("Pop = %+v, want a.go", item)
	}
	_, ok = q.Pop()
	if ok {
		t.Fatal("second Pop after drain should return ok=false")
	}
}

func TestClose_WakesBlockedPops(t *testing.T) {
	q := NewWalkQueue()
	const n = 5
	var wg sync.WaitGroup
	wg.Add(n)
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, ok := q.Pop()
			results <- ok
		}()
	}
	time.Sleep(50 * time.Millisecond)
	q.Close()
	wg.Wait()
	close(results)
	for ok := range results {
		if ok {
			t.Error("blocked Pop returned ok=true after Close, want false")
		}
	}
}

func TestClose_Idempotent(t *testing.T) {
	q := NewWalkQueue()
	q.Close()
	q.Close()
}

func TestPushAfterClose_Panics(t *testing.T) {
	q := NewWalkQueue()
	q.Close()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Push after Close did not panic")
		}
	}()
	q.Push(WalkResult{RelPath: "panic.go"})
}

func TestLen(t *testing.T) {
	q := NewWalkQueue()
	if n := q.Len(); n != 0 {
		t.Fatalf("Len() = %d, want 0", n)
	}
	for i := 0; i < 5; i++ {
		q.Push(WalkResult{RelPath: "file.go"})
	}
	if n := q.Len(); n != 5 {
		t.Fatalf("Len() = %d, want 5 after 5 pushes", n)
	}
	q.Pop()
	q.Pop()
	if n := q.Len(); n != 3 {
		t.Fatalf("Len() = %d, want 3 after 2 pops", n)
	}
}

func TestConcurrent_ProducerConsumer(t *testing.T) {
	q := NewWalkQueue()
	const n = 1000
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_, ok := q.Pop()
			if !ok {
				t.Errorf("Pop returned ok=false at iteration %d", i)
				return
			}
		}
	}()
	for i := 0; i < n; i++ {
		q.Push(WalkResult{RelPath: "file.go"})
	}
	q.Close()
	wg.Wait()
}

func TestConcurrent_MultipleConsumers(t *testing.T) {
	q := NewWalkQueue()
	const numItems = 5000
	const numConsumers = 4

	produced := make(map[string]bool, numItems)
	for i := 0; i < numItems; i++ {
		key := fmt.Sprintf("file%d.go", i)
		produced[key] = false
	}

	var popMu sync.Mutex
	seen := make(map[string]int)

	var wg sync.WaitGroup
	wg.Add(numConsumers)
	for c := 0; c < numConsumers; c++ {
		go func() {
			defer wg.Done()
			for {
				item, ok := q.Pop()
				if !ok {
					return
				}
				popMu.Lock()
				seen[item.RelPath]++
				popMu.Unlock()
			}
		}()
	}

	for path := range produced {
		q.Push(WalkResult{RelPath: path})
	}
	q.Close()
	wg.Wait()

	popMu.Lock()
	defer popMu.Unlock()
	for path := range produced {
		if seen[path] != 1 {
			t.Errorf("path %q seen %d times, want 1", path, seen[path])
		}
	}
	if len(seen) != numItems {
		t.Errorf("seen %d unique paths, want %d", len(seen), numItems)
	}
}

func TestStress_Interleaved(t *testing.T) {
	q := NewWalkQueue()
	const n = 2000

	var wg sync.WaitGroup

	producerDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			q.Push(WalkResult{RelPath: "file.go"})
			time.Sleep(time.Microsecond)
		}
		close(producerDone)
	}()

	var consumerCount int
	var consumerMu sync.Mutex
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, ok := q.Pop()
			if !ok {
				break
			}
			consumerMu.Lock()
			consumerCount++
			consumerMu.Unlock()
		}
	}()

	<-producerDone
	q.Close()
	wg.Wait()

	consumerMu.Lock()
	got := consumerCount
	consumerMu.Unlock()
	if got != n {
		t.Fatalf("consumer consumed %d items, want %d", got, n)
	}
}
