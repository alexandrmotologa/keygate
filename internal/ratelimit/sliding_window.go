package ratelimit

import (
	"math"
	"strconv"
	"sync"
	"time"
)

// Result represents the evaluation output of the rate limiter.
type Result struct {
	Allowed    bool          // Whether the request is permitted
	Limit      int           // Maximum permitted requests in window
	Remaining  int           // Remaining quota in window
	ResetSec   int           // Seconds until current window reset
	RetryAfter int           // Seconds to wait before retry (if rate limited)
	BurstUsed  bool          // Whether burst capacity was drawn upon
	Duration   time.Duration // Duration of the window
}

// WindowCounter tracks request counts across consecutive time windows.
type WindowCounter struct {
	mu           sync.Mutex
	currentStart int64
	currentCount int
	prevCount    int
	burstTokens  float64
	lastRefill   time.Time
}

// Limiter implements sliding-window token-bucket rate limiting.
type Limiter struct {
	mu       sync.RWMutex
	counters map[string]*WindowCounter
	cleaner  *time.Ticker
	stopCh   chan struct{}
}

// NewLimiter creates a new thread-safe sliding window rate limiter.
func NewLimiter() *Limiter {
	limiter := &Limiter{
		counters: make(map[string]*WindowCounter),
		cleaner:  time.NewTicker(5 * time.Minute),
		stopCh:   make(chan struct{}),
	}

	go limiter.cleanupLoop()

	return limiter
}

// Close terminates background cleanup workers.
func (l *Limiter) Close() {
	close(l.stopCh)
	l.cleaner.Stop()
}

func (l *Limiter) cleanupLoop() {
	for {
		select {
		case <-l.cleaner.C:
			l.mu.Lock()
			nowSec := time.Now().Unix()
			for key, counter := range l.counters {
				counter.mu.Lock()
				// Evict counters inactive for more than 10 minutes
				if nowSec-counter.currentStart > 600 {
					delete(l.counters, key)
				}
				counter.mu.Unlock()
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}

func (l *Limiter) getCounter(key string, nowSec int64, windowSec int64, burstCapacity int) *WindowCounter {
	l.mu.RLock()
	c, exists := l.counters[key]
	l.mu.RUnlock()

	if exists {
		return c
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double check
	if c, exists = l.counters[key]; exists {
		return c
	}

	currentStart := (nowSec / windowSec) * windowSec
	c = &WindowCounter{
		currentStart: currentStart,
		burstTokens:  float64(burstCapacity),
		lastRefill:   time.Now(),
	}
	l.counters[key] = c
	return c
}

// LimitOptions configures sliding window parameters.
type LimitOptions struct {
	Key      string        // Unique identifier (e.g. key_id or client IP)
	Limit    int           // Quota limit
	Duration time.Duration // Window duration (default 60s)
	Burst    int           // Burst capacity beyond limit
}

// Allow evaluates whether a request with the given options is allowed.
func (l *Limiter) Allow(opts LimitOptions) Result {
	if opts.Limit <= 0 {
		return Result{
			Allowed:   true,
			Limit:     0,
			Remaining: 0,
		}
	}

	duration := opts.Duration
	if duration <= 0 {
		duration = 60 * time.Second
	}
	windowSec := int64(duration.Seconds())
	if windowSec <= 0 {
		windowSec = 1
	}

	now := time.Now()
	nowSec := now.Unix()
	currentWindowStart := (nowSec / windowSec) * windowSec
	elapsed := nowSec - currentWindowStart
	resetSec := int(windowSec - elapsed)
	if resetSec <= 0 {
		resetSec = 1
	}

	counter := l.getCounter(opts.Key, nowSec, windowSec, opts.Burst)

	counter.mu.Lock()
	defer counter.mu.Unlock()

	// Rotate windows if needed
	if counter.currentStart < currentWindowStart {
		if counter.currentStart == currentWindowStart-windowSec {
			// Previous consecutive window
			counter.prevCount = counter.currentCount
		} else {
			// Gap of two or more windows
			counter.prevCount = 0
		}
		counter.currentStart = currentWindowStart
		counter.currentCount = 0
	}

	// Refill burst tokens if burst configured
	burstUsed := false
	if opts.Burst > 0 {
		elapsedRefill := now.Sub(counter.lastRefill).Seconds()
		refillRate := float64(opts.Limit) / float64(windowSec) // tokens/sec
		counter.burstTokens = math.Min(float64(opts.Burst), counter.burstTokens+(elapsedRefill*refillRate))
		counter.lastRefill = now
	}

	// Calculate sliding window estimated count:
	// weight = 1 - (elapsed / windowSize)
	// estimatedCount = (prevCount * weight) + currentCount
	weight := 1.0 - (float64(elapsed) / float64(windowSec))
	if weight < 0 {
		weight = 0
	}
	estimatedCount := int(math.Floor(float64(counter.prevCount)*weight)) + counter.currentCount

	if estimatedCount+1 <= opts.Limit {
		counter.currentCount++
		remaining := opts.Limit - (estimatedCount + 1)
		return Result{
			Allowed:    true,
			Limit:      opts.Limit,
			Remaining:  remaining,
			ResetSec:   resetSec,
			RetryAfter: 0,
			BurstUsed:  false,
			Duration:   duration,
		}
	}

	// Check if burst capacity can absorb this request
	if opts.Burst > 0 && counter.burstTokens >= 1.0 {
		counter.burstTokens -= 1.0
		counter.currentCount++
		burstUsed = true
		return Result{
			Allowed:    true,
			Limit:      opts.Limit,
			Remaining:  0,
			ResetSec:   resetSec,
			RetryAfter: 0,
			BurstUsed:  burstUsed,
			Duration:   duration,
		}
	}

	// Rate limit exceeded
	return Result{
		Allowed:    false,
		Limit:      opts.Limit,
		Remaining:  0,
		ResetSec:   resetSec,
		RetryAfter: resetSec,
		BurstUsed:  false,
		Duration:   duration,
	}
}

// FormatHeaders returns standard IETF RateLimit HTTP headers as key-value pairs.
func (r Result) FormatHeaders() map[string]string {
	headers := make(map[string]string)
	headers["RateLimit-Limit"] = strconv.Itoa(r.Limit)
	headers["RateLimit-Remaining"] = strconv.Itoa(r.Remaining)
	headers["RateLimit-Reset"] = strconv.Itoa(r.ResetSec)

	if !r.Allowed {
		headers["Retry-After"] = strconv.Itoa(r.RetryAfter)
	}

	return headers
}
