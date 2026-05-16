package server

import (
	"net/http"
	"sync"
	"time"
)

// tokenBucket è un rate limiter a token bucket per IP.
type tokenBucket struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // token al secondo
	capacity float64 // massimo token
}

type bucket struct {
	tokens    float64
	lastRefil time.Time
}

func newTokenBucket(ratePerSec, capacity float64) *tokenBucket {
	tb := &tokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     ratePerSec,
		capacity: capacity,
	}
	// Cleanup goroutine: rimuove bucket inattivi ogni 5 minuti
	go func() {
		for range time.Tick(5 * time.Minute) {
			tb.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for ip, b := range tb.buckets {
				if b.lastRefil.Before(cutoff) {
					delete(tb.buckets, ip)
				}
			}
			tb.mu.Unlock()
		}
	}()
	return tb
}

func (tb *tokenBucket) allow(ip string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, ok := tb.buckets[ip]
	if !ok {
		b = &bucket{tokens: tb.capacity, lastRefil: time.Now()}
		tb.buckets[ip] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.lastRefil).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > tb.capacity { b.tokens = tb.capacity }
	b.lastRefil = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rate limiters globali
var (
	generateLimiter = newTokenBucket(2, 5)  // 2 req/s, burst 5
	pullLimiter     = newTokenBucket(1, 3)  // 1 req/s, burst 3
)

func withRateLimit(tb *tokenBucket, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// Strip port if present
		if i := len(ip) - 1; i >= 0 {
			for i >= 0 && ip[i] != ':' { i-- }
			if i > 0 { ip = ip[:i] }
		}
		if !tb.allow(ip) {
			jsonError(w, http.StatusTooManyRequests, "rate limit exceeded — riprova tra qualche secondo")
			return
		}
		h(w, r)
	}
}
