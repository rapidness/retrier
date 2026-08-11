package retry

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// Backoff calculates the delay before the next retry attempt.
type Backoff struct {
	strategy     string
	initialDelay time.Duration
	multiplier   float64
	maxDelay     time.Duration
	jitter       bool
	rng         *rand.Rand
	rngMu       sync.Mutex
}

// NewBackoff creates a backoff calculator from config.
func NewBackoff(strategy string, initialDelayMs int, multiplier float64, maxDelayMs int, jitter bool) *Backoff {
	b := &Backoff{
		strategy:     strategy,
		initialDelay: time.Duration(initialDelayMs) * time.Millisecond,
		multiplier:   multiplier,
		maxDelay:     time.Duration(maxDelayMs) * time.Millisecond,
		jitter:       jitter,
	}
	if jitter {
		b.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return b
}

// Delay returns the wait duration before attempt number `attempt` (1-based).
// Attempt 1 = first retry (after initial failure).
func (b *Backoff) Delay(attempt int) time.Duration {
	var delay time.Duration

	switch b.strategy {
	case "fixed":
		delay = b.initialDelay

	case "exponential":
		// initialDelay * multiplier^(attempt-1)
		factor := math.Pow(b.multiplier, float64(attempt-1))
		delay = time.Duration(float64(b.initialDelay) * factor)

	case "linear":
		// initialDelay * attempt
		delay = b.initialDelay * time.Duration(attempt)

	default:
		delay = b.initialDelay
	}

	// Cap at max delay
	if delay > b.maxDelay {
		delay = b.maxDelay
	}

	// Add jitter: randomize between [0.5*delay, 1.5*delay]
	if b.jitter && delay > 0 {
		b.rngMu.Lock()
		jitterFactor := 0.5 + b.rng.Float64() // 0.5 to 1.5
		b.rngMu.Unlock()
		delay = time.Duration(float64(delay) * jitterFactor)
		if delay > b.maxDelay {
			delay = b.maxDelay
		}
	}

	return delay
}
