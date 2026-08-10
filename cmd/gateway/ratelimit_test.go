package main

import (
	"testing"
)

func TestLimiterAllowsUnderLimit(t *testing.T) {
	l := NewLimiter(3)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestLimiterBlocksOverLimit(t *testing.T) {
	l := NewLimiter(2)
	l.Allow("1.2.3.4")
	l.Allow("1.2.3.4")
	if l.Allow("1.2.3.4") {
		t.Fatal("3rd request should be blocked")
	}
}

func TestLimiterDifferentIPsIndependent(t *testing.T) {
	l := NewLimiter(1)
	ok1 := l.Allow("1.1.1.1")
	ok2 := l.Allow("2.2.2.2")
	if !ok1 || !ok2 {
		t.Fatal("different IPs should be independent")
	}
}
