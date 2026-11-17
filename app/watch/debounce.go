package watch

import (
	"sync"
	"time"
)

// debounceEntry tracks one key's pending callback. gen guards against stale
// timer callbacks: a timer that was already firing when a newer event arrived
// must not delete the newer timer nor invoke the callback early.
type debounceEntry struct {
	timer *time.Timer
	gen   uint64
}

type debouncer struct {
	mu       sync.Mutex
	timers   map[string]*debounceEntry
	delay    time.Duration
	callback func(string)
}

func newDebouncer(delay time.Duration, callback func(string)) *debouncer {
	return &debouncer{
		timers:   make(map[string]*debounceEntry),
		delay:    delay,
		callback: callback,
	}
}

func (d *debouncer) trigger(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.timers[key]
	if !exists {
		entry = &debounceEntry{}
		d.timers[key] = entry
	} else if entry.timer != nil {
		entry.timer.Stop()
	}

	entry.gen++
	gen := entry.gen
	entry.timer = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		current, ok := d.timers[key]
		if !ok || current != entry || entry.gen != gen {
			d.mu.Unlock()
			return
		}
		delete(d.timers, key)
		d.mu.Unlock()

		d.callback(key)
	})
}

func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, entry := range d.timers {
		if entry.timer != nil {
			entry.timer.Stop()
		}
		delete(d.timers, key)
	}
}
