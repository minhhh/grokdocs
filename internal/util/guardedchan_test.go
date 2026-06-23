package util

import (
	"testing"
	"time"
)

func TestNewGuardedChan(t *testing.T) {
	g := NewGuardedChan[int](5)
	if g == nil {
		t.Fatal("expected non-nil GuardedChan")
	}
}

func TestGuardedChanSendAndReceive(t *testing.T) {
	g := NewGuardedChan[int](5)
	if !g.Send(42) {
		t.Fatal("Send should succeed")
	}
	select {
	case v := <-g.Ch():
		if v != 42 {
			t.Errorf("expected 42, got %d", v)
		}
	default:
		t.Fatal("expected to receive value")
	}
}

func TestGuardedChanSend_DropsWhenFull(t *testing.T) {
	g := NewGuardedChan[int](1)
	g.Send(1)
	if g.Send(2) {
		t.Error("Send should return false when buffer full")
	}
}

func TestGuardedChanSend_Closed(t *testing.T) {
	g := NewGuardedChan[int](5)
	g.Close()
	if g.Send(1) {
		t.Error("Send should return false after Close")
	}
}

func TestGuardedChanClose(t *testing.T) {
	g := NewGuardedChan[int](5)
	g.Send(1)
	g.Close()
	if g.Send(2) {
		t.Error("Send should return false after close")
	}
	<-g.Ch()
	_, ok := <-g.Ch()
	if ok {
		t.Error("channel should be closed")
	}
}

func TestGuardedChanClose_Idempotent(t *testing.T) {
	g := NewGuardedChan[int](5)
	g.Close()
	g.Close()
	_, ok := <-g.Ch()
	_ = ok
}

func TestGuardedChanCh(t *testing.T) {
	g := NewGuardedChan[int](3)
	ch := g.Ch()
	if ch == nil {
		t.Fatal("Ch() should not return nil")
	}
}

func TestGuardedChanSendConcurrent(t *testing.T) {
	g := NewGuardedChan[int](10)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			g.Send(i)
			time.Sleep(time.Microsecond)
		}
		close(done)
	}()
	count := 0
loop:
	for {
		select {
		case _, ok := <-g.Ch():
			if !ok {
				break loop
			}
			count++
		case <-done:
			break loop
		}
	}
	if count == 0 {
		t.Error("expected to receive at least one value")
	}
}
