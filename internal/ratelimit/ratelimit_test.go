package ratelimit_test

import (
	"testing"
	"time"

	"github.com/yourusername/hctf2/internal/ratelimit"
)

func TestLimiter_AllowsUnderLimit(t *testing.T) {
	l := ratelimit.New(5, time.Minute)
	for i := 0; i < 5; i++ {
		if !l.Allow("user-1") {
			t.Fatalf("expected Allow() = true on attempt %d", i+1)
		}
	}
}

func TestLimiter_BlocksOverLimit(t *testing.T) {
	l := ratelimit.New(2, time.Minute)
	l.Allow("user-1")
	l.Allow("user-1")
	if l.Allow("user-1") {
		t.Fatal("expected Allow() = false after exceeding limit")
	}
}

func TestLimiter_IsolatesUsers(t *testing.T) {
	l := ratelimit.New(1, time.Minute)
	l.Allow("user-1")
	if !l.Allow("user-2") {
		t.Fatal("user-2 should not be affected by user-1's limit")
	}
}
