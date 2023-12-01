package merkledb

import (
	"context"
	"sync/atomic"
)

type semaphore struct {
	max          int32
	totalRunning atomic.Int32
}

func newSemaphore(limit int32) *semaphore {
	result := &semaphore{totalRunning: atomic.Int32{}, max: limit}
	return result
}

// acquire will spinlock until the semaphore can be acquired
// WARNING: this should be ok because there should not be much conflict.
// We call this only once per view commit, so it would require concurrent views being committed to start conflicting often
func (l *semaphore) acquire(ctx context.Context) bool {
	var ctxDoneCh <-chan struct{}
	if ctx != nil {
		ctxDoneCh = ctx.Done()
	}

	for {
		select {
		case <-ctxDoneCh:
			return false
		default:
		}
		current := l.totalRunning.Load()
		if current < l.max && l.totalRunning.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// tryAcquire will spinlock until the semaphore can be acquired or there are no more resources to acquire
// WARNING: this should be ok because we spend most of the time at current == 0, where it will fast fail
func (l *semaphore) tryAcquire() bool {
	if l.totalRunning.Load() == l.max {
		return false
	}
	if newValue := l.totalRunning.Add(1); newValue >= l.max {
		l.totalRunning.Add(-1)
		return false
	}
	l.totalRunning.Add(1)
	if l.totalRunning.Load() > l.max {
		panic("more running than should be possible")
	}
	return true
}

func (l *semaphore) release() {
	l.totalRunning.Add(-1)
	if l.totalRunning.Load() < 0 {
		panic("released more than possible")
	}
}
