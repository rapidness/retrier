package retry

import (
	"testing"
	"time"
)

func TestBackoffFixed(t *testing.T) {
	b := NewBackoff("fixed", 1000, 2.0, 30000, false)

	for attempt := 1; attempt <= 5; attempt++ {
		delay := b.Delay(attempt)
		if delay != 1000*time.Millisecond {
			t.Errorf("Fixed backoff attempt %d: got %v, want 1s", attempt, delay)
		}
	}
}

func TestBackoffExponential(t *testing.T) {
	b := NewBackoff("exponential", 1000, 2.0, 30000, false)

	// Attempt 1: 1000ms
	// Attempt 2: 2000ms
	// Attempt 3: 4000ms
	expected := []time.Duration{
		1000 * time.Millisecond,
		2000 * time.Millisecond,
		4000 * time.Millisecond,
	}

	for i, exp := range expected {
		got := b.Delay(i + 1)
		if got != exp {
			t.Errorf("Exponential backoff attempt %d: got %v, want %v", i+1, got, exp)
		}
	}
}

func TestBackoffLinear(t *testing.T) {
	b := NewBackoff("linear", 1000, 2.0, 30000, false)

	// Attempt 1: 1000ms
	// Attempt 2: 2000ms
	// Attempt 3: 3000ms
	expected := []time.Duration{
		1000 * time.Millisecond,
		2000 * time.Millisecond,
		3000 * time.Millisecond,
	}

	for i, exp := range expected {
		got := b.Delay(i + 1)
		if got != exp {
			t.Errorf("Linear backoff attempt %d: got %v, want %v", i+1, got, exp)
		}
	}
}

func TestBackoffMaxDelay(t *testing.T) {
	b := NewBackoff("exponential", 1000, 2.0, 3000, false)

	// Attempt 3 would be 4000ms but max is 3000ms
	got := b.Delay(3)
	if got != 3000*time.Millisecond {
		t.Errorf("MaxDelay cap: got %v, want 3s", got)
	}
}

func TestBackoffJitter(t *testing.T) {
	b := NewBackoff("fixed", 1000, 2.0, 30000, true)

	// With jitter, delay should be between 500ms and 1500ms
	for i := 0; i < 100; i++ {
		delay := b.Delay(1)
		if delay < 500*time.Millisecond || delay > 1500*time.Millisecond {
			t.Errorf("Jitter out of range: got %v, expected [500ms, 1500ms]", delay)
		}
	}
}

func BenchmarkBackoffExponential(b *testing.B) {
	bo := NewBackoff("exponential", 1000, 2.0, 30000, false)
	for i := 0; i < b.N; i++ {
		bo.Delay(3)
	}
}

func BenchmarkBackoffWithJitter(b *testing.B) {
	bo := NewBackoff("exponential", 1000, 2.0, 30000, true)
	for i := 0; i < b.N; i++ {
		bo.Delay(3)
	}
}
