package market

// Страна в каталоге: код (ISO-2 в нижнем регистре, «ez» — еврозона), имя, валюта и какой ряд отвечает за каждый показатель карточки.
// Страны еврозоны по отдельности сюда не входят: еврозона — одна карточка «ez», сравнение стран-членов лежит внутри раздела «Еврозона» (ezm.go).
// Новая страна = одна запись здесь + её источник в Feeds; фронт, каталог и тесты ничего больше не требуют.
// Показатели: policy — ключевая ставка, cpi — инфляция, cpi_core — базовая, u — безработица, gdp — рост ВВП, y10 — доходность 10-летних,
// fx — курс валюты к гривне (если есть у НБУ).
type Country struct {
	CC       string            `json:"cc"`
	Name     string            `json:"name"`
	Currency string            `json:"currency"`
	Source   string            `json:"source"` // источник, который опрашивает «Обновить» в карточке страны
	Metrics  map[string]string `json:"metrics"`
}

// Countries — страны набора в порядке показа. Показатели — id рядов, которые уже отдаёт Catalog.
func Countries() []Country {
	return []Country{
		{CC: "us", Name: "США", Currency: "USD", Source: SourceUS, Metrics: map[string]string{
			"policy": SerFedFunds, "cpi": SerUSCPIYoY, "cpi_core": SerUSCoreCPIYoY, "u": SerUSUnrate, "gdp": SerUSGDP, "y10": SerUST10Y, "fx": SerUSDUAH}},
		{CC: "ua", Name: "Украина", Currency: "UAH", Source: SourceMacro, Metrics: map[string]string{
			"policy": SerPolicyRate, "cpi": SerCPIYoY, "cpi_core": SerCoreYoY}},
		{CC: "ez", Name: "Еврозона", Currency: "EUR", Source: SourceEZ, Metrics: map[string]string{
			"policy": SerEZPolicy, "cpi": SerEUCPIYoY, "cpi_core": SerEZCPICore, "u": SerEZUnrate, "gdp": SerEZGDP, "y10": SerEZY10, "fx": SerEURUAH}},
		{CC: "gb", Name: "Великобритания", Currency: "GBP", Source: SourceGB, Metrics: map[string]string{
			"policy": SerGBPolicy, "cpi": SerGBCPI, "u": SerGBUnrate, "gdp": SerGBGDP, "y10": SerGBY10, "fx": SerGBPUAH}},
		{CC: "ch", Name: "Швейцария", Currency: "CHF", Source: SourceCH, Metrics: map[string]string{
			"policy": SerCHPolicy, "cpi": SerCHCPI, "u": SerCHUnrate, "y10": SerCHY10, "fx": SerCHFUAH}},
		{CC: "jp", Name: "Япония", Currency: "JPY", Source: SourceJP, Metrics: map[string]string{
			"policy": SerJPPolicy, "cpi": SerJPCPI, "u": SerJPUnrate, "y10": SerJPY10, "fx": SerJPYUAH}},
		{CC: "cn", Name: "Китай", Currency: "CNY", Source: SourceCN, Metrics: map[string]string{
			"policy": SerCNPolicy, "cpi": SerCNCPI, "fx": SerCNYUAH}},
		{CC: "ru", Name: "Россия", Currency: "RUB", Source: SourceRU, Metrics: map[string]string{
			"policy": SerRUPolicy, "cpi": SerRUCPI, "fx": SerRUBUAH}},
		{CC: "eg", Name: "Египет", Currency: "EGP", Source: SourceEG, Metrics: map[string]string{
			"policy": SerEGPolicy, "cpi": SerEGCPI, "fx": SerEGPUAH}},
	}
}
