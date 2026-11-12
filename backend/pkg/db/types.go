package db

import "encoding/json"

// ───────── Статика (core.db): то, что нельзя терять ─────────

// AliasLink — запомненная связь «алиас → счёт/карта».
// Confirmed=true  → то же самое сейчас подтверждает кеш (зелёный).
// Confirmed=false → данные есть только в статике (жёлтый).
type AliasLink struct {
	Alias             string `json:"alias"`
	AccountIdentifier string `json:"account_identifier"`
	ParentIdentifier  string `json:"parent_identifier,omitempty"`
	Confirmed         bool   `json:"confirmed"`
}

// UserAccount — плоское представление счетов лица (производное от alias_links).
type UserAccount struct {
	AccountIdentifier string `json:"account_identifier"`
	ParentIdentifier  string `json:"parent_identifier,omitempty"`
	Note              string `json:"note,omitempty"` // алиас, через который счёт попал к лицу
	Confirmed         bool   `json:"confirmed"`
}

type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	AvatarURL string `json:"avatar_url"`

	Aliases    []string      `json:"aliases"`
	AliasLinks []AliasLink   `json:"alias_links"`
	AliasBans  []AliasLink   `json:"alias_bans"` // теневые баны: кеш НЕ имеет права это подхватывать
	Accounts   []UserAccount `json:"accounts"`   // производное
	IBANs      []string      `json:"ibans"`      // производное
}

// AliasStructure — структура алиаса (в т.ч. «бесхозного»: user_id == "").
type AliasStructure struct {
	Alias    string      `json:"alias"`
	UserID   string      `json:"user_id"`
	Accounts []AliasLink `json:"accounts"`
}

// ───────── Кеш (cache.db): всё восполняемо ─────────

type Account struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Balance      float64  `json:"balance"`
	CreditLimit  float64  `json:"credit_limit"`
	Type         string   `json:"type"`
	Currency     int      `json:"currency_code"`
	IBAN         string   `json:"iban"`
	MaskedPan    string   `json:"masked_pan"` // главный PAN (тот, что виден в приложении банка)
	Pans         []string `json:"pans"`       // все известные PAN счёта, главный первым
	Source       string   `json:"source"`     // api | import
	OpenedAt     int64    `json:"opened_at"`  // unix; первая операция выписки (0 — ещё неизвестно)
	ClosedAt     int64    `json:"closed_at"`  // unix; 0 — счёт жив. Банк даты не отдаёт: счёт пропал из ответа банка → закрыт
	IsClosed     bool     `json:"is_closed"`
	ManualOpened bool     `json:"manual_opened"` // дата задана руками (core.db), синк её не трогает
	ManualClosed bool     `json:"manual_closed"`

	BankKey   string `json:"bank_key,omitempty"`   // бренд счёта-«не банка»: icu (брокерский отчёт) или binance
	HasDetail bool   `json:"has_detail,omitempty"` // у счёта есть своя модалка (журнал ICU, портфель Binance)

	Portfolio *PortfolioView `json:"portfolio,omitempty"` // состав счёта для карточки (считается на чтении): облигации ICU или кошельки Binance
}

// PortfolioView — что лежит на счёте-«не банке».
//
//	bonds   — свободные деньги, стоимость всех облигаций и позиции (штуки, ISIN, накопленный купон)
//	wallets — Binance: кошелёк → монета (JSON из последней «Разведки»: binance.WalletLine)
type PortfolioView struct {
	Kind     string          `json:"kind"`
	Cash     float64         `json:"cash"`
	Invested float64         `json:"invested"`
	Bonds    []BondLine      `json:"bonds,omitempty"`
	Wallets  json.RawMessage `json:"wallets,omitempty"`
	USDTUAH  float64         `json:"usdt_uah,omitempty"` // курс из последней «Разведки»: баланс Binance на карточке показывается в долларах
}

// BondLine — позиция по облигации: накопленный купон (НКД) на всю позицию.
type BondLine struct {
	ISIN    string  `json:"isin"`
	Name    string  `json:"name"`
	Qty     float64 `json:"qty"`
	Accrued float64 `json:"accrued"`
}

type Transaction struct {
	ID           string  `json:"id"`
	AccountID    string  `json:"account_id"`
	Time         string  `json:"time"`
	Description  string  `json:"description"`
	Comment      string  `json:"comment"`
	CounterIBAN  string  `json:"counter_iban"`
	CounterName  string  `json:"counter_name"`
	MCC          int     `json:"mcc"`
	Amount       float64 `json:"amount"`
	CurrencyCode int     `json:"currency_code"`

	// Подробности операции (для таблицы истории по карте). Как лежит в кеше, парсеры/комиссии не пересчитываются.
	OperationAmount float64 `json:"operation_amount"` // сумма в валюте операции
	Commission      float64 `json:"commission"`
	Cashback        float64 `json:"cashback"`
	IsHold          bool    `json:"is_hold"`

	ResolvedUserID string `json:"resolved_user_id,omitempty"` // считается на чтении, в кеш не пишется

	// Перемещение между своими счетами (на брокерский счёт и записи его журнала): не доход и не расход.
	Internal        bool   `json:"internal,omitempty"`
	LinkedAccountID string `json:"linked_account_id,omitempty"` // куда ушла операция кеша (брокерский счёт)

	// Вид операции счёта-«не банка» (считается на чтении): у Binance deposit | withdraw | p2p | dividend | bonus. Карточные операции без вида.
	Kind   string `json:"kind,omitempty"`
	Wallet string `json:"wallet,omitempty"` // у Binance: кошелёк, где произошло движение (Spot, Funding, Strategy, USD-M Futures)
}
