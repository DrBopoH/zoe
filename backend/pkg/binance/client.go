// Package binance — read-only клиент Binance (ключ только на чтение: без торговли и вывода).
//
// Подпись: HMAC-SHA256 от строки запроса секретом ключа, ключ — в заголовке X-MBX-APIKEY.
package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	key, secret string
	base        string
	http        *http.Client
	Sleep       func(time.Duration) // для тестов; по умолчанию time.Sleep

	hk   histHooks // ход восстановления истории (ставит BuildHistory)
	reqs atomic.Int64

	mu     sync.Mutex
	syms   map[string]Sym // exchangeInfo не меняется за сеанс
	px     map[string]float64
	offset time.Duration // серверное время − локальное (часы на компе могут уезжать; Binance не принимает отклонение >recvWindow)
}

func NewClient(key, secret string) *Client {
	return &Client{key: strings.TrimSpace(key), secret: strings.TrimSpace(secret), base: DefaultBaseURL, http: &http.Client{Timeout: 20 * time.Second}}
}

// SetBaseURL — для тестов (подставной сервер).
func (c *Client) SetBaseURL(u string) { c.base = strings.TrimRight(u, "/") }

// APIError — ответ Binance с кодом ошибки.
type APIError struct {
	Status int
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("binance %d (code %d): %s", e.Status, e.Code, e.Msg)
}

func (c *Client) do(method, path string, params url.Values, signed bool) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	for attempt := 0; ; attempt++ {
		body, status, retryAfter, err := c.once(method, path, params, signed)
		// 429/418 — лимит запросов: ждём сколько просит Binance и повторяем (история тянет сотни запросов)
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
	if c.hk.wait != nil && d >= time.Second { // вынужденная пауза (лимит) — в панели «остывание»
		c.hk.wait(d)
		defer c.hk.wait(0)
	}
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (c *Client) once(method, path string, params url.Values, signed bool) ([]byte, int, time.Duration, error) {
	p := url.Values{}
	for k, v := range params {
		p[k] = v
	}
	if signed {
		c.mu.Lock()
		off := c.offset
		c.mu.Unlock()
		p.Set("timestamp", strconv.FormatInt(time.Now().Add(off).UnixMilli(), 10))
		p.Set("recvWindow", "10000")
	}
	query := p.Encode()
	if signed {
		mac := hmac.New(sha256.New, []byte(c.secret))
		mac.Write([]byte(query))
		query += "&signature=" + hex.EncodeToString(mac.Sum(nil))
	}
	req, err := http.NewRequest(method, c.base+path+"?"+query, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	if c.key != "" {
		req.Header.Set("X-MBX-APIKEY", c.key)
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

// SyncTime подгоняет часы под серверные Binance (один запрос).
func (c *Client) SyncTime() error {
	body, err := c.do("GET", "/api/v3/time", nil, false)
	if err != nil {
		return err
	}
	var t struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.Unmarshal(body, &t); err != nil || t.ServerTime == 0 {
		return fmt.Errorf("binance time: %v", err)
	}
	c.mu.Lock()
	c.offset = time.UnixMilli(t.ServerTime).Sub(time.Now())
	c.mu.Unlock()
	return nil
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// ───────────────────────── балансы ─────────────────────────

// Spot — остатки спот-кошелька (free + locked), только ненулевые.
func (c *Client) Spot() (map[string]float64, error) {
	body, err := c.do("GET", "/api/v3/account", url.Values{"omitZeroBalances": {"true"}}, true)
	if err != nil {
		return nil, err
	}
	var r struct {
		Balances []struct{ Asset, Free, Locked string } `json:"balances"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, b := range r.Balances {
		if v := num(b.Free) + num(b.Locked); v != 0 {
			out[b.Asset] += v
		}
	}
	return out, nil
}

// Funding — кошелёк Funding (P2P, Pay и т.п.).
func (c *Client) Funding() (map[string]float64, error) {
	body, err := c.do("POST", "/sapi/v1/asset/get-funding-asset", url.Values{"needBtcValuation": {"false"}}, true)
	if err != nil {
		return nil, err
	}
	var r []struct{ Asset, Free, Locked, Freeze string }
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, b := range r {
		if v := num(b.Free) + num(b.Locked) + num(b.Freeze); v != 0 {
			out[b.Asset] += v
		}
	}
	return out, nil
}

func (c *Client) earn(path string, field string) (map[string]float64, error) {
	out := map[string]float64{}
	for page := 1; page <= 20; page++ {
		body, err := c.do("GET", path, url.Values{"current": {strconv.Itoa(page)}, "size": {"100"}}, true)
		if err != nil {
			return nil, err
		}
		var r struct {
			Rows  []map[string]any `json:"rows"`
			Total int              `json:"total"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		for _, row := range r.Rows {
			asset, _ := row["asset"].(string)
			if s, ok := row[field].(string); ok && asset != "" {
				out[asset] += num(s)
			}
		}
		if len(r.Rows) < 100 {
			break
		}
	}
	return out, nil
}

// EarnFlexible — Simple Earn, гибкие продукты.
func (c *Client) EarnFlexible() (map[string]float64, error) {
	return c.earn("/sapi/v1/simple-earn/flexible/position", "totalAmount")
}

// EarnLocked — Simple Earn, фиксированные продукты.
func (c *Client) EarnLocked() (map[string]float64, error) {
	return c.earn("/sapi/v1/simple-earn/locked/position", "amount")
}

// Prices — последние цены всех пар (публично): symbol → price.
func (c *Client) Prices() (map[string]float64, error) {
	body, err := c.do("GET", "/api/v3/ticker/price", nil, false)
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
