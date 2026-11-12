package market

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// USTAuctions — аукционы казначейства США (FiscalData, v1/accounting/od/auctions_query), новые сверху.
// Одна бумага (CUSIP) может размещаться несколько раз (re-opening) — остаётся самый поздний аукцион.
// Поля: cusip, security_type (Bill/Note/Bond/TIPS/FRN), security_term, auction_date, issue_date, maturity_date,
// int_rate (купон, %), high_yield / high_investment_rate / avg_med_yield (доходность). Номинал минимального лота — 100 $.
func (c *Client) USTAuctions(ctx context.Context) ([]Security, Result, error) {
	res := Result{Source: SourceUST}
	years := c.USTYears
	if years <= 0 {
		years = 3
	}
	since := c.now().AddDate(-years, 0, 0).Format("2006-01-02")
	q := url.Values{
		"filter":       {"auction_date:gte:" + since},
		"sort":         {"-auction_date"},
		"page[size]":   {"10000"},
		"page[number]": {"1"},
		"format":       {"json"},
	}
	body, err := c.get(ctx, strings.TrimRight(c.USTBase, "/")+"/v1/accounting/od/auctions_query?"+q.Encode())
	if err != nil {
		return nil, res, fmt.Errorf("аукционы US Treasury: %w", err)
	}
	recs, err := records(body)
	if err != nil {
		return nil, res, fmt.Errorf("аукционы US Treasury: %w", err)
	}
	seen := map[string]bool{}
	var out []Security
	for _, r := range recs {
		id := strings.ToUpper(r.str("cusip"))
		if id == "" {
			res.Skipped++
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		kind, term := r.str("security_type"), r.str("security_term")
		s := Security{
			Source:      SourceUST,
			ID:          id,
			Name:        strings.TrimSpace(term + " " + kind),
			Currency:    "USD",
			Maturity:    r.date("maturity_date"),
			Issued:      r.date("issue_date"),
			CouponRate:  r.num("int_rate"),
			Nominal:     100,
			Yield:       firstPositive(r.num("high_yield"), r.num("high_investment_rate"), r.num("avg_med_yield"), r.num("high_discnt_rate")),
			LastAuction: r.date("auction_date"),
		}
		out = append(out, s)
	}
	res.Fetched = len(out)
	return out, res, nil
}

func firstPositive(vs ...float64) float64 {
	for _, v := range vs {
		if v > 0 {
			return v
		}
	}
	return 0
}
