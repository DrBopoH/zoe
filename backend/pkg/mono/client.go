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

func (c *Client) GetClientInfo() (*UserInfoDTO, error) {
	req, err := http.NewRequest("GET", BaseURL+"/personal/client-info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", c.token)

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
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, &RateLimitError{}
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