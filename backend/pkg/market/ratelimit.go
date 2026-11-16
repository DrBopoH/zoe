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
	slots map[string]chan struct{} // хост → семафор одновременных запросов
}

// NewHostLimiter — пауза gap[хост] между началами запросов к хосту.
func NewHostLimiter(gap map[string]time.Duration) *HostLimiter {
	return &HostLimiter{gap: gap, next: map[string]time.Time{}, now: time.Now, sleep: sleepCtx}
}

// WithSlots ограничивает число одновременных запросов к хосту (хост без записи не ограничен). Пауза между стартами этого не заменяет:
// медленный источник (НБУ при полном опросе) отвечает по десятку секунд, и запросы копятся, пока он не начнёт обрывать соединения.
func (l *HostLimiter) WithSlots(n map[string]int) *HostLimiter {
	l.slots = map[string]chan struct{}{}
	for host, k := range n {
		if k > 0 {
			l.slots[host] = make(chan struct{}, k)
		}
	}
	return l
}

// Acquire — Wait плюс место в очереди одновременных запросов хоста. release вызывается, когда ответ прочитан.
func (l *HostLimiter) Acquire(ctx context.Context, host string) (release func(), err error) {
	if l == nil {
		return func() {}, nil
	}
	if err := l.Wait(ctx, host); err != nil {
		return nil, err
	}
	sem := l.slots[host]
	if sem == nil {
		return func() {}, nil
	}
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
		"bank.gov.ua":            100 * time.Millisecond,
	}
}

// DefaultHostSlots — сколько запросов к хосту могут идти одновременно. НБУ при параллельном опросе всех источников отвечает десятками секунд и рвёт запросы.
func DefaultHostSlots() map[string]int {
	return map[string]int{"bank.gov.ua": 2}
}
