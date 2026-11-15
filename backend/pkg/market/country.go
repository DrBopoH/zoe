package market

// Страна в каталоге: код (ISO-2 в нижнем регистре, «ez» — еврозона), имя, валюта и какой ряд отвечает за каждый показатель карточки.
// Новая страна = одна запись здесь + её источник в Feeds; фронт, каталог и тесты ничего больше не требуют.
// Показатели: policy — ключевая ставка, cpi — инфляция, cpi_core — базовая, u — безработица, gdp — рост ВВП, y10 — доходность 10-летних,
// fx — курс валюты к гривне (если есть у НБУ).
type Country struct {
	CC       string            `json:"cc"`
	Name     string            `json:"name"`
	Currency string            `json:"currency"`
	Metrics  map[string]string `json:"metrics"`
}

// Countries — страны набора в порядке показа. Показатели — id рядов, которые уже отдаёт Catalog.
func Countries() []Country {
	return []Country{
		{CC: "us", Name: "США", Currency: "USD", Metrics: map[string]string{
			"policy": SerFedFunds, "cpi": SerUSCPIYoY, "cpi_core": SerUSCoreCPIYoY, "u": SerUSUnrate, "gdp": SerUSGDP, "y10": SerUST10Y, "fx": SerUSDUAH}},
		{CC: "ua", Name: "Украина", Currency: "UAH", Metrics: map[string]string{
			"policy": SerPolicyRate, "cpi": SerCPIYoY, "cpi_core": SerCoreYoY}},
		{CC: "ez", Name: "Еврозона", Currency: "EUR", Metrics: map[string]string{
			"policy": SerEZPolicy, "cpi": SerEUCPIYoY, "cpi_core": SerEZCPICore, "u": SerEZUnrate, "gdp": SerEZGDP, "y10": SerEZY10, "fx": SerEURUAH}},
		{CC: "de", Name: "Германия", Currency: "EUR", Metrics: ezMember("de")},
		{CC: "fr", Name: "Франция", Currency: "EUR", Metrics: ezMember("fr")},
		{CC: "it", Name: "Италия", Currency: "EUR", Metrics: ezMember("it")},
		{CC: "es", Name: "Испания", Currency: "EUR", Metrics: ezMember("es")},
		{CC: "nl", Name: "Нидерланды", Currency: "EUR", Metrics: ezMember("nl")},
	}
}

// ezMember — показатели страны-члена еврозоны: ставка общая (ЕЦБ), остальное — её собственные ряды из Евростата.
func ezMember(cc string) map[string]string {
	return map[string]string{"policy": SerEZPolicy, "cpi": memberID(cc, "cpi"), "u": memberID(cc, "u"), "y10": memberID(cc, "y10"), "fx": SerEURUAH}
}
