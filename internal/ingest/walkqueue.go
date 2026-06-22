package ingest

import "sync"

type WalkQueue struct {
	mu     sync.Mutex
	cond   sync.Cond
	items  []WalkResult
	closed bool
}

func NewWalkQueue() *WalkQueue {
	q := &WalkQueue{}
	q.cond.L = &q.mu
	return q
}

func (q *WalkQueue) Push(item WalkResult) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		panic("walkqueue: push after close")
	}
	q.items = append(q.items, item)
	q.cond.Signal()
}

func (q *WalkQueue) Pop() (WalkResult, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return WalkResult{}, false
	}
	item := q.items[0]
	q.items[0] = WalkResult{}
	q.items = q.items[1:]
	return item, true
}

func (q *WalkQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.cond.Broadcast()
}

func (q *WalkQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
