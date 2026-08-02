package main

import (
	"net/http"
	"time"
)

type rateLimiter struct{}

func newRateLimiter(limit int, window time.Duration) *rateLimiter { return &rateLimiter{} }
func (rl *rateLimiter) allow(ip string) bool                      { return true }
func (rl *rateLimiter) middleware(next http.Handler) http.Handler  { return next }
