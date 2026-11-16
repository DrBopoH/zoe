package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Группа «Экономика» — данные, из которых собирается кеш макро-блока (ряды, снимки). Тот же файл `.zoe` (Format, Hash, gzip, Created),
// но другая папка и другой вид: в «Выписки» и «Загрузить» эти файлы не попадают. Первый набор — часовые снимки крипто-контура
// (P2P-цена, комиссии сетей, доминация): источник отдаёт их только «сейчас», заново не скачать, поэтому им нужен файл, а не кеш.
const (
	GroupAccounts = "accounts" // Счета: пустое значение поля тоже означает её (старые файлы не мигрируют)
	GroupEconomy  = "economy"

	// SetChainTicks — набор часовых снимков контура (папка economy/chain-ticks, файл на месяц).
	SetChainTicks = "chain-ticks"
)

// Dataset — таблица рядов набора: id ряда → точки по возрастанию времени (unix-секунды, значение).
type Dataset struct {
	Name   string             `json:"name"` // набор и период, например chain-ticks-2026-11
	Step   int64              `json:"step,omitempty"`
	Series map[string]*Points `json:"series"`
}

// Points — параллельные массивы времени и значений (компактнее списка объектов).
type Points struct {
	T []int64   `json:"t"`
	V []float64 `json:"v"`
}

// Tick — точка ряда.
type Tick struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

// groupDir — корневая папка группы файла: у Счетов это сам каталог источников (как было), у Экономики — его подпапка economy.
func (s Store) groupDir(f *File) string {
	if f.Group == GroupEconomy {
		return filepath.Join(s.Dir, GroupEconomy)
	}
	return s.Dir
}

func monthName(set string, slot int64) string {
	return set + "-" + time.Unix(slot, 0).UTC().Format("2006-01")
}

// ListEconomy — файлы группы «Экономика» одного набора (card пустой — всех наборов), по возрастанию начала периода.
func (s Store) ListEconomy(card string) ([]Entry, map[string]string) {
	var out []Entry
	bad := map[string]string{}
	root := filepath.Join(s.Dir, GroupEconomy)
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, bad
	}
	for _, d := range dirs {
		if !d.IsDir() || (card != "" && d.Name() != folder(card)) {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(root, d.Name()))
		for _, e := range files {
			if e.IsDir() || !strings.HasSuffix(e.Name(), Ext) {
				continue
			}
			p := filepath.Join(root, d.Name(), e.Name())
			f, err := Read(p)
			if err != nil {
				bad[p] = err.Error()
				continue
			}
			out = append(out, Entry{p, f})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File.From != out[j].File.From {
			return out[i].File.From < out[j].File.From
		}
		return out[i].Path < out[j].Path
	})
	return out, bad
}

// AppendTicks кладёт значения рядов в слот сетки (slot — unix-секунды начала слота) в файл месяца набора set. Повторная запись в тот же слот
// заменяет значение, новый слот добавляется, новый месяц — новый файл. Прежний файл месяца заменяется новым только после успешной записи.
func (s Store) AppendTicks(set string, step, slot int64, vals map[string]float64) error {
	if len(vals) == 0 {
		return nil
	}
	name := monthName(set, slot)
	list, _ := s.ListEconomy(set)
	var old *Entry
	data := &Dataset{Name: name, Step: step, Series: map[string]*Points{}}
	for i := range list {
		if d := list[i].File.Data; d != nil && d.Name == name {
			old = &list[i]
			for id, p := range d.Series {
				cp := &Points{T: append([]int64(nil), p.T...), V: append([]float64(nil), p.V...)}
				data.Series[id] = cp
			}
			break
		}
	}
	for id, v := range vals {
		p := data.Series[id]
		if p == nil {
			p = &Points{}
			data.Series[id] = p
		}
		if n := len(p.T); n > 0 && p.T[n-1] == slot {
			p.V[n-1] = v
		} else if n == 0 || p.T[n-1] < slot {
			p.T, p.V = append(p.T, slot), append(p.V, v)
		} else { // слот раньше последнего: вставка с сохранением порядка
			i := sort.Search(n, func(i int) bool { return p.T[i] >= slot })
			if i < n && p.T[i] == slot {
				p.V[i] = v
				continue
			}
			p.T = append(p.T[:i], append([]int64{slot}, p.T[i:]...)...)
			p.V = append(p.V[:i], append([]float64{v}, p.V[i:]...)...)
		}
	}
	f := &File{Name: name, Origin: OriginAPI, Kind: KindDataset, Group: GroupEconomy, Card: set, Data: data}
	dst, added, err := s.Put(f)
	if err != nil {
		return err
	}
	if old != nil && added && old.Path != dst {
		if err := os.Remove(old.Path); err != nil {
			return fmt.Errorf("прежний файл месяца: %w", err)
		}
	}
	return nil
}

// ReadTicks — точки ряда id набора set со времени since (unix, 0 — все) из всех месяцев по возрастанию времени.
func (s Store) ReadTicks(set, id string, since int64) []Tick {
	list, _ := s.ListEconomy(set)
	out := []Tick{}
	for _, e := range list {
		d := e.File.Data
		if d == nil {
			continue
		}
		p := d.Series[id]
		if p == nil {
			continue
		}
		for i, t := range p.T {
			if t >= since && i < len(p.V) {
				out = append(out, Tick{t, p.V[i]})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// LastTick — время последнего сохранённого слота набора (0 — снимков ещё не было). Читает только последний файл месяца.
func (s Store) LastTick(set string) int64 {
	list, _ := s.ListEconomy(set)
	var last int64
	for _, e := range list {
		if d := e.File.Data; d != nil {
			for _, p := range d.Series {
				if n := len(p.T); n > 0 && p.T[n-1] > last {
					last = p.T[n-1]
				}
			}
		}
	}
	return last
}
