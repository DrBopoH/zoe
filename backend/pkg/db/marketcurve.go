package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
)

// Кривая доходности рынка (срок в годах → доходность в %), которую пользователь задаёт сам по экрану брокера: несколько
// точек. Ими оцениваются бумаги, у которых нет цены. Данные пользователя — в core.db.
const marketCurveSchema = `
CREATE TABLE IF NOT EXISTS market_curves (
	name TEXT PRIMARY KEY,
	points TEXT NOT NULL DEFAULT '[]', -- JSON [{"years":1,"ytm":15.3}]
	updated_at INTEGER NOT NULL DEFAULT 0
);`

type CurvePoint struct {
	Years float64 `json:"years"`
	YTM   float64 `json:"ytm"`
}

func (s *Storage) initMarketCurves() error {
	_, err := s.coreDB.Exec(marketCurveSchema)
	return err
}

func (s *Storage) MarketCurve(name string) ([]CurvePoint, error) {
	if !marketListName.MatchString(name) {
		return nil, errors.New("имя кривой: a-z, 0-9, _ и -")
	}
	var raw string
	err := s.coreDB.QueryRow(`SELECT points FROM market_curves WHERE name = ?`, name).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	pts := []CurvePoint{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &pts); err != nil {
			return nil, err
		}
	}
	return pts, nil
}

// SaveMarketCurve заменяет кривую: точки вне разумного (срок 0…50 лет, доходность 0…100 %) отбрасываются, по сроку сортируются.
func (s *Storage) SaveMarketCurve(name string, pts []CurvePoint, nowUnix int64) ([]CurvePoint, error) {
	if !marketListName.MatchString(name) {
		return nil, errors.New("имя кривой: a-z, 0-9, _ и -")
	}
	clean := []CurvePoint{}
	for _, p := range pts {
		if p.Years > 0 && p.Years <= 50 && p.YTM > 0 && p.YTM < 100 {
			clean = append(clean, p)
		}
	}
	sort.Slice(clean, func(i, j int) bool { return clean[i].Years < clean[j].Years })
	raw, _ := json.Marshal(clean)
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`
		INSERT INTO market_curves(name, points, updated_at) VALUES(?,?,?)
		ON CONFLICT(name) DO UPDATE SET points = excluded.points, updated_at = excluded.updated_at`,
		name, string(raw), nowUnix)
	return clean, err
}
