package market

import (
	"context"
	"sync"
)

// fredParallel — сколько рядов FRED качается одновременно. Публичный CSV без ключа: держим скромно, но не по одному —
// пачка из пяти рядов с 60-летней историей по очереди занимала минуту.
const fredParallel = 4

// fredJob — один ряд FRED: его id и короткая заметка для окна процесса.
type fredJob struct{ ID, Note string }

// fredResult — итог пачки: ряды по id FRED (упавшего ряда в Obs нет), число пропущенных строк, ошибки в порядке заданий.
type fredResult struct {
	Obs     map[string][]fredObs
	Skipped int
	Errs    []string
}

// fredAll скачивает пачку рядов параллельно (до fredParallel сразу). Сбой одного ряда не трогает остальные.
// Прогресс идёт по числу завершённых рядов (5…90 %), так что окно процесса двигается ровно, а не прыжками по таблице.
func (c *Client) fredAll(ctx context.Context, rep Reporter, jobs []fredJob) fredResult {
	type one struct {
		obs     []fredObs
		skipped int
		err     error
	}
	got := make([]one, len(jobs))
	sem := make(chan struct{}, fredParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for i, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			obs, sk, err := c.fred(ctx, j.ID)
			got[i] = one{obs, sk, err}
			mu.Lock()
			done++
			rep.report(5+85*done/len(jobs), j.Note)
			mu.Unlock()
		}()
	}
	wg.Wait()
	res := fredResult{Obs: map[string][]fredObs{}}
	for i, g := range got {
		res.Skipped += g.skipped
		if g.err != nil {
			res.Errs = append(res.Errs, g.err.Error())
			continue
		}
		res.Obs[jobs[i].ID] = g.obs
	}
	return res
}
