package db

// История C2C-ордеров Binance — дополнение к выписке того же счёта (в журнале у P2P есть только монета, без суммы в фиате).
// Лежит в КЕШЕ (cache.db) рядом с журналом, восстанавливается из файла-источника (.zoe, вид binance-p2p).

const binanceP2PSchema = `
CREATE TABLE IF NOT EXISTS binance_p2p (
	id TEXT PRIMARY KEY,             -- номер ордера
	t INTEGER NOT NULL,              -- время создания, мс
	side TEXT NOT NULL,              -- Buy / Sell
	asset TEXT NOT NULL,
	fiat TEXT NOT NULL,
	total REAL NOT NULL,             -- сумма в фиате
	price REAL NOT NULL,
	qty REAL NOT NULL,               -- количество монеты до комиссии
	rate REAL NOT NULL DEFAULT 0,
	fee REAL NOT NULL DEFAULT 0,     -- комиссия в монете
	counterparty TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_binance_p2p_t ON binance_p2p(t);
`

// P2POrder — строка истории C2C-ордеров.
type P2POrder struct {
	ID           string  `json:"id"`
	T            int64   `json:"t"` // мс
	Side         string  `json:"side"`
	Asset        string  `json:"asset"`
	Fiat         string  `json:"fiat"`
	Total        float64 `json:"total"`
	Price        float64 `json:"price"`
	Qty          float64 `json:"qty"`
	Rate         float64 `json:"rate"`
	Fee          float64 `json:"fee"`
	Counterparty string  `json:"counterparty"`
	Status       string  `json:"status"`
}

// SaveBinanceP2P кладёт ордера в кеш. Идемпотентно по номеру ордера: перекрывающиеся выгрузки дополняют друг друга,
// а статус ордера берётся из последней (отменённый ордер мог позже стать завершённым).
func (s *Storage) SaveBinanceP2P(orders []P2POrder) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, o := range orders {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO binance_p2p(id,t,side,asset,fiat,total,price,qty,rate,fee,counterparty,status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			o.ID, o.T, o.Side, o.Asset, o.Fiat, o.Total, o.Price, o.Qty, o.Rate, o.Fee, o.Counterparty, o.Status); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BinanceP2P — все ордера по времени.
func (s *Storage) BinanceP2P() ([]P2POrder, error) {
	rows, err := s.cacheDB.Query(`SELECT id,t,side,asset,fiat,total,price,qty,rate,fee,counterparty,status FROM binance_p2p ORDER BY t, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []P2POrder{}
	for rows.Next() {
		var o P2POrder
		if err := rows.Scan(&o.ID, &o.T, &o.Side, &o.Asset, &o.Fiat, &o.Total, &o.Price, &o.Qty, &o.Rate, &o.Fee, &o.Counterparty, &o.Status); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
