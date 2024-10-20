package ghratelimit

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Mock response with rate limit headers
func mockResponse(remaining int64, reset time.Duration) *http.Response {
	// Get the Unix timestamp for the reset time
	resetTime := time.Now().Add(reset).Unix()

	return &http.Response{
		Header: map[string][]string{
			"X-Ratelimit-Remaining": {fmt.Sprintf("%d", remaining)},
			"X-Ratelimit-Reset":     {fmt.Sprintf("%d", resetTime)},
		},
	}
}

// Test adding a response and triggering throttling
func TestGithubHeaderRateLimiter_AddResponseAndThrottle(t *testing.T) {
	limiter := newGitHubHeaderRateLimiter(50)

	// Create a response with rate limit headers below the threshold
	resp := mockResponse(0, 2*time.Second) // Remaining below the threshold

	// Add the response
	limiter.AddResponse(resp)

	time.Sleep(1 * time.Millisecond)

	// Expect throttling to start and block
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := limiter.Acquire(ctx)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected context deadline exceeded error, got: %v", err)
	}
}

// Test that manageThrottle processes responses correctly
func TestGithubHeaderRateLimiter_ManageThrottle(t *testing.T) {
	t.Skip()
	limiter := newGitHubHeaderRateLimiter(50)

	// Create a response with rate limit headers below the threshold
	resp := mockResponse(40, 50*time.Millisecond)

	// Add the response
	limiter.AddResponse(resp)

	// Wait for the throttle to be released after the reset time
	time.Sleep(100 * time.Millisecond)

	// Try to acquire again, it should not block
	ctx := context.Background()
	err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("expected acquire to pass, got error: %v", err)
	}
}

// Test that releasing with a nil response does not cause issues
func TestRateLimiter_ReleaseNilResponse(t *testing.T) {
	limiter := newRateLimiter(http.DefaultTransport, 10)

	// Acquire a slot in the rate limiter
	ctx := context.Background()
	err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("expected to acquire a slot, got error: %v", err)
	}

	// Release with a nil response, should not cause any errors
	limiter.Release(nil)

	// Acquire again to ensure the slot was properly released
	err = limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("expected to acquire a slot after releasing nil response, got error: %v", err)
	}
}
