package market

import (
	"context"
	"sync"
	"time"
)

// HostLimiter держит минимальную паузу между запросами к одному хосту: источники опрашиваются параллельно, и без этого
// десятки рядов FRED или BIS уходили бы пачкой и упирались в чужие лимиты. Хост без записи в gap не ограничен (тесты ходят на 127.0.0.1).
type HostLimiter struct {
	mu    sync.Mutex
	gap   map[string]time.Duration
	next  map[string]time.Time
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

// NewHostLimiter — пауза gap[хост] между началами запросов к хосту.
func NewHostLimiter(gap map[string]time.Duration) *HostLimiter {
	return &HostLimiter{gap: gap, next: map[string]time.Time{}, now: time.Now, sleep: sleepCtx}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Wait занимает очередной слот хоста и ждёт его начала. nil-лимитер и хост без паузы не ждут. Отмена контекста прерывает ожидание.
func (l *HostLimiter) Wait(ctx context.Context, host string) error {
	if l == nil {
		return nil
	}
	g := l.gap[host]
	if g <= 0 {
		return nil
	}
	l.mu.Lock()
	now := l.now()
	at := l.next[host]
	if at.Before(now) {
		at = now
	}
	l.next[host] = at.Add(g)
	l.mu.Unlock()
	if d := at.Sub(now); d > 0 {
		return l.sleep(ctx, d)
	}
	return ctx.Err()
}

// DefaultHostGaps — паузы по хостам из базовых адресов клиента: публичные API без ключей, у большинства есть негласные лимиты.
func DefaultHostGaps() map[string]time.Duration {
	return map[string]time.Duration{
		"fred.stlouisfed.org":    150 * time.Millisecond,
		"stats.bis.org":          250 * time.Millisecond,
		"api.worldbank.org":      200 * time.Millisecond,
		"ec.europa.eu":           200 * time.Millisecond,
		"data-api.ecb.europa.eu": 150 * time.Millisecond,
		"p2p.binance.com":        500 * time.Millisecond,
		"api.coingecko.com":      1500 * time.Millisecond,
		"api.trongrid.io":        400 * time.Millisecond,
	}
}
