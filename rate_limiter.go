package ghratelimit

import (
	"context"
	"io"
	"net/http"
)

// Each installer of a GitHub Application is rate limited with a Primary rate
// limit of 5000 GitHub API requests/hour.
//
// "Secondary" rate limit per installation: 100 concurrent requests.
//
// This does not affect any other applications that the customer may have
// installed, which will have their own separate resource allowances.
//
// Nor does it affect other installers of the same application.

// RateLimitTransport contains two rate limiting objects, to solve both the Primary
// and Secondary rate limit questions.
type RateLimitTransport[T http.RoundTripper] struct {
	semaphore     semaphore
	headerLimiter *githubHeaderRateLimiter
	transport     T
}

// RoundTrip implements http.RoundTripper.
func (r *RateLimitTransport[T]) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if err = r.Acquire(req.Context()); err != nil {
		return
	}
	defer r.Release(resp)
	resp, err = r.transport.RoundTrip(req)
	return
}

var (
	_ http.RoundTripper = (*RateLimitTransport[http.RoundTripper])(nil)
	_ io.Closer         = (*RateLimitTransport[http.RoundTripper])(nil)
)

// newRateLimiter creates a new rate limiter with both concurrency control and
// GitHub header-based rate limiting.
func newRateLimiter[T http.RoundTripper](rt T, maxConcurrent int64) *RateLimitTransport[T] {
	return &RateLimitTransport[T]{
		semaphore:     newSemaphore(int(maxConcurrent)),
		headerLimiter: newGitHubHeaderRateLimiter(maxConcurrent),
		transport:     rt,
	}
}

func NewRateLimitTransport[T http.RoundTripper](rt T, maxConcurrent int64) *RateLimitTransport[T] {
	return newRateLimiter(rt, maxConcurrent)
}

// Acquire blocks based on both throttling and concurrency limits.
func (r *RateLimitTransport[T]) Acquire(ctx context.Context) error {
	if err := r.headerLimiter.Acquire(ctx); err != nil {
		return err
	}

	return r.semaphore.Acquire(ctx)
}

// Release a slot and process the GitHub API response for potential throttling.
func (r *RateLimitTransport[T]) Release(resp *http.Response) {
	r.semaphore.Release()
	r.headerLimiter.AddResponse(resp)
}

// Close releases all resources.
func (r *RateLimitTransport[T]) Close() error {
	r.semaphore.Close()
	r.headerLimiter.Close()
	return nil
}
