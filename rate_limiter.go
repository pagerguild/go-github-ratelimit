package ratelimit

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

// rateLimiter contains two rate limiting objects, to solve both the Primary
// and Secondary rate limit questions.
type rateLimiter struct {
	semaphore     semaphore
	headerLimiter *githubHeaderRateLimiter
	transport     http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (r *rateLimiter) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if err = r.Acquire(req.Context()); err != nil {
		return
	}
	defer r.Release(resp)
	resp, err = r.transport.RoundTrip(req)
	return
}

type RateLimitTransport interface {
	http.RoundTripper
	io.Closer
}

var _ RateLimitTransport = (*rateLimiter)(nil)

// newRateLimiter creates a new rate limiter with both concurrency control and
// GitHub header-based rate limiting.
func newRateLimiter(rt http.RoundTripper, maxConcurrent int64) *rateLimiter {
	return &rateLimiter{
		semaphore:     newSemaphore(int(maxConcurrent)),
		headerLimiter: newGitHubHeaderRateLimiter(maxConcurrent),
		transport:     rt,
	}
}

func NewRateLimitTransport(rt http.RoundTripper, maxConcurrent int64) RateLimitTransport {
	return newRateLimiter(rt, maxConcurrent)
}

// Acquire blocks based on both throttling and concurrency limits.
func (r *rateLimiter) Acquire(ctx context.Context) error {
	if err := r.headerLimiter.Acquire(ctx); err != nil {
		return err
	}

	return r.semaphore.Acquire(ctx)
}

// Release a slot and process the GitHub API response for potential throttling.
func (r *rateLimiter) Release(resp *http.Response) {
	r.semaphore.Release()
	r.headerLimiter.AddResponse(resp)
}

// Close releases all resources.
func (r *rateLimiter) Close() error {
	r.semaphore.Close()
	r.headerLimiter.Close()
	return nil
}
