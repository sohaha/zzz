package watch

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncerFiresOnceForRapidTriggers(t *testing.T) {
	var calls int32
	d := newDebouncer(50*time.Millisecond, func(string) {
		atomic.AddInt32(&calls, 1)
	})
	t.Cleanup(d.stop)

	for i := 0; i < 5; i++ {
		d.trigger("key")
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(120 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly one callback after rapid triggers, got %d", got)
	}

	// A stale timer callback must not delete the settled state or fire again.
	time.Sleep(100 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected no trailing callbacks, got %d", got)
	}
}

func TestDebouncerFiresSeparateKeysIndependently(t *testing.T) {
	fired := make(chan string, 2)
	d := newDebouncer(10*time.Millisecond, func(key string) {
		fired <- key
	})
	t.Cleanup(d.stop)

	d.trigger("a")
	d.trigger("b")

	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case key := <-fired:
			seen[key] = true
		case <-time.After(time.Second):
			t.Fatalf("expected both keys to fire, got %v", seen)
		}
	}
	if !seen["a"] || !seen["b"] {
		t.Fatalf("expected both keys to fire, got %v", seen)
	}
}

func TestDebouncerStopPreventsCallback(t *testing.T) {
	var calls int32
	d := newDebouncer(30*time.Millisecond, func(string) {
		atomic.AddInt32(&calls, 1)
	})

	d.trigger("key")
	d.stop()

	time.Sleep(100 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected no callback after stop, got %d", got)
	}
}
