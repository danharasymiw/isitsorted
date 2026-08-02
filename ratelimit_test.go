package main

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		rl := newRateLimiter(3, time.Minute)
		for i := 0; i < 3; i++ {
			if !rl.allow("1.2.3.4") {
				t.Fatalf("request %d should be allowed", i+1)
			}
		}
	})

	t.Run("blocks request over limit", func(t *testing.T) {
		rl := newRateLimiter(3, time.Minute)
		for i := 0; i < 3; i++ {
			rl.allow("1.2.3.4")
		}
		if rl.allow("1.2.3.4") {
			t.Fatal("4th request should be blocked")
		}
	})

	t.Run("different IPs are independent", func(t *testing.T) {
		rl := newRateLimiter(1, time.Minute)
		rl.allow("1.2.3.4")
		if !rl.allow("5.6.7.8") {
			t.Fatal("different IP should not be rate limited")
		}
	})

	t.Run("window resets after duration", func(t *testing.T) {
		rl := newRateLimiter(1, 10*time.Millisecond)
		rl.allow("1.2.3.4")
		if rl.allow("1.2.3.4") {
			t.Fatal("should be rate limited before window expires")
		}
		time.Sleep(15 * time.Millisecond)
		if !rl.allow("1.2.3.4") {
			t.Fatal("should be allowed after window resets")
		}
	})
}
