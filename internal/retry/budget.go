package retry

import (
	"sync"
	"time"
)

// Budget tracks the global retry rate limit using a sliding window counter.
type Budget struct {
	mu          sync.Mutex
	maxBurst    int           // max retries in window
	window      time.Duration // window duration
	timestamps  []time.Time   // recorded retry timestamps
}

// NewBudget creates a retry budget limiter.
func NewBudget(maxBurst int, windowSec int) *Budget {
	return &Budget{
		maxBurst:   maxBurst,
		window:     time.Duration(windowSec) * time.Second,
		timestamps: make([]time.Time, 0, maxBurst),
	}
}

// Allow checks if a retry is allowed within the budget.
// Returns true if the retry can proceed, false if budget exceeded.
func (b *Budget) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-b.window)

	// Remove expired entries
	valid := b.timestamps[:0]
	for _, ts := range b.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	b.timestamps = valid

	// Check budget
	if len(b.timestamps) >= b.maxBurst {
		return false
	}

	// Record this retry
	b.timestamps = append(b.timestamps, now)
	return true
}

// Update adjusts the budget parameters (for hot-reload).
func (b *Budget) Update(maxBurst int, windowSec int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxBurst = maxBurst
	b.window = time.Duration(windowSec) * time.Second
}
