package util

import (
	"sync"
	"time"
)

type Debouncer struct {
	mu     sync.Mutex
	timers map[string]*debounceEntry
}

type debounceEntry struct {
	timer    *time.Timer
	onCancel func()
}

func NewDebouncer() *Debouncer {
	return &Debouncer{
		timers: make(map[string]*debounceEntry),
	}
}

func (d *Debouncer) Debounce(key string, fn, onCancel func(), delay time.Duration) {
	d.mu.Lock()
	var canceled func()

	if entry, exists := d.timers[key]; exists && entry.timer.Stop() {
		canceled = entry.onCancel
	}

	entry := &debounceEntry{onCancel: onCancel}
	entry.timer = time.AfterFunc(delay, func() {
		d.mu.Lock()
		if d.timers[key] == entry {
			delete(d.timers, key)
		}
		d.mu.Unlock()
		fn()
	})
	d.timers[key] = entry
	d.mu.Unlock()

	if canceled != nil {
		canceled()
	}
}

func (d *Debouncer) Cancel(key string) {
	d.mu.Lock()
	var canceled func()

	if entry, exists := d.timers[key]; exists {
		if entry.timer.Stop() {
			canceled = entry.onCancel
		}
		delete(d.timers, key)
	}
	d.mu.Unlock()

	if canceled != nil {
		canceled()
	}
}
