package market

import (
	"context"
	"fmt"
	"strings"
)

// ovdpPrefix — ISIN государственных облигаций Украины начинаются с UA4000. В реестре депозитария лежат и корпоративные,
// и муниципальные бумаги; ОВДП отбираются по этому префиксу.
const ovdpPrefix = "UA4000"

// auctionsFrom — с какого процента начинаются аукционы: реестр (один запрос) занимает первую часть шкалы.
const auctionsFrom = 15

// NBURegistry — реестр ОВДП из справочника ценных бумаг депозитария (bank.gov.ua/depo_securities?json).
// Поля по документации НБУ: cpcode (ISIN), cpdescr (описание), val_code, nominal, auk_proc (ставка, %),
// razm_date (размещение), pgs_date (погашение).
func (c *Client) NBURegistry(ctx context.Context) ([]Security, Result, error) {
	res := Result{Source: SourceNBU}
	body, err := c.get(ctx, strings.TrimRight(c.NBUBase, "/")+"/depo_securities?json")
	if err != nil {
		return nil, res, fmt.Errorf("реестр ОВДП: %w", err)
	}
	recs, err := records(body)
	if err != nil {
		return nil, res, fmt.Errorf("реестр ОВДП: %w", err)
	}
	var out []Security
	for _, r := range recs {
		id := strings.ToUpper(r.str("cpcode", "isin", "stockcode"))
		if !strings.HasPrefix(id, ovdpPrefix) {
			res.Skipped++
			continue
		}
		s := Security{
			Source:     SourceNBU,
			ID:         id,
			Name:       r.str("cpdescr", "name", "cpname"),
			Currency:   upperCur(r.str("val_code", "valcode", "currency")),
			Maturity:   r.date("pgs_date", "repaydate", "maturity"),
			Issued:     r.date("razm_date", "issuedate", "placement_date"),
			CouponRate: r.num("auk_proc", "coupon_rate", "rate"),
			Nominal:    r.num("nominal"),
		}
		if s.Nominal <= 0 {
			s.Nominal = 1000
		}
		out = append(out, s)
	}
	res.Fetched = len(out)
	return out, res, nil
}

// NBUAuctions — результаты аукционов Минфина за последние дни (bank.gov.ua/NBU_ovdp?json&date=DD.MM.YYYY).
// Формат даты именно DD.MM.YYYY. День без аукциона даёт пустой массив, не ошибку. Берётся доходность и дата аукциона
// по каждой бумаге; побеждает самый поздний аукцион.
func (c *Client) NBUAuctions(ctx context.Context, rep Reporter) ([]Security, Result, error) {
	res := Result{Source: SourceNBU}
	days := c.AuctionDays
	if days <= 0 {
		days = 14
	}
	byID := map[string]Security{}
	var firstErr error
	ok := 0
	for i := days - 1; i >= 0; i-- { // от старых к новым: новый аукцион перетирает старый
		if err := ctx.Err(); err != nil {
			return nil, res, err
		}
		day := c.now().AddDate(0, 0, -i)
		rep.report(auctionsFrom+(100-auctionsFrom)*(days-1-i)/days, "аукционы: "+day.Format("02.01"))
		body, err := c.get(ctx, strings.TrimRight(c.NBUBase, "/")+"/NBU_ovdp?json&date="+day.Format("02.01.2006"))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		recs, err := records(body)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ok++
		for _, r := range recs {
			id := strings.ToUpper(r.str("stockcode", "cpcode", "isin"))
			if !strings.HasPrefix(id, ovdpPrefix) {
				res.Skipped++
				continue
			}
			s := byID[id]
			s.Source, s.ID = SourceNBU, id
			s.Currency = upperCur(r.str("valcode", "val_code", "currency"))
			if m := r.date("repaydate", "pgs_date", "maturity"); m != "" {
				s.Maturity = m
			}
			if y := r.num("auc_proc", "auk_proc", "yield", "rate"); y > 0 {
				s.Yield = y
			}
			if d := r.date("auctiondate", "auction_date"); d != "" {
				s.LastAuction = d
			} else {
				s.LastAuction = day.Format("2006-01-02")
			}
			byID[id] = s
		}
	}
	if ok == 0 && firstErr != nil {
		return nil, res, fmt.Errorf("аукционы ОВДП: %w", firstErr)
	}
	out := make([]Security, 0, len(byID))
	for _, s := range byID {
		out = append(out, s)
	}
	res.Fetched = len(out)
	return out, res, nil
}

// SyncNBU — реестр + аукционы одним списком: условия из реестра, доходность и дата аукциона из результатов.
// Если упал реестр, но аукционы пришли (или наоборот) — отдаёт, что есть, и ошибку в Result.
func (c *Client) SyncNBU(ctx context.Context, rep Reporter) ([]Security, Result) {
	rep.report(0, "реестр")
	reg, rres, rerr := c.NBURegistry(ctx)
	auc, ares, aerr := c.NBUAuctions(ctx, rep)
	res := Result{Source: SourceNBU, Skipped: rres.Skipped + ares.Skipped}
	byID := map[string]*Security{}
	var order []string
	for i := range reg {
		s := reg[i]
		byID[s.ID] = &s
		order = append(order, s.ID)
	}
	for _, a := range auc {
		s, found := byID[a.ID]
		if !found {
			cp := a
			if cp.Nominal <= 0 {
				cp.Nominal = 1000
			}
			byID[a.ID] = &cp
			order = append(order, a.ID)
			continue
		}
		s.Yield, s.LastAuction = a.Yield, a.LastAuction
		if s.Maturity == "" {
			s.Maturity = a.Maturity
		}
	}
	out := make([]Security, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	res.Fetched = len(out)
	var errs []string
	for _, e := range []error{rerr, aerr} {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}
	res.Error = strings.Join(errs, "; ")
	return out, res
}
