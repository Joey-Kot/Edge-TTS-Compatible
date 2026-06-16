package edge

import (
	"context"
	"sync"
	"time"
)

type upstreamLimiter struct {
	slots    chan struct{}
	interval time.Duration

	mu       sync.Mutex
	lastSend time.Time
}

func newUpstreamLimiter(concurrency int, interval time.Duration) *upstreamLimiter {
	return &upstreamLimiter{
		slots:    make(chan struct{}, concurrency),
		interval: interval,
	}
}

func (l *upstreamLimiter) acquire(ctx context.Context) (func(), error) {
	select {
	case l.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if err := l.waitTurn(ctx); err != nil {
		<-l.slots
		return nil, err
	}

	return func() {
		<-l.slots
	}, nil
}

func (l *upstreamLimiter) waitTurn(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.interval > 0 && !l.lastSend.IsZero() {
		next := l.lastSend.Add(l.interval)
		if wait := time.Until(next); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return ctx.Err()
			}
		}
	}
	l.lastSend = time.Now()
	return nil
}
