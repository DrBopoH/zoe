package db

import (
	"regexp"
	"strings"
)

// IBANMask — скрытый вид IBAN. Полный номер хранится отдельно; masked пустым не бывает.
type IBANMask struct {
	Masked string `json:"masked"`
	Custom bool   `json:"custom"` // true — псевдоним-имя, заданное вручную (карандаш)
}

const maxMaskLen = 40

var reBinanceID = regexp.MustCompile(`^BN\d{6,12}$`)

// GenerateMask — автоматический скрытый IBAN цифрами: страна+контрольные, код банка, 4 первые цифры счёта, 4 последние.
// UA44 322001 0000026000000009976 → «UA44-322001-0000-9976». Флаг по коду страны и значок банка по коду рисует фронт на лету.
// Профиль Binance (BN + id пользователя) — «BN-0041»: префикс на экране заменяется значком Binance. Для карты маски нет.
func GenerateMask(iban string) string {
	n := normID(iban)
	if reBinanceID.MatchString(n) {
		return "BN-" + n[len(n)-4:]
	}
	if !isIBANLike(n) || len(n) < 15 {
		return ""
	}
	bank := 4 // у не-UA код банка в IBAN свой, берём 4 знака после контрольных
	if strings.HasPrefix(n, "UA") && len(n) == 29 {
		bank = 6 // МФО
	}
	head := 4 + bank
	if len(n) < head+8 {
		return ""
	}
	return n[:4] + "-" + n[4:head] + "-" + n[head:head+4] + "-" + n[len(n)-4:]
}

// IBANMasks — скрытые виды всех известных IBAN (счета кеша, структуры алиасов, ручные правки). Нет записи — генерируем и пишем.
func (s *Storage) IBANMasks() (map[string]IBANMask, error) {
	known := map[string]string{} // normID → как записан
	if accs, err := s.GetAccounts(); err == nil {
		for _, a := range s.withBinanceProfile(accs) {
			known[normID(a.IBAN)] = a.IBAN
		}
	}
	for _, q := range []string{
		`SELECT account_identifier FROM alias_accounts`,
		`SELECT parent_identifier FROM alias_accounts WHERE parent_identifier != ''`,
		`SELECT iban FROM account_overrides WHERE iban != ''`,
		`SELECT iban FROM iban_pans`,
	} {
		rows, err := s.coreDB.Query(q)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				known[normID(id)] = id
			}
		}
		rows.Close()
	}

	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	out := map[string]IBANMask{}
	rows, err := s.coreDB.Query(`SELECT iban, masked, custom FROM iban_masks`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var iban, masked string
		var custom int
		if rows.Scan(&iban, &masked, &custom) == nil {
			out[normID(iban)] = IBANMask{masked, custom != 0}
		}
	}
	rows.Close()
	for n, raw := range known {
		if _, ok := out[n]; ok || n == "" {
			continue
		}
		g := GenerateMask(raw)
		if g == "" {
			continue
		}
		if _, err := s.coreDB.Exec(`INSERT OR IGNORE INTO iban_masks(iban, masked, custom) VALUES(?,?,0)`, n, g); err != nil {
			return nil, err
		}
		out[n] = IBANMask{g, false}
	}
	return out, nil
}

// SetIBANMask — карандаш: свой псевдоним-имя счёта. Пустое значение возвращает автоматическую маску.
func (s *Storage) SetIBANMask(iban, masked string) (IBANMask, error) {
	n := normID(iban)
	masked = strings.Join(strings.Fields(masked), " ")
	if r := []rune(masked); len(r) > maxMaskLen {
		masked = string(r[:maxMaskLen])
	}
	m := IBANMask{Masked: masked, Custom: masked != ""}
	if !m.Custom {
		if m.Masked = GenerateMask(n); m.Masked == "" {
			return m, nil // не IBAN: нечего генерировать, запись не нужна
		}
	}
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	custom := 0
	if m.Custom {
		custom = 1
	}
	_, err := s.coreDB.Exec(`INSERT INTO iban_masks(iban, masked, custom) VALUES(?,?,?)
		ON CONFLICT(iban) DO UPDATE SET masked=excluded.masked, custom=excluded.custom`, n, m.Masked, custom)
	return m, err
}
