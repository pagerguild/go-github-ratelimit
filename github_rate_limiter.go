package ratelimit

import (
	"context"
	"net/http"
	"time"
)

// githubHeaderRateLimiter manages rate limits based on GitHub rate limit headers.
type githubHeaderRateLimiter struct {
	resetThreshold int64               // Throttle when remaining requests are below this threshold
	responses      chan *http.Response // Channel to process HTTP responses
	lock           chan struct{}       // Channel that blocks when throttling
}

func (r *githubHeaderRateLimiter) Acquire(ctx context.Context) error {
	select {
	// Normal operation (Primary rate limit has NOT been reached):
	// 		r.lock is CLOSED and therefore we fall through immediately
	// 		to return successfully.
	// LOCKED/BLOCKED (Primary rate limit has been reached):
	// 		r.lock is an open channel, which will be closed when the
	// 		rate limit resets, therefore allowing a successful return
	// 		as long as the reset happens before the context expires.
	case <-r.lock:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *githubHeaderRateLimiter) Close() {
	close(r.responses)
}

// newGitHubHeaderRateLimiter creates a new rate limiter based on GitHub headers.
func newGitHubHeaderRateLimiter(threshold int64) *githubHeaderRateLimiter {
	rateLimiter := &githubHeaderRateLimiter{
		resetThreshold: threshold,
		responses:      make(chan *http.Response, threshold),
		lock:           make(chan struct{}),
	}
	close(rateLimiter.lock)

	go rateLimiter.manageThrottle()
	return rateLimiter
}

// manageThrottle manages rate limits based on incoming responses.
func (r *githubHeaderRateLimiter) manageThrottle() {
	for {
		select {
		case resp, stillOpen := <-r.responses:
			if !stillOpen {
				return
			}
			if resp == nil {
				continue
			}

			rateLimitInfo := NewGitHubRateLimitInfo(resp)

			// If remaining requests drop below the threshold, start throttling
			if rateLimitInfo.Remaining <= r.resetThreshold {
				r.lock = make(chan struct{})
				timer := time.NewTimer(rateLimitInfo.TimeToReset())
				defer timer.Stop()

				<-timer.C
				close(r.lock)
			}
		}
	}
}

// AddResponse processes the HTTP response headers for rate limiting.
func (r *githubHeaderRateLimiter) AddResponse(resp *http.Response) {
	r.responses <- resp
}
