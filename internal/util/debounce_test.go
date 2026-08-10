package util

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebounceNotifiesSupersededCall(t *testing.T) {
	d := NewDebouncer()
	var firstRan atomic.Bool
	var firstCanceled atomic.Bool

	d.Debounce("completion", func() {
		firstRan.Store(true)
	}, func() {
		firstCanceled.Store(true)
	}, time.Hour)

	d.Debounce("completion", func() {}, func() {}, time.Hour)
	d.Cancel("completion")

	if firstRan.Load() {
		t.Fatal("superseded callback ran")
	}
	if !firstCanceled.Load() {
		t.Fatal("superseded callback was not notified")
	}
}

func TestCancelNotifiesPendingCall(t *testing.T) {
	d := NewDebouncer()
	var canceled atomic.Bool

	d.Debounce("completion", func() {}, func() {
		canceled.Store(true)
	}, time.Hour)
	d.Cancel("completion")

	if !canceled.Load() {
		t.Fatal("canceled callback was not notified")
	}
}
