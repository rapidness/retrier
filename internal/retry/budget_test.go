package retry

import (
	"testing"
	"time"
)

func TestBudgetAllow(t *testing.T) {
	b := NewBudget(3, 60) // 3 retries per 60s

	// First 3 should be allowed
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Errorf("Attempt %d should be allowed", i+1)
		}
	}

	// 4th should be denied
	if b.Allow() {
		t.Error("4th attempt should be denied")
	}
}

func TestBudgetWindowExpiry(t *testing.T) {
	b := NewBudget(1, 1) // 1 retry per 1s window

	if !b.Allow() {
		t.Error("First attempt should be allowed")
	}

	if b.Allow() {
		t.Error("Second attempt should be denied (same window)")
	}

	// Wait for window to expire
	time.Sleep(1100 * time.Millisecond)

	if !b.Allow() {
		t.Error("Attempt after window expiry should be allowed")
	}
}

func BenchmarkBudgetAllow(b *testing.B) {
	budget := NewBudget(1000, 60)
	for i := 0; i < b.N; i++ {
		budget.Allow()
	}
}
