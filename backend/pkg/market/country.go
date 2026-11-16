package market

import "sort"

// Страна в каталоге: код (ISO-2 в нижнем регистре, «ez» — еврозона), имя, валюта и какой ряд отвечает за каждый показатель карточки.
// Страны еврозоны по отдельности сюда не входят: еврозона — одна карточка «ez», сравнение стран-членов лежит внутри раздела «Еврозона» (ezm.go).
// Новая страна = одна запись здесь + её источник в Feeds; фронт, каталог и тесты ничего больше не требуют.
// Показатели: policy — ключевая ставка, cpi — инфляция, cpi_core — базовая, u — безработица, gdp — рост ВВП, y10 — доходность 10-летних,
// fx — курс валюты к гривне (если есть у НБУ), reer — реальный эффективный курс, stock — индекс акций страны (2015 = 100), если его опрашивает источник страны.
type Country struct {
	CC       string            `json:"cc"`
	Name     string            `json:"name"`
	Currency string            `json:"currency"`
	Source   string            `json:"source"` // источник, который опрашивает «Обновить» в карточке страны
	Metrics  map[string]string `json:"metrics"`
	// Sources — все источники, из которых собрана карточка: свой первым, затем те, что дают остальные показатели (курс, реальный курс, безработица…).
	// «Обновить» в карточке опрашивает их все. Заполняет WithSources по каталогу рядов, руками не ведётся.
	Sources []string `json:"sources"`
}

// Countries — страны набора в порядке показа. Показатели — id рядов, которые уже отдаёт Catalog.
func Countries() []Country {
	return []Country{
		{CC: "us", Name: "США", Currency: "USD", Source: SourceUS, Metrics: map[string]string{
			"policy": SerFedFunds, "cpi": SerUSCPIYoY, "cpi_core": SerUSCoreCPIYoY, "u": SerUSUnrate, "gdp": SerUSGDP, "y10": SerUST10Y, "fx": SerUSDUAH, "reer": SerREER("us")}},
		{CC: "ua", Name: "Украина", Currency: "UAH", Source: SourceMacro, Metrics: map[string]string{
			"policy": SerPolicyRate, "cpi": SerCPIYoY, "cpi_core": SerCoreYoY, "ppi": SerUAPPI, "reserves": SerUAReserves, "m3": SerUAM3YoY, "ca": SerUACA}},
		{CC: "ez", Name: "Еврозона", Currency: "EUR", Source: SourceEZ, Metrics: map[string]string{
			"policy": SerEZPolicy, "cpi": SerEUCPIYoY, "cpi_core": SerEZCPICore, "u": SerEZUnrate, "gdp": SerEZGDP, "y10": SerEZY10, "fx": SerEURUAH, "reer": SerREER("ez")}},
		{CC: "gb", Name: "Великобритания", Currency: "GBP", Source: SourceGB, Metrics: map[string]string{
			"policy": SerGBPolicy, "cpi": SerGBCPI, "u": SerGBUnrate, "gdp": SerGBGDP, "y10": SerGBY10, "fx": SerGBPUAH, "reer": SerREER("gb")}},
		{CC: "ch", Name: "Швейцария", Currency: "CHF", Source: SourceCH, Metrics: map[string]string{
			"policy": SerCHPolicy, "cpi": SerCHCPI, "u": SerCHUnrate, "y10": SerCHY10, "fx": SerCHFUAH, "reer": SerREER("ch")}},
		{CC: "jp", Name: "Япония", Currency: "JPY", Source: SourceJP, Metrics: map[string]string{
			"policy": SerJPPolicy, "cpi": SerJPCPI, "u": SerJPUnrate, "y10": SerJPY10, "fx": SerJPYUAH, "reer": SerREER("jp")}},
		{CC: "cn", Name: "Китай", Currency: "CNY", Source: SourceCN, Metrics: map[string]string{
			"policy": SerCNPolicy, "cpi": SerCNCPI, "fx": SerCNYUAH, "reer": SerREER("cn")}},
		{CC: "ru", Name: "Россия", Currency: "RUB", Source: SourceRU, Metrics: map[string]string{
			"policy": SerRUPolicy, "cpi": SerRUCPI, "fx": SerRUBUAH, "reer": SerREER("ru")}},
		{CC: "eg", Name: "Египет", Currency: "EGP", Source: SourceEG, Metrics: map[string]string{
			"cpi": SerEGCPI, "fx": SerEGPUAH}},
		{CC: "pl", Name: "Польша", Currency: "PLN", Source: SourcePL, Metrics: map[string]string{
			"policy": SerPLPolicy, "cpi": SerPLCPI, "u": SerPLUnrate, "y10": SerPLY10, "fx": SerPLNUAH, "reer": SerREER("pl")}},
		{CC: "tr", Name: "Турция", Currency: "TRY", Source: SourceTR, Metrics: map[string]string{
			"policy": SerTRPolicy, "cpi": SerTRCPI, "u": SerTRUnrate, "fx": SerTRYUAH, "reer": SerREER("tr")}},
		{CC: "no", Name: "Норвегия", Currency: "NOK", Source: SourceNO, Metrics: map[string]string{
			"policy": SerNOPolicy, "cpi": SerNOCPI, "u": SerNOUnrate, "y10": SerNOY10, "fx": SerNOKUAH, "reer": SerREER("no")}},
		{CC: "se", Name: "Швеция", Currency: "SEK", Source: SourceSE, Metrics: map[string]string{
			"policy": SerSEPolicy, "cpi": SerSECPI, "u": SerSEUnrate, "y10": SerSEY10, "fx": SerSEKUAH, "reer": SerREER("se")}},
		{CC: "ca", Name: "Канада", Currency: "CAD", Source: SourceCA, Metrics: map[string]string{
			"policy": SerCAPolicy, "cpi": SerCACPI, "u": SerCAUnrate, "y10": SerCAY10, "fx": SerCADUAH, "reer": SerREER("ca")}},
		{CC: "kr", Name: "Южная Корея", Currency: "KRW", Source: SourceKR, Metrics: map[string]string{
			"policy": SerKRPolicy, "cpi": SerKRCPI, "reer": SerREER("kr"), "stock": SerWM("kr")}},
		{CC: "tw", Name: "Тайвань", Currency: "TWD", Source: SourceTW, Metrics: map[string]string{
			"reer": SerREER("tw"), "stock": SerWM("tw")}},
	}
}

// WithSources проставляет каждой стране источники её показателей по каталогу рядов (Client.Catalog: источник → его ряды).
// Показатель, которого нет в каталоге, пропускается; свой источник страны идёт первым и всегда есть.
func WithSources(list []Country, cat map[string][]string) []Country {
	owner := map[string]string{}
	for feed, ids := range cat {
		for _, id := range ids {
			owner[id] = feed
		}
	}
	out := make([]Country, len(list))
	for i, c := range list {
		keys := make([]string, 0, len(c.Metrics))
		for k := range c.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		c.Sources = []string{c.Source}
		seen := map[string]bool{c.Source: true}
		for _, k := range keys {
			if f := owner[c.Metrics[k]]; f != "" && !seen[f] {
				seen[f] = true
				c.Sources = append(c.Sources, f)
			}
		}
		out[i] = c
	}
	return out
}
