package ghratelimit

import (
	"context"
)

// semaphore type limits concurrency
type semaphore chan struct{}

func (s semaphore) Close() {
	close(s)
}

// Acquire tries to Acquire a slot in the semaphore with context support.
// If the context expires before a slot is acquired, it returns an error.
func (s semaphore) Acquire(ctx context.Context) error {
	select {
	case s <- struct{}{}:
		// Acquired a slot
		return nil
	case <-ctx.Done():
		// Context expired or was cancelled
		return ctx.Err()
	}
}

// Release a slot in the semaphore
func (s semaphore) Release() {
	<-s
}

// newSemaphore creates a new semaphore with a given capacity
func newSemaphore(length int) semaphore {
	return make(semaphore, length)
}
