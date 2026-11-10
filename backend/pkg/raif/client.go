package raif

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"economic/pkg/mono"
)

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

type RaifAccountDTO struct {
	ID          string  `json:"id"`
	IBAN        string  `json:"iban"`
	MaskedPan   string  `json:"masked_pan"`
	Title       string  `json:"title"`
	Balance     float64 `json:"balance"`
	CreditLimit float64 `json:"credit_limit"`
	Currency    int     `json:"currency_code"`
	Type        string  `json:"type"`
}

type RaifStatementDTO struct {
	ID            string  `json:"id"`
	AccountID     string  `json:"account_id"`
	Time          int64   `json:"time"`
	Description   string  `json:"description"`
	Comment       string  `json:"comment"`
	CounterEdrpou string  `json:"counter_edrpou"`
	CounterIBAN   string  `json:"counter_iban"`
	CounterName   string  `json:"counter_name"`
	MCC           int     `json:"mcc"`
	Amount        float64 `json:"amount"`
	CurrencyCode  int     `json:"currency_code"`
}

type UserInfoResponse struct {
	Name     string           `json:"name"`
	Accounts []RaifAccountDTO `json:"accounts"`
}

func NewClient(token string, baseURL ...string) *Client {
	url := "https://api.raiffeisen.ua/v1"
	if len(baseURL) > 0 && baseURL[0] != "" {
		url = baseURL[0]
	}

	return &Client{
		token:   token,
		baseURL: url,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetUserInfo() (string, []mono.AccountDTO, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/personal/user-info", c.baseURL), nil)
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("User-Agent", "Economic-Raif-Crawler/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("raif api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("raif api status %d: %s", resp.StatusCode, string(body))
	}

	var data UserInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", nil, err
	}

	monoAccounts := make([]mono.AccountDTO, len(data.Accounts))
	for i, acc := range data.Accounts {
		maskedPans := []string{}
		if acc.MaskedPan != "" {
			maskedPans = append(maskedPans, acc.MaskedPan)
		}

		accType := acc.Type
		if accType == "" {
			accType = "raif"
		}

		monoAccounts[i] = mono.AccountDTO{
			ID:           acc.ID,
			IBAN:         acc.IBAN,
			MaskedPan:    maskedPans,
			Balance:      toCents(acc.Balance),
			CreditLimit:  toCents(acc.CreditLimit),
			Type:         accType,
			CurrencyCode: acc.Currency,
		}
	}

	return data.Name, monoAccounts, nil
}

func (c *Client) GetStatement(accountID string, from, to time.Time) ([]mono.StatementItemDTO, error) {
	url := fmt.Sprintf("%s/personal/statement/%s/%d/%d", c.baseURL, accountID, from.Unix(), to.Unix())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("User-Agent", "Economic-Raif-Crawler/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("raif statement request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("raif statement status %d: %s", resp.StatusCode, string(body))
	}

	var items []RaifStatementDTO
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	monoItems := make([]mono.StatementItemDTO, len(items))
	for i, st := range items {
		monoItems[i] = mono.StatementItemDTO{
			ID:            st.ID,
			Time:          st.Time,
			Description:   st.Description,
			Comment:       st.Comment,
			CounterEdrpou: st.CounterEdrpou,
			CounterIban:   st.CounterIBAN,
			CounterName:   st.CounterName,
			MCC:           st.MCC,
			Amount:        toCents(st.Amount),
			CurrencyCode:  st.CurrencyCode,
		}
	}

	return monoItems, nil
}
