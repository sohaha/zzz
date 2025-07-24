package watch

import (
	"sync"
	"time"
)

type debouncer struct {
	mu       sync.RWMutex
	timers   map[string]*time.Timer
	delay    time.Duration
	callback func(string)
}

func newDebouncer(delay time.Duration, callback func(string)) *debouncer {
	return &debouncer{
		timers:   make(map[string]*time.Timer),
		delay:    delay,
		callback: callback,
	}
}

func (d *debouncer) trigger(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
	}
	
	d.timers[key] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
		d.callback(key)
	})
}

func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	for key, timer := range d.timers {
		timer.Stop()
		delete(d.timers, key)
	}
}

func (d *debouncer) cancel(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
	}
}

func (d *debouncer) pending(key string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	_, exists := d.timers[key]
	return exists
}

func (d *debouncer) count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	return len(d.timers)
}

func (d *debouncer) setDelay(delay time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	d.delay = delay
}

func (d *debouncer) getDelay() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	return d.delay
}