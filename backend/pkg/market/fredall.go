package market

import (
	"context"
	"sync"
	"time"
)

// fredParallel — сколько рядов FRED качается одновременно. Публичный CSV без ключа: держим скромно, но не по одному —
// пачка из пяти рядов с 60-летней историей по очереди занимала минуту.
const fredParallel = 4

// fredJob — один ряд FRED: его id, короткая заметка для окна процесса и наш id ряда в кеше.
// Series заполняют только ряды, которые кладутся в кеш как есть или недельными точками: у них хвост докачивается с последней даты (tailStart).
// Пустой Series — ряд всегда качается целиком (считаемые ряды вроде инфляции из индекса и сшитые с историей).
type fredJob struct{ ID, Note, Series string }

// tailStart — с какой даты докачивать ряд, если в кеше он дошёл до last: понедельник недели за две недели до last.
// Запас закрывает правки и пропуски, а понедельник гарантирует целую первую неделю, чтобы недельная точка совпала с сохранённой.
func tailStart(last string) string {
	t, err := time.Parse("2006-01-02", last)
	if err != nil {
		return ""
	}
	t = t.AddDate(0, 0, -14)
	t = t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
	return t.Format("2006-01-02")
}

// fredResult — итог пачки: ряды по id FRED (упавшего ряда в Obs нет), число пропущенных строк, ошибки в порядке заданий.
type fredResult struct {
	Obs     map[string][]fredObs
	Tail    []string // наши id рядов, которые пришли хвостом
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
		tail    bool
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
			since := ""
			if j.Series != "" && c.LastDate != nil {
				if last := c.LastDate(j.Series); last != "" {
					since = tailStart(last)
				}
			}
			obs, sk, err := c.fred(ctx, j.ID, since)
			got[i] = one{obs, sk, err, since != ""}
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
		if g.tail {
			res.Tail = append(res.Tail, jobs[i].Series)
		}
	}
	return res
}
