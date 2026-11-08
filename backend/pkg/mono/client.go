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
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
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
	req, err := c.newRequest("GET", BaseURL+"/personal/client-info")
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, &RateLimitError{}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mono client info status: %d", resp.StatusCode)
	}

	var info UserInfoDTO
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

func (c *Client) GetStatement(accountID string, from, to time.Time) ([]StatementItemDTO, error) {
	url := fmt.Sprintf("%s/personal/statement/%s/%d/%d", BaseURL, accountID, from.Unix(), to.Unix())
	req, err := c.newRequest("GET", url)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, &RateLimitError{}
	}
	if resp.StatusCode == 400 {
		return nil, &BadRequestError{}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mono statement status: %d", resp.StatusCode)
	}

	var items []StatementItemDTO
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return items, nil
}

func (c *Client) newRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", c.token)
	return req, nil
}