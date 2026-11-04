package ratelimit

import (
	"testing"
	"time"
)

func TestSlidingWindowLimiter(t *testing.T) {
	limiter := NewLimiter()
	defer limiter.Close()

	key := "test_key_abc"
	limit := 5
	duration := 2 * time.Second

	// 1. Consume 5 tokens
	for i := 1; i <= limit; i++ {
		res := limiter.Allow(LimitOptions{
			Key:      key,
			Limit:    limit,
			Duration: duration,
			Burst:    0,
		})
		if !res.Allowed {
			t.Fatalf("request %d should be allowed", i)
		}
		expectedRemaining := limit - i
		if res.Remaining != expectedRemaining {
			t.Errorf("request %d: expected remaining %d, got %d", i, expectedRemaining, res.Remaining)
		}
		headers := res.FormatHeaders()
		if headers["RateLimit-Limit"] != "5" {
			t.Errorf("expected RateLimit-Limit=5, got %s", headers["RateLimit-Limit"])
		}
		if _, hasRetry := headers["Retry-After"]; hasRetry {
			t.Errorf("successful request should not have Retry-After header")
		}
	}

	// 2. Request 6 should be rejected with 429
	resBlocked := limiter.Allow(LimitOptions{
		Key:      key,
		Limit:    limit,
		Duration: duration,
		Burst:    0,
	})
	if resBlocked.Allowed {
		t.Fatalf("request 6 should be rejected (rate limited)")
	}
	if resBlocked.Remaining != 0 {
		t.Errorf("expected remaining=0, got %d", resBlocked.Remaining)
	}
	headersBlocked := resBlocked.FormatHeaders()
	if _, hasRetry := headersBlocked["Retry-After"]; !hasRetry {
		t.Errorf("blocked request must have Retry-After header")
	}

	// 3. Test Burst Capacity
	burstKey := "test_burst_key"
	for i := 0; i < 3; i++ {
		limiter.Allow(LimitOptions{
			Key:      burstKey,
			Limit:    3,
			Duration: duration,
			Burst:    2,
		})
	}

	// Next request should use burst token
	resBurst := limiter.Allow(LimitOptions{
		Key:      burstKey,
		Limit:    3,
		Duration: duration,
		Burst:    2,
	})
	if !resBurst.Allowed || !resBurst.BurstUsed {
		t.Errorf("expected request to be allowed via burst capacity: allowed=%v, burstUsed=%v", resBurst.Allowed, resBurst.BurstUsed)
	}

	// 4. Test Tiers
	free := ResolveTier("free")
	if free.Limit != 10 {
		t.Errorf("expected free tier limit=10, got %d", free.Limit)
	}
	pro := ResolveTier("pro")
	if pro.Limit != 500 {
		t.Errorf("expected pro tier limit=500, got %d", pro.Limit)
	}
	ent := ResolveTier("enterprise")
	if ent.Limit != 10000 {
		t.Errorf("expected enterprise tier limit=10000, got %d", ent.Limit)
	}
}
