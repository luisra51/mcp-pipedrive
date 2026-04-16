package internal

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

type MultiLimiter struct {
	limiters []*rate.Limiter
}

func NewMultiLimiter(limiters ...*rate.Limiter) *MultiLimiter {
	return &MultiLimiter{limiters: limiters}
}

func (m *MultiLimiter) Wait(ctx context.Context) error {
	for _, limiter := range m.limiters {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// NewPipedriveLimiter approximates Pipedrive's documented limits:
//   - Burst: ~10 requests per 2 seconds per token (≈ 5 req/s steady).
//   - Daily token budget varies by plan/seats; we cap at a conservative
//     per-minute bucket to avoid bursts exhausting the daily quota.
//
// Tune via code when real usage confirms different numbers.
func NewPipedriveLimiter() *MultiLimiter {
	perSecond := rate.NewLimiter(rate.Every(200*time.Millisecond), 10) // 5 req/s, burst 10
	perMinute := rate.NewLimiter(rate.Every(time.Minute/200), 200)     // 200 req/min
	perHour := rate.NewLimiter(rate.Every(time.Hour/5000), 500)        // 5k req/hour
	return NewMultiLimiter(perSecond, perMinute, perHour)
}
