package mono

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const BaseURL = "https://api.monobank.ua"

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: BaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type AccountDTO struct {
	ID           string   `json:"id"`
	SendId       string   `json:"sendId"`
	Balance      int64    `json:"balance"`
	CreditLimit  int64    `json:"creditLimit"`
	Type         string   `json:"type"`
	CurrencyCode int      `json:"currencyCode"`
	CashbackType string   `json:"cashbackType"`
	MaskedPan    []string `json:"maskedPan"`
	IBAN         string   `json:"iban"`
}

type UserInfoDTO struct {
	Name       string       `json:"name"`
	WebHookURL string       `json:"webHookUrl"`
	Accounts   []AccountDTO `json:"accounts"`
}

type StatementItemDTO struct {
	ID              string `json:"id"`
	Time            int64  `json:"time"`
	Description     string `json:"description"`
	Comment         string `json:"comment"`
	CounterEdrpou   string `json:"counterEdrpou"`
	CounterIban     string `json:"counterIban"`
	CounterName     string `json:"counterName"`
	MCC             int    `json:"mcc"`
	OriginalMCC     int    `json:"originalMcc"`
	Amount          int64  `json:"amount"`
	OperationAmount int64  `json:"operationAmount"`
	CurrencyCode    int    `json:"currencyCode"`
	CommissionRate  int64  `json:"commissionRate"`
	CashbackAmount  int64  `json:"cashbackAmount"`
	IsHold          bool   `json:"isHold"`
}

type RateLimitError struct{}

func (e *RateLimitError) Error() string {
	return "rate limit 429: mono api limits to 1 req / 60 sec"
}

type BadRequestError struct{}

func (e *BadRequestError) Error() string {
	return "bad request 400: reached account creation limit"
}

func (c *Client) GetClientInfo() (*UserInfoDTO, error) {
	var info UserInfoDTO
	if err := c.get("/personal/client-info", "client info", false, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) GetStatement(accountID string, from, to time.Time) ([]StatementItemDTO, error) {
	var items []StatementItemDTO
	path := fmt.Sprintf("/personal/statement/%s/%d/%d", accountID, from.Unix(), to.Unix())
	if err := c.get(path, "statement", true, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// get — GET к API и разбор JSON в out. 429 → RateLimitError; 400 → BadRequestError, если badRequest
// (у выписки это «достигнут лимит создания счетов»), иначе обычная ошибка статуса.
func (c *Client) get(path, what string, badRequest bool, out any) error {
	req, err := c.newRequest("GET", c.baseURL+path)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return &RateLimitError{}
	case resp.StatusCode == http.StatusBadRequest && badRequest:
		return &BadRequestError{}
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("mono %s status: %d", what, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) newRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", c.token)
	return req, nil
}
