package util

import "sync"

type GuardedChan[T any] struct {
	ch     chan T
	closed bool
	mu     sync.Mutex
}

func NewGuardedChan[T any](size int) *GuardedChan[T] {
	return &GuardedChan[T]{ch: make(chan T, size)}
}

func (g *GuardedChan[T]) Send(v T) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false
	}
	select {
	case g.ch <- v:
		return true
	default:
		return false
	}
}

func (g *GuardedChan[T]) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		close(g.ch)
		g.closed = true
	}
}

func (g *GuardedChan[T]) Ch() <-chan T {
	return g.ch
}
