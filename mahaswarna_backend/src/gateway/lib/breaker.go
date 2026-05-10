package lib

import (
	"time"

	"github.com/mahaswarna/shared"
	"github.com/sony/gobreaker"
)

// NewBreaker creates a gobreaker.CircuitBreaker tuned for internal service calls.
//
// Settings:
//   - Opens after 5 consecutive failures in a 10-second window
//   - Half-opens after 30 seconds (one probe request allowed)
//   - Resets on 2 consecutive successes in half-open state
//   - All timeouts / 5xx from upstreams count as failures
func NewBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 2,               // allow 2 test requests in half-open state
		Interval:    10 * time.Second, // rolling window for failure counting
		Timeout:     30 * time.Second, // time in open state before transitioning to half-open

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open the breaker if 5 or more failures occurred in the current interval.
			return counts.ConsecutiveFailures >= 5
		},

		OnStateChange: func(name string, from, to gobreaker.State) {
			shared.Logger.Warn("circuit breaker state change",
				"service", name,
				"from", from.String(),
				"to", to.String(),
			)
		},
	})
}
