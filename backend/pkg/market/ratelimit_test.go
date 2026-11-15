package market

import (
	"context"
	"testing"
	"time"
)

func TestHostLimiterSpacesRequestsPerHost(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var slept []time.Duration
	l := NewHostLimiter(map[string]time.Duration{"slow.example": 100 * time.Millisecond})
	l.now = func() time.Time { return clock }
	l.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := l.Wait(ctx, "slow.example"); err != nil {
			t.Fatal(err)
		}
	}
	if len(slept) != 2 || slept[0] != 100*time.Millisecond || slept[1] != 200*time.Millisecond {
		t.Fatalf("паузы подряд: %v", slept)
	}
	if err := l.Wait(ctx, "other.example"); err != nil || len(slept) != 2 {
		t.Fatalf("другой хост не ждёт: %v %v", err, slept)
	}
	clock = clock.Add(time.Second) // слот давно прошёл: ждать не нужно
	if err := l.Wait(ctx, "slow.example"); err != nil || len(slept) != 2 {
		t.Fatalf("после паузы: %v %v", err, slept)
	}
	var nilLimiter *HostLimiter
	if err := nilLimiter.Wait(ctx, "x"); err != nil {
		t.Fatal(err)
	}
}

func TestHostLimiterStopsOnCancel(t *testing.T) {
	l := NewHostLimiter(map[string]time.Duration{"h": time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	if err := l.Wait(ctx, "h"); err != nil { // первый запрос идёт сразу
		t.Fatal(err)
	}
	cancel()
	if err := l.Wait(ctx, "h"); err == nil {
		t.Fatal("отмена должна прервать ожидание")
	}
}

func TestDefaultClientHasLimiter(t *testing.T) {
	if NewClient().Limiter == nil {
		t.Fatal("у боевого клиента лимитер включён")
	}
}
