// Package binance — выписка Binance и публичные рыночные данные.
//
// Движения по счёту приходят только из выписки (Wallet → Transaction History → Export), ключа API нет:
// цены монет — публичные минутные свечи, они не требуют подписи.
package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultBaseURL = "https://api.binance.com"

type Client struct {
	base  string
	http  *http.Client
	Sleep func(time.Duration) // для тестов; по умолчанию time.Sleep
	// OnWait сообщает о вынужденной паузе из-за лимита запросов (d > 0), затем 0, когда она закончилась.
	OnWait func(d time.Duration)
	// Stop — true, когда обход отменён (сброс кеша): новые запросы не отправляются, а ожидание лимита прерывается.
	Stop func() bool
	// Workers — сколько окон свечей качается одновременно (0 — 4).
	Workers int

	page func() // вызывается после каждой скачанной страницы свечей (Series: прогресс по запросам)
	reqs atomic.Int64

	mu   sync.Mutex
	syms map[string]Sym // exchangeInfo не меняется за сеанс
	px   map[string]float64
}

func NewClient() *Client {
	return &Client{base: DefaultBaseURL, http: &http.Client{Timeout: 20 * time.Second}}
}

// SetBaseURL — для тестов (подставной сервер).
func (c *Client) SetBaseURL(u string) { c.base = strings.TrimRight(u, "/") }

// ErrStopped — запрос не отправлен: обход отменён.
var ErrStopped = errors.New("binance: обход остановлен")

// APIError — ответ Binance с кодом ошибки.
type APIError struct {
	Status int
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("binance %d (code %d): %s", e.Status, e.Code, e.Msg)
}

// do — публичный GET. 429/418 — лимит запросов: ждём сколько просит Binance и повторяем (минутные цены — сотни запросов).
func (c *Client) do(path string, params url.Values) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		if c.Stop != nil && c.Stop() {
			return nil, ErrStopped
		}
		body, status, retryAfter, err := c.once(path, params)
		if err == nil && (status == http.StatusTooManyRequests || status == 418) && attempt < 4 {
			c.sleep(retryAfter)
			continue
		}
		return body, err
	}
}

// Requests — сколько запросов к Binance сделано за жизнь клиента (для оценки времени до конца).
func (c *Client) Requests() int64 { return c.reqs.Load() }

func (c *Client) sleep(d time.Duration) {
	if c.OnWait != nil && d >= time.Second {
		c.OnWait(d)
		defer c.OnWait(0)
	}
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (c *Client) once(path string, params url.Values) ([]byte, int, time.Duration, error) {
	req, err := http.NewRequest(http.MethodGet, c.base+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, 0, 0, err
	}
	c.reqs.Add(1)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, 0, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 {
		wait := 5 * time.Second
		if v, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && v > 0 {
			wait = time.Duration(v) * time.Second
		}
		if wait > 60*time.Second {
			wait = 60 * time.Second
		}
		return body, resp.StatusCode, wait, nil
	}
	if resp.StatusCode != http.StatusOK {
		e := &APIError{Status: resp.StatusCode}
		_ = json.Unmarshal(body, e)
		if e.Msg == "" {
			e.Msg = strings.TrimSpace(string(body))
		}
		return nil, resp.StatusCode, 0, e
	}
	return body, resp.StatusCode, 0, nil
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// Prices — последние цены всех пар: symbol → price.
func (c *Client) Prices() (map[string]float64, error) {
	body, err := c.do("/api/v3/ticker/price", nil)
	if err != nil {
		return nil, err
	}
	var r []struct{ Symbol, Price string }
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(r))
	for _, p := range r {
		out[p.Symbol] = num(p.Price)
	}
	return out, nil
}

// Sym — торговая пара.
type Sym struct{ Base, Quote string }

// Symbols — все пары биржи: symbol → (base, quote).
func (c *Client) Symbols() (map[string]Sym, error) {
	c.mu.Lock()
	cached := c.syms
	c.mu.Unlock()
	if cached != nil {
		return cached, nil
	}
	body, err := c.do("/api/v3/exchangeInfo", nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		Symbols []struct{ Symbol, BaseAsset, QuoteAsset string } `json:"symbols"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := make(map[string]Sym, len(r.Symbols))
	for _, s := range r.Symbols {
		out[s.Symbol] = Sym{s.BaseAsset, s.QuoteAsset}
	}
	c.mu.Lock()
	c.syms = out
	c.mu.Unlock()
	return out, nil
}
