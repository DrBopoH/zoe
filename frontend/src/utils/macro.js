import { scaleTicks, timeTicks } from './growthChart.js'
import { robustRanges, scaleOf } from './chartView.js'
import { dmy, numText } from './numfmt.js'

// Макро НБУ: ключевые ставки и инфляция (ряды из /api/macro). Чистые функции — без интерфейса.
// Ставки приходят датами смены значения (значение действует до следующей точки), инфляция — помесячно.

export const SERIES = {
  policy_rate: { label: 'Облікова ставка', color: '#34d399', step: true },
  cpi_yoy: { label: 'Инфляция г/г', color: '#fbbf24', step: false },
  core_yoy: { label: 'Базовая инфляция г/г', color: '#fb923c', step: false, thin: true },
  overnight_cd: { label: 'Сертификаты овернайт', color: '#38bdf8', step: true },
  overnight_loans: { label: 'Кредиты овернайт', color: '#a78bfa', step: true },
  cpi_mom: { label: 'Инфляция м/м', color: '#fbbf24', step: false },
  core_mom: { label: 'Базовая м/м', color: '#fb923c', step: false, thin: true },
  usd_uah: { label: 'Доллар США', color: '#34d399', step: false },
  eur_uah: { label: 'Евро', color: '#38bdf8', step: false },
  xau_uah: { label: 'Золото, ₴ за унцию', color: '#fbbf24', step: false },
  xau_usd: { label: 'Золото, $ за унцию', color: '#fbbf24', step: false },
  fed_funds: { label: 'Ставка ФРС (факт)', color: '#a78bfa', step: false },
  fed_target: { label: 'ФРС: верх коридора', color: '#34d399', step: true },
  ust_10y: { label: 'Treasury 10 л.', color: '#38bdf8', step: false, thin: true },
  eu_cpi_yoy: { label: 'Инфляция еврозоны г/г', color: '#38bdf8', step: false },
  us_cpi_yoy: { label: 'Инфляция США г/г', color: '#fbbf24', step: false },
  us_cpi_mom: { label: 'Инфляция США м/м', color: '#fb923c', step: false },
  us_unrate: { label: 'Безработица США', color: '#38bdf8', step: false },
  us_gdp: { label: 'Рост ВВП США (в год)', color: '#34d399', step: true },
  us_spread: { label: 'Кривая: 10 л. − 2 г.', color: '#a78bfa', step: false },
  // как у индекса страха и жадности: середина (около 20, обычный фон) полупрозрачный серый, высокий VIX = страх голубой, очень низкий = самоуспокоенность янтарная
  us_vix: { label: 'VIX (индекс страха)', color: '#94a3b8', step: false, gradient: { from: 10, to: 50, stops: [[0, '#fbbf24', 1], [0.25, '#94a3b8', 0.4], [0.75, '#38bdf8', 1], [1, '#38bdf8', 1]] } },
  // светофор: чем оптимистичнее люди, тем зеленее (от 50 красный, около 75 жёлтый, от 100 зелёный)
  us_sentiment: { label: 'Настроения потребителей', color: '#fbbf24', step: false, noLog: true, gradient: { from: 50, to: 100, stops: [[0, '#f87171', 1], [0.5, '#fbbf24', 1], [1, '#34d399', 1]] } },
  ust_1m: { label: 'Treasury 1 мес.', color: '#64748b', step: false, thin: true },
  ust_3m: { label: 'Treasury 3 мес.', color: '#fbbf24', step: false },
  ust_6m: { label: 'Treasury 6 мес.', color: '#fb923c', step: false, thin: true },
  ust_1y: { label: 'Treasury 1 г.', color: '#f472b6', step: false, thin: true },
  ust_2y: { label: 'Treasury 2 г.', color: '#a78bfa', step: false },
  ust_3y: { label: 'Treasury 3 г.', color: '#818cf8', step: false, thin: true },
  ust_5y: { label: 'Treasury 5 л.', color: '#2dd4bf', step: false, thin: true },
  ust_7y: { label: 'Treasury 7 л.', color: '#4ade80', step: false, thin: true },
  ust_20y: { label: 'Treasury 20 л.', color: '#f87171', step: false, thin: true },
  ust_30y: { label: 'Treasury 30 л.', color: '#34d399', step: false },
  us_spread3m: { label: 'Кривая: 10 л. − 3 мес.', color: '#a78bfa', step: false },
  us_real5: { label: 'Реальная доходность 5 л. (TIPS)', color: '#2dd4bf', step: false, thin: true },
  us_real10: { label: 'Реальная доходность 10 л. (TIPS)', color: '#38bdf8', step: false },
  us_be5: { label: 'Брейкивен 5 л.', color: '#fb923c', step: false, thin: true },
  us_be10: { label: 'Брейкивен 10 л.', color: '#fbbf24', step: false },
  us_fwd5y5y: { label: 'Инфляция 5y5y', color: '#a78bfa', step: false },
  us_termprem: { label: 'Срочная премия 10 л.', color: '#f472b6', step: false },
  us_mortgage: { label: 'Ипотека 30 л.', color: '#34d399', step: false },
  us_corecpi_yoy: { label: 'Базовый CPI г/г', color: '#fb923c', step: false },
  us_corecpi_mom: { label: 'Базовый CPI м/м', color: '#fb923c', step: false },
  us_pce_yoy: { label: 'PCE г/г', color: '#a78bfa', step: false, thin: true },
  us_corepce_yoy: { label: 'Базовый PCE г/г', color: '#34d399', step: false },
  us_ppi_yoy: { label: 'Цены производителей г/г', color: '#f472b6', step: false },
  us_shelter_yoy: { label: 'Жильё в CPI г/г', color: '#38bdf8', step: false },
  us_nfp: { label: 'Прирост занятых за месяц', color: '#34d399', step: false, noLog: true },
  us_claims: { label: 'Заявки на пособие (неделя)', color: '#fbbf24', step: false },
  us_jolts: { label: 'Открытые вакансии', color: '#38bdf8', step: false },
  us_wages_yoy: { label: 'Зарплаты г/г', color: '#34d399', step: false },
  us_u6: { label: 'Безработица U-6', color: '#fb923c', step: false },
  us_eci_yoy: { label: 'Стоимость рабочей силы г/г', color: '#a78bfa', step: false, thin: true },
  us_retail_yoy: { label: 'Розничные продажи г/г', color: '#38bdf8', step: false },
  us_indpro_yoy: { label: 'Промышленность г/г', color: '#fbbf24', step: false },
  us_housing: { label: 'Начато строительство жилья', color: '#a78bfa', step: false },
  us_homeprice_yoy: { label: 'Цены на дома г/г', color: '#34d399', step: false },
  us_m2: { label: 'Денежная масса M2', color: '#38bdf8', step: false },
  us_m2_yoy: { label: 'M2 г/г', color: '#38bdf8', step: false },
  us_fedbs: { label: 'Баланс ФРС', color: '#a78bfa', step: false },
  us_rrp: { label: 'Обратное РЕПО', color: '#fbbf24', step: false, thin: true },
  us_tga: { label: 'Счёт Казначейства', color: '#f472b6', step: false, thin: true },
  us_netliq: { label: 'Чистая ликвидность', color: '#34d399', step: false },
  ez_policy: { label: 'ЕЦБ: ставка по депозитам', color: '#a78bfa', step: true },
  ez_mro: { label: 'ЕЦБ: основная ставка', color: '#34d399', step: true },
  ez_estr: { label: '€STR (овернайт)', color: '#38bdf8', step: false, thin: true },
  ez_y2: { label: 'Еврозона 2 г. (AAA)', color: '#fbbf24', step: false, thin: true },
  ez_y10: { label: 'Еврозона 10 л. (AAA)', color: '#38bdf8', step: false },
  ez_cpi_core: { label: 'Базовая инфляция еврозоны г/г', color: '#fb923c', step: false, thin: true },
  ez_u: { label: 'Безработица еврозоны', color: '#38bdf8', step: false },
  ez_gdp: { label: 'Рост ВВП еврозоны (за квартал)', color: '#34d399', step: true },
  de_cpi: { label: 'Германия: инфляция г/г', color: '#fbbf24', step: false },
  de_u: { label: 'Германия: безработица', color: '#fbbf24', step: false },
  de_y10: { label: 'Германия: 10 лет', color: '#fbbf24', step: false },
  fr_cpi: { label: 'Франция: инфляция г/г', color: '#38bdf8', step: false },
  fr_u: { label: 'Франция: безработица', color: '#38bdf8', step: false },
  fr_y10: { label: 'Франция: 10 лет', color: '#38bdf8', step: false },
  it_cpi: { label: 'Италия: инфляция г/г', color: '#34d399', step: false },
  it_u: { label: 'Италия: безработица', color: '#34d399', step: false },
  it_y10: { label: 'Италия: 10 лет', color: '#34d399', step: false },
  es_cpi: { label: 'Испания: инфляция г/г', color: '#fb923c', step: false },
  es_u: { label: 'Испания: безработица', color: '#fb923c', step: false },
  es_y10: { label: 'Испания: 10 лет', color: '#fb923c', step: false },
  nl_cpi: { label: 'Нидерланды: инфляция г/г', color: '#a78bfa', step: false },
  nl_u: { label: 'Нидерланды: безработица', color: '#a78bfa', step: false },
  nl_y10: { label: 'Нидерланды: 10 лет', color: '#a78bfa', step: false },
  it_bund_spread: { label: 'Италия − Германия, 10 лет', color: '#34d399', step: false },
  fr_bund_spread: { label: 'Франция − Германия, 10 лет', color: '#38bdf8', step: false },
  es_bund_spread: { label: 'Испания − Германия, 10 лет', color: '#fb923c', step: false },
  gb_policy: { label: 'Банк Англии: ставка', color: '#a78bfa', step: true },
  gb_sonia: { label: 'SONIA (овернайт)', color: '#38bdf8', step: false, thin: true },
  gb_y10: { label: 'Gilt 10 лет', color: '#fbbf24', step: false },
  gb_cpi: { label: 'Инфляция Великобритании г/г', color: '#fb923c', step: false },
  gb_u: { label: 'Безработица Великобритании', color: '#38bdf8', step: false },
  gb_gdp: { label: 'Рост ВВП Великобритании (за квартал)', color: '#34d399', step: true },
  ch_policy: { label: 'SNB: ставка', color: '#a78bfa', step: true },
  ch_cpi: { label: 'Инфляция Швейцарии г/г', color: '#fb923c', step: false },
  ch_u: { label: 'Безработица Швейцарии', color: '#38bdf8', step: false },
  gbp_uah: { label: 'Фунт стерлингов', color: '#a78bfa', step: false },
  chf_uah: { label: 'Швейцарский франк', color: '#f87171', step: false },
  ch_y10: { label: 'Швейцария: 10 лет', color: '#fbbf24', step: false },
  jp_policy: { label: 'Банк Японии: ставка', color: '#a78bfa', step: true },
  jp_cpi: { label: 'Инфляция Японии г/г', color: '#fb923c', step: false },
  jp_u: { label: 'Безработица Японии', color: '#38bdf8', step: false },
  jp_y10: { label: 'JGB 10 лет', color: '#fbbf24', step: false },
  jp_nikkei: { label: 'Nikkei 225', color: '#34d399', step: false },
  cn_policy: { label: 'Народный банк Китая: ставка', color: '#a78bfa', step: true },
  cn_cpi: { label: 'Инфляция Китая г/г', color: '#fb923c', step: false },
  jpy_uah: { label: 'Иена', color: '#f472b6', step: false },
  cny_uah: { label: 'Юань', color: '#f87171', step: false },
  ru_policy: { label: 'Банк России: ключевая ставка', color: '#a78bfa', step: true },
  ru_cpi: { label: 'Инфляция России г/г', color: '#fb923c', step: false },
  eg_cpi: { label: 'Инфляция Египта г/г', color: '#fb923c', step: false },
  rub_uah: { label: 'Российский рубль', color: '#f87171', step: false },
  egp_uah: { label: 'Египетский фунт', color: '#fbbf24', step: false },
  reer_us: { label: 'Реальный курс США', color: '#38bdf8', step: false },
  reer_ez: { label: 'Реальный курс Еврозоны', color: '#34d399', step: false },
  reer_gb: { label: 'Реальный курс Великобритании', color: '#a78bfa', step: false },
  reer_ch: { label: 'Реальный курс Швейцарии', color: '#f87171', step: false },
  reer_jp: { label: 'Реальный курс Японии', color: '#f472b6', step: false },
  reer_cn: { label: 'Реальный курс Китая', color: '#fb923c', step: false },
  reer_ru: { label: 'Реальный курс России', color: '#fbbf24', step: false },
  reer_pl: { label: 'Реальный курс Польши', color: '#2dd4bf', step: false },
  reer_tr: { label: 'Реальный курс Турции', color: '#818cf8', step: false },
  reer_no: { label: 'Реальный курс Норвегии', color: '#4ade80', step: false },
  reer_se: { label: 'Реальный курс Швеции', color: '#94a3b8', step: false },
  reer_ca: { label: 'Реальный курс Канады', color: '#64748b', step: false },
  reer_kr: { label: 'Реальный курс Южной Кореи', color: '#22d3ee', step: false },
  reer_tw: { label: 'Реальный курс Тайваня', color: '#e879f9', step: false },
  reer_au: { label: 'Реальный курс Австралии', color: '#facc15', step: false },
  reer_br: { label: 'Реальный курс Бразилии', color: '#4ade80', step: false },
  met_xau: { label: 'Золото', color: '#fbbf24', step: false },
  met_xag: { label: 'Серебро', color: '#cbd5e1', step: false },
  met_xpt: { label: 'Платина', color: '#38bdf8', step: false },
  met_xpd: { label: 'Палладий', color: '#a78bfa', step: false },
  com_brent: { label: 'Нефть Brent', color: '#fb923c', step: false },
  com_wti: { label: 'Нефть WTI', color: '#f87171', step: false },
  com_gas_us: { label: 'Газ США (Henry Hub)', color: '#38bdf8', step: false },
  com_gas_eu: { label: 'Газ Европа (TTF)', color: '#a78bfa', step: false },
  com_copper: { label: 'Медь', color: '#fb923c', step: false },
  com_alum: { label: 'Алюминий', color: '#cbd5e1', step: false },
  com_nickel: { label: 'Никель', color: '#34d399', step: false },
  com_wheat: { label: 'Пшеница', color: '#fbbf24', step: false },
  com_corn: { label: 'Кукуруза', color: '#a3e635', step: false },
  com_rice: { label: 'Рис', color: '#e2e8f0', step: false },
  com_soy: { label: 'Соя', color: '#34d399', step: false },
  com_cocoa: { label: 'Какао', color: '#b45309', step: false },
  com_sugar: { label: 'Сахар', color: '#f472b6', step: false },
  com_coffee: { label: 'Кофе (арабика)', color: '#a16207', step: false },
  xag_uah: { label: 'Серебро, ₴ за унцию', color: '#cbd5e1', step: false },
  xpt_uah: { label: 'Платина, ₴ за унцию', color: '#38bdf8', step: false },
  xpd_uah: { label: 'Палладий, ₴ за унцию', color: '#a78bfa', step: false },
  wm_us: { label: 'Акции: США', color: '#38bdf8', step: false },
  wm_jp: { label: 'Акции: Япония', color: '#f87171', step: false },
  wm_gb: { label: 'Акции: Великобритания', color: '#a78bfa', step: false },
  wm_de: { label: 'Акции: Германия', color: '#fbbf24', step: false },
  wm_fr: { label: 'Акции: Франция', color: '#34d399', step: false },
  wm_ch: { label: 'Акции: Швейцария', color: '#e2e8f0', step: false },
  wm_ca: { label: 'Акции: Канада', color: '#fb923c', step: false },
  wm_se: { label: 'Акции: Швеция', color: '#38bdf8', step: false },
  wm_no: { label: 'Акции: Норвегия', color: '#fb923c', step: false },
  wm_pl: { label: 'Акции: Польша', color: '#f472b6', step: false },
  wm_cn: { label: 'Акции: Китай', color: '#f87171', step: false },
  wm_kr: { label: 'Акции: Южная Корея (KOSPI)', color: '#22d3ee', step: false },
  wm_tw: { label: 'Акции: Тайвань (TAIEX)', color: '#e879f9', step: false },
  wm_au: { label: 'Акции: Австралия (ASX 200)', color: '#facc15', step: false },
  wm_br: { label: 'Акции: Бразилия (Bovespa)', color: '#4ade80', step: false },
  wm_tr: { label: 'Акции: Турция (в лирах)', color: '#fbbf24', step: false },
  val_buffett: { label: 'Индекс Баффета: акции к ВВП США', color: '#fbbf24', step: false },
  val_cape: { label: 'CAPE S&P 500', color: '#a78bfa', step: false },
  val_divy: { label: 'Дивидендная доходность S&P 500', color: '#34d399', step: false },
  p2p_ask: { label: 'P2P: купить USDT, ₴', color: '#f87171', step: false },
  p2p_bid: { label: 'P2P: продать USDT, ₴', color: '#34d399', step: false },
  p2p_spread: { label: 'P2P: спред покупки и продажи, %', color: '#fbbf24', step: false },
  p2p_prem_ask: { label: 'Покупка USDT дороже курса НБУ, %', color: '#f87171', step: false },
  p2p_prem_bid: { label: 'Продажа USDT к курсу НБУ, %', color: '#34d399', step: false },
  fund_btc: { label: 'Фандинг BTC, % годовых', color: '#fb923c', step: false },
  fund_eth: { label: 'Фандинг ETH, % годовых', color: '#a78bfa', step: false },
  oi_btc: { label: 'Открытый интерес BTC, $ млрд', color: '#fb923c', step: false },
  oi_eth: { label: 'Открытый интерес ETH, $ млрд', color: '#a78bfa', step: false },
  stable_usdt: { label: 'USDT в обращении, $ млрд', color: '#34d399', step: false },
  stable_usdc: { label: 'USDC в обращении, $ млрд', color: '#38bdf8', step: false },
  fee_btc_sat: { label: 'Комиссия Bitcoin (медиана блоков), сат/vB', color: '#fb923c', step: false },
  fee_btc_usd: { label: 'Перевод Bitcoin, $', color: '#fb923c', step: false },
  fee_tron_usd: { label: 'Перевод USDT по TRON, $', color: '#f87171', step: false },
  fee_eth_usd: { label: 'Перевод USDT по Ethereum, $', color: '#a78bfa', step: false },
  dom_btc: { label: 'Доля Bitcoin в рынке, %', color: '#fb923c', step: false },
  mcap_total: { label: 'Капитализация крипторынка, $ трлн', color: '#38bdf8', step: false },
  pw_ua: { label: 'Покупательная способность: Украина', color: '#fbbf24', step: false },
  pw_us: { label: 'Покупательная способность: США', color: '#38bdf8', step: false },
  stress_idx: { label: 'Индекс стресса США', color: '#f87171', step: false, noLog: true },
  pw_ez: { label: 'Покупательная способность: Еврозона', color: '#a78bfa', step: false },
  pw_de: { label: 'Покупательная способность: Германия', color: '#a78bfa', step: false },
  pw_gb: { label: 'Покупательная способность: Великобритания', color: '#f472b6', step: false },
  pw_ch: { label: 'Покупательная способность: Швейцария', color: '#e2e8f0', step: false },
  pw_jp: { label: 'Покупательная способность: Япония', color: '#f87171', step: false },
  pw_cn: { label: 'Покупательная способность: Китай', color: '#fb923c', step: false },
  pw_ru: { label: 'Покупательная способность: Россия', color: '#94a3b8', step: false },
  pw_pl: { label: 'Покупательная способность: Польша', color: '#34d399', step: false },
  pw_tr: { label: 'Покупательная способность: Турция', color: '#f87171', step: false },
  pw_no: { label: 'Покупательная способность: Норвегия', color: '#38bdf8', step: false },
  pw_se: { label: 'Покупательная способность: Швеция', color: '#fbbf24', step: false },
  pw_ca: { label: 'Покупательная способность: Канада', color: '#fb923c', step: false },
  pw_eg: { label: 'Покупательная способность: Египет', color: '#a3e635', step: false },
  pw_kr: { label: 'Покупательная способность: Южная Корея', color: '#22d3ee', step: false },
  pw_au: { label: 'Покупательная способность: Австралия', color: '#facc15', step: false },
  pw_br: { label: 'Покупательная способность: Бразилия', color: '#4ade80', step: false },
  rerusd_ua: { label: 'Реальный курс к доллару: Украина', color: '#fbbf24', step: false },
  rerusd_ez: { label: 'Реальный курс к доллару: Еврозона', color: '#a78bfa', step: false },
  rerusd_de: { label: 'Реальный курс к доллару: Германия', color: '#a78bfa', step: false },
  rerusd_gb: { label: 'Реальный курс к доллару: Великобритания', color: '#f472b6', step: false },
  rerusd_ch: { label: 'Реальный курс к доллару: Швейцария', color: '#e2e8f0', step: false },
  rerusd_jp: { label: 'Реальный курс к доллару: Япония', color: '#f87171', step: false },
  rerusd_cn: { label: 'Реальный курс к доллару: Китай', color: '#fb923c', step: false },
  rerusd_ru: { label: 'Реальный курс к доллару: Россия', color: '#94a3b8', step: false },
  rerusd_pl: { label: 'Реальный курс к доллару: Польша', color: '#34d399', step: false },
  rerusd_tr: { label: 'Реальный курс к доллару: Турция', color: '#f87171', step: false },
  rerusd_no: { label: 'Реальный курс к доллару: Норвегия', color: '#38bdf8', step: false },
  rerusd_se: { label: 'Реальный курс к доллару: Швеция', color: '#fbbf24', step: false },
  rerusd_ca: { label: 'Реальный курс к доллару: Канада', color: '#fb923c', step: false },
  rerusd_eg: { label: 'Реальный курс к доллару: Египет', color: '#a3e635', step: false },
  rerusd_kr: { label: 'Реальный курс к доллару: Южная Корея', color: '#22d3ee', step: false },
  rerusd_au: { label: 'Реальный курс к доллару: Австралия', color: '#facc15', step: false },
  rerusd_br: { label: 'Реальный курс к доллару: Бразилия', color: '#4ade80', step: false },
  fx_dxy: { label: 'Индекс доллара (расчёт по курсам НБУ)', color: '#34d399', step: false },
  ua_reserves: { label: 'Резервы НБУ, млрд $', color: '#34d399', step: false },
  ua_m0: { label: 'Наличные вне банков (M0), млрд ₴', color: '#38bdf8', step: false },
  ua_m3: { label: 'Денежная масса M3, млрд ₴', color: '#a78bfa', step: false },
  ua_m3_yoy: { label: 'Денежная масса M3 г/г', color: '#fb923c', step: false },
  ua_ppi_yoy: { label: 'Цены производителей г/г', color: '#f472b6', step: false },
  ua_ca: { label: 'Текущий счёт, млрд $ за месяц', color: '#38bdf8', step: false },
  ua_gs: { label: 'Товары и услуги, млрд $ за месяц', color: '#fbbf24', step: false },
  ua_cpi_01: { label: 'Продукты питания и безалк. напитки', color: '#34d399', step: false },
  ua_cpi_02: { label: 'Алкоголь и табак', color: '#a78bfa', step: false },
  ua_cpi_03: { label: 'Одежда и обувь', color: '#f472b6', step: false },
  ua_cpi_04: { label: 'Жильё, вода, энергия, газ', color: '#fb923c', step: false },
  ua_cpi_05: { label: 'Предметы домашнего обихода', color: '#2dd4bf', step: false },
  ua_cpi_06: { label: 'Здравоохранение', color: '#f87171', step: false },
  ua_cpi_07: { label: 'Транспорт', color: '#38bdf8', step: false },
  ua_cpi_08: { label: 'Связь', color: '#818cf8', step: false },
  ua_cpi_09: { label: 'Отдых и культура', color: '#4ade80', step: false },
  ua_cpi_10: { label: 'Образование', color: '#94a3b8', step: false },
  ua_cpi_11: { label: 'Рестораны и отели', color: '#fbbf24', step: false },
  ua_cpi_12: { label: 'Разное', color: '#64748b', step: false },
  pl_policy: { label: 'NBP: ставка', color: '#a78bfa', step: true },
  pl_cpi: { label: 'Инфляция Польши г/г', color: '#fb923c', step: false },
  pl_u: { label: 'Безработица Польши', color: '#38bdf8', step: false },
  pl_y10: { label: 'Польша: 10 лет', color: '#fbbf24', step: false },
  pln_uah: { label: 'Польский злотый', color: '#f87171', step: false },
  tr_policy: { label: 'ЦБ Турции: ставка', color: '#a78bfa', step: true },
  tr_cpi: { label: 'Инфляция Турции г/г', color: '#fb923c', step: false },
  tr_u: { label: 'Безработица Турции', color: '#38bdf8', step: false },
  try_uah: { label: 'Турецкая лира', color: '#f87171', step: false },
  no_policy: { label: 'Norges Bank: ставка', color: '#a78bfa', step: true },
  no_cpi: { label: 'Инфляция Норвегии г/г', color: '#fb923c', step: false },
  no_u: { label: 'Безработица Норвегии', color: '#38bdf8', step: false },
  no_y10: { label: 'Норвегия: 10 лет', color: '#fbbf24', step: false },
  nok_uah: { label: 'Норвежская крона', color: '#f87171', step: false },
  se_policy: { label: 'Riksbank: ставка', color: '#a78bfa', step: true },
  se_cpi: { label: 'Инфляция Швеции г/г', color: '#fb923c', step: false },
  se_u: { label: 'Безработица Швеции', color: '#38bdf8', step: false },
  se_y10: { label: 'Швеция: 10 лет', color: '#fbbf24', step: false },
  sek_uah: { label: 'Шведская крона', color: '#f87171', step: false },
  ca_policy: { label: 'Банк Канады: ставка', color: '#a78bfa', step: true },
  ca_cpi: { label: 'Инфляция Канады г/г', color: '#fb923c', step: false },
  kr_policy: { label: 'Банк Кореи: ставка', color: '#22d3ee', step: true },
  kr_cpi: { label: 'Инфляция Южной Кореи г/г', color: '#f472b6', step: false },
  au_policy: { label: 'РБА: ставка', color: '#facc15', step: true },
  au_cpi: { label: 'Инфляция Австралии г/г', color: '#fb923c', step: false },
  au_u: { label: 'Безработица Австралии', color: '#38bdf8', step: false },
  au_y10: { label: 'Австралия: 10 лет', color: '#fbbf24', step: false },
  aud_uah: { label: 'Австралийский доллар', color: '#facc15', step: false },
  br_policy: { label: 'Бразилия: ставка Selic', color: '#4ade80', step: true },
  br_cpi: { label: 'Инфляция Бразилии г/г', color: '#fb923c', step: false },
  ca_u: { label: 'Безработица Канады', color: '#38bdf8', step: false },
  ca_y10: { label: 'Канада: 10 лет', color: '#fbbf24', step: false },
  cad_uah: { label: 'Канадский доллар', color: '#f87171', step: false },
  us_sofr: { label: 'SOFR', color: '#38bdf8', step: false },
  us_hy: { label: 'Спред HY (высокий доход)', color: '#f87171', step: false },
  us_ig: { label: 'Спред IG (инвест. рейтинг)', color: '#38bdf8', step: false },
  us_nfci: { label: 'Финансовые условия (NFCI)', color: '#a78bfa', step: false },
  us_usd: { label: 'Индекс доллара (широкий)', color: '#fbbf24', step: false },
  us_debt_gdp: { label: 'Госдолг к ВВП', color: '#fb923c', step: false },
  crypto_btc: { label: 'Bitcoin', color: '#fb923c', step: false },
  crypto_eth: { label: 'Ethereum', color: '#a78bfa', step: false },
  crypto_bnb: { label: 'BNB', color: '#fbbf24', step: false },
  crypto_sol: { label: 'Solana', color: '#34d399', step: false },
  // gradient: цвет линии зависит от значения (stops: [доля от from до to, цвет, прозрачность]); noLog — ограниченный индекс, логарифм не нужен
  crypto_fng: { label: 'Страх и жадность', color: '#94a3b8', step: false, noLog: true, gradient: { from: 0, to: 100, stops: [[0, '#38bdf8', 1], [0.5, '#94a3b8', 0.4], [1, '#fbbf24', 1]] } },
  stock_sp500: { label: 'S&P 500', color: '#38bdf8', step: false },
  stock_nasdaq: { label: 'NASDAQ Composite', color: '#a78bfa', step: false },
  stock_dow: { label: 'Dow Jones', color: '#fbbf24', step: false },
  stock_sp500_dd: { label: 'S&P 500: просадка от максимума', color: '#38bdf8', step: false },
  stock_nasdaq_dd: { label: 'NASDAQ: просадка от максимума', color: '#a78bfa', step: false },
  stock_dow_dd: { label: 'Dow Jones: просадка от максимума', color: '#fbbf24', step: false },
  crypto_btc_dd: { label: 'Bitcoin: просадка от максимума', color: '#fb923c', step: false },
  crypto_eth_dd: { label: 'Ethereum: просадка от максимума', color: '#a78bfa', step: false }
}

// Число для оси и подсказки по единице ряда: «%» — процент, «₴»/«$» — деньги (тысячи сокращаются: 183 562 → «184 тыс.»)
export function unitText(v, unit = '%') {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  if (unit === '%') return plainPct(v)
  const abs = Math.abs(v)
  const body = abs >= 10000 ? `${Math.round(v / 1000)} тыс.` : abs >= 100 ? String(Math.round(v)) : numText(v, 2)
  return `${body} ${unit}`
}
// Метка оси: то же, но короче (без десятых у дробных, чтобы подписи не слипались)
export const axisText = (v, unit = '%') => {
  if (unit === '%') return `${v} %`
  if (Math.abs(v) >= 10000) return `${Math.round(v / 1000)}к ${unit}`
  return unitText(v, unit)
}

// Золото в долларах: гривенная цена унции на курс доллара той же даты (курс — последний не позже даты)
export function goldUsd(series) {
  const g = series && series.xau_uah, u = series && series.usd_uah
  if (!g || !u || !g.length || !u.length) return []
  const out = []
  for (const p of g) {
    const r = valueAt(u, p.d)
    if (r) out.push({ d: p.d, v: p.v / r })
  }
  return out
}

// Среднегодовое изменение ряда за последние years лет, % (сложный процент). Нет данных так далеко назад — null.
export function annualChange(points, todayIso, years) {
  const last = lastPoint(points)
  if (!last || !(years > 0)) return null
  const then = valueAt(points, isoAt(-years, todayIso))
  if (then === null || !(then > 0) || !(last.v > 0)) return null
  return (Math.pow(last.v / then, 1 / years) - 1) * 100
}

const DAY = 86400000
const ms = (iso) => Date.parse(`${iso}T00:00:00Z`)

// Значение ряда на дату: последняя точка не позже даты; раньше первой точки — null. Ряд отсортирован по возрастанию.
export function valueAt(points, iso) {
  if (!points || !points.length || !iso || iso < points[0].d) return null
  let lo = 0, hi = points.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (points[mid].d <= iso) lo = mid
    else hi = mid - 1
  }
  return points[lo].v
}

// Последняя точка ряда или null
export const lastPoint = (points) => (points && points.length ? points[points.length - 1] : null)

// «2026-11-13» → «13.11.2026»; пусто для пустой даты
export const dateOf = dmy
// Есть ли хоть одна точка хоть в одном ряду (пока нет — на экране подсказка «Нажми Обновить»)
export const hasPoints = (series) => Object.values(series || {}).some((a) => a && a.length)

// Реальная доходность, % (по Фишеру): (1 + номинальная) / (1 + инфляция) − 1. Нет инфляции или ставки — null.
export function realRate(nominal, inflation) {
  if (!(typeof nominal === 'number' && isFinite(nominal)) || !(typeof inflation === 'number' && isFinite(inflation))) return null
  return ((1 + nominal / 100) / (1 + inflation / 100) - 1) * 100
}

/** Инфляция г/г (последний известный месяц) в валюте бумаги: UAH — НБУ, USD — США, EUR — еврозона (HICP). Нет данных или чужая валюта — null. */
export function inflationFor(currency, series) {
  const id = { UAH: 'cpi_yoy', USD: 'us_cpi_yoy', EUR: 'eu_cpi_yoy' }[currency]
  return id ? lastPoint(series && series[id])?.v ?? null : null
}

// Текущая инфляция для расчёта реальной доходности: последний известный месяц (г/г). null — данных нет.
export const currentInflation = (series) => lastPoint(series && series.cpi_yoy)?.v ?? null

// Знаковый процент с запятой: «+9,4 %», «−1,2 %», «0 %»
export function signedPct(v) {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  const r = Math.round(v * 10) / 10
  const s = numText(Math.abs(r), 1)
  return `${r > 0 ? '+' : r < 0 ? '−' : ''}${s} %`
}
export const plainPct = (v) => (v === null || v === undefined || !isFinite(v) ? '—' : `${numText(v, 2)} %`)

// Сводка для шапки блока: текущие значения и разрыв «ставка − инфляция»
export function macroSummary(series, todayIso) {
  const at = (id) => (series && series[id] ? valueAt(series[id], todayIso) : null)
  const policy = at('policy_rate')
  const cpi = lastPoint(series && series.cpi_yoy)
  return {
    policy,
    overnightCD: at('overnight_cd'),
    cpi: cpi ? cpi.v : null,
    cpiMonth: cpi ? cpi.d.slice(0, 7) : '',
    core: lastPoint(series && series.core_yoy)?.v ?? null,
    realPolicy: policy !== null && cpi ? realRate(policy, cpi.v) : null
  }
}

export const MACRO_PAD = { l: 56, r: 16, t: 12, b: 36 } // поля графика (px во viewBox): по ним мышь переводится в данные
export const MACRO_MIN_SPAN = 0.5 // дальше приближать смысла нет: полгода
export const MACRO_MAX_SPAN = 60

const YEAR_MS = 365.25 * DAY
// Годы от сегодня (отрицательные — прошлое): та же ось X, что у графика роста, поэтому и общий движок (composables/useChartView.js)
export const yearsFrom = (iso, todayIso) => (ms(iso) - ms(todayIso)) / YEAR_MS
export const isoAt = (years, todayIso) => new Date(ms(todayIso) + years * YEAR_MS).toISOString().slice(0, 10)

const usable = (series, ids) => (ids || []).filter((id) => series && series[id] && series[id].length)

/** Окно по умолчанию: years лет назад (0 — с первой точки, весь период), до сегодня с небольшим запасом справа. Левее первой точки данных не уходит. */
export function macroWindow({ series, ids, todayIso, years = 0 }) {
  const use = usable(series, ids)
  if (!use.length) return { x0: -1, x1: 0.05 }
  const first = use.map((id) => series[id][0].d).sort()[0]
  const firstT = yearsFrom(first, todayIso)
  const left = years ? Math.max(-years, firstT) : firstT
  const span = Math.max(-left, MACRO_MIN_SPAN)
  return { x0: -span, x1: span * 0.02 }
}

/** Вертикаль: auto — под видимое окно без редких выбросов (один сбойный месяц не должен сплющивать график), ext — крайности по всем данным. */
export function macroRanges({ series, ids, view, todayIso, floor = 0, log = false }) {
  const visible = [], all = []
  for (const id of usable(series, ids)) {
    const pts = series[id]
    const before = valueAt(pts, isoAt(view.x0, todayIso))
    if (before !== null) visible.push(before)
    for (const p of pts) {
      all.push(p.v)
      const t = yearsFrom(p.d, todayIso)
      if (t >= view.x0 && t <= view.x1) visible.push(p.v)
    }
  }
  return robustRanges({ visible, all, floor, log })
}

/**
 * График: ставка ступенькой (держится до следующей смены, последнее значение тянется до сегодня), инфляция ломаной.
 * view — окно { x0, x1 } в годах от сегодня, yRange — { lo, hi }. Точки за краями остаются в линии, обрезает рамка графика.
 * Возвращает { lines:[{id,color,label,d}], yTicks, xTicks, todayX, zeroY, empty, pad, width, height }.
 */
export function buildMacroChart({ series, ids, view, yRange, todayIso, width = 1000, height = 220, pad = MACRO_PAD, scale = 'linear' }) {
  const use = usable(series, ids)
  if (!use.length) return { lines: [], yTicks: [], xTicks: [], todayX: 0, zeroY: null, empty: true, pad, width, height }
  const { x0, x1 } = view
  const { lo, hi } = yRange
  const X = (t) => pad.l + ((t - x0) / (x1 - x0)) * (width - pad.l - pad.r)
  const sc = scaleOf(scale)
  // yRange — в шкальном пространстве (при log это логарифмы); значения ≤ 0 на логарифмической оси не рисуются (разрыв линии)
  const Y = (v) => pad.t + (1 - (sc.fwd(v) - lo) / (hi - lo)) * (height - pad.t - pad.b)
  const margin = (x1 - x0) * 0.05

  const lines = use.map((id) => {
    const meta = SERIES[id] || { label: id, color: '#94a3b8', step: false }
    // точки окна с запасом и одна точка перед ним (чтобы линия входила с левого края)
    const pts = []
    let prev = null
    for (const p of series[id]) {
      if (sc.mode === 'log' && !(p.v > 0)) continue
      const t = yearsFrom(p.d, todayIso)
      if (t < x0 - margin) { prev = { t: x0 - margin, v: p.v }; continue }
      if (t > x1 + margin) break
      if (prev) { pts.push(prev); prev = null }
      pts.push({ t, v: p.v })
    }
    if (prev) pts.push(prev)
    let d = ''
    pts.forEach((p, i) => {
      const x = X(p.t).toFixed(1), y = Y(p.v).toFixed(1)
      d += i === 0 ? `M${x} ${y}` : meta.step ? `H${x}V${y}` : `L${x} ${y}`
    })
    if (meta.step && pts.length) d += `H${X(Math.min(x1 + margin, 0)).toFixed(1)}`
    // градиент по значению: вектор от значения from до to в пикселях графика (userSpaceOnUse), поэтому цвет привязан к уровню, а не к положению на экране
    const g = meta.gradient ? { y1: Y(meta.gradient.from), y2: Y(meta.gradient.to), stops: meta.gradient.stops.map(([offset, color, opacity]) => ({ offset, color, opacity })) } : null
    // ряд из единственной точки (снимок, который только начал копиться) линией не нарисовать: рисуется маркер
    const dot = series[id].length === 1 && pts.length === 1 ? { x: X(pts[0].t), y: Y(pts[0].v) } : null
    return { id, color: meta.color, label: meta.label, d, dot, gradient: g && isFinite(g.y1) && isFinite(g.y2) && g.y1 !== g.y2 ? g : null }
  })

  return {
    lines,
    yTicks: scaleTicks(sc.mode, lo, hi, 5).map((v) => ({ v, y: Y(v) })),
    xTicks: timeTicks(x0, x1, todayIso).map((t) => ({ x: X(t.t), label: t.label })),
    todayX: X(0),
    zeroY: sc.mode === 'linear' && lo < 0 && hi > 0 ? Y(0) : null,
    pad, width, height,
    empty: false
  }
}

// ── Экономика США: сигналы по рядам FRED (чистые функции; подсказки на экране, не советы).

/**
 * Правило Сам (Sahm): средняя безработица за 3 месяца минус её минимум за последние 12 месяцев (включая текущий).
 * Рост на 0,5 п. п. и больше исторически совпадал с началом рецессии. Ряд — помесячные точки по возрастанию; меньше 14 точек — null.
 * Возвращает { value, signal } (value в п. п.).
 */
export function sahmRule(points) {
  const v = (points || []).map((p) => p.v)
  if (v.length < 14) return null
  const avg = []
  for (let i = 2; i < v.length; i++) avg.push((v[i] + v[i - 1] + v[i - 2]) / 3)
  const last = avg[avg.length - 1]
  const min = Math.min(...avg.slice(-12))
  const value = Math.round((last - min) * 100) / 100
  return { value, signal: value >= 0.5 }
}

/** Два квартала подряд спада ВВП — «техническая рецессия». Ряд поквартальный; false, если меньше двух точек. */
export function technicalRecession(points) {
  const n = (points || []).length
  return n >= 2 && points[n - 1].v < 0 && points[n - 2].v < 0
}

/** Состояние кривой доходности (10 л. − 2 г., п. п.): отрицательная — «инвертирована» (исторически опережала рецессии). null — нет данных. */
export const curveState = (spread) => (spread === null || spread === undefined || !isFinite(spread) ? null : spread < 0 ? 'inverted' : spread < 0.5 ? 'flat' : 'normal')

/** Уровень VIX словами: до 20 спокойно, 20–30 тревожно, выше 30 паника. */
export const vixLevel = (v) => (v === null || v === undefined || !isFinite(v) ? null : v < 20 ? 'calm' : v <= 30 ? 'nervous' : 'panic')

// ── Крипто: просадка от максимума, рост за период, индекс страха и жадности словами (чистые функции).

/** Просадка от исторического максимума, %: 0 на максимуме, −50 — цена вдвое ниже пика. Ряд по возрастанию дат. */
export function drawdown(points) {
  let peak = 0
  const out = []
  for (const p of points || []) {
    if (!(p.v > 0)) continue
    if (p.v > peak) peak = p.v
    out.push({ d: p.d, v: (p.v / peak - 1) * 100 })
  }
  return out
}

/** Максимум за всю историю и где цена сейчас относительно него: { ath, date, fromAth (%, ≤ 0) }. Нет данных — null. */
export function athInfo(points) {
  const pts = (points || []).filter((p) => p.v > 0)
  if (!pts.length) return null
  let top = pts[0]
  for (const p of pts) if (p.v > top.v) top = p
  const last = pts[pts.length - 1]
  return { ath: top.v, date: top.d, fromAth: (last.v / top.v - 1) * 100 }
}

/** Изменение цены за years лет назад (по последней точке не позже даты), % — не среднегодовое. Нет данных так далеко — null. */
export function changeSince(points, todayIso, years) {
  const last = lastPoint(points)
  if (!last || !(years > 0)) return null
  const then = valueAt(points, isoAt(-years, todayIso))
  return then > 0 && last.v > 0 ? (last.v / then - 1) * 100 : null
}

/** Индекс страха и жадности словами: до 25 крайний страх, до 45 страх, до 55 нейтрально, до 75 жадность, выше крайняя жадность. */
export const fearGreedLevel = (v) => (v === null || v === undefined || !isFinite(v) ? null : v < 25 ? 'extreme-fear' : v < 45 ? 'fear' : v <= 55 ? 'neutral' : v <= 75 ? 'greed' : 'extreme-greed')

// Кривая доходности Treasury: сроки слева направо и ряды, из которых берётся точка кривой (10 лет лежит в источнике «us»)
export const CURVE_TENORS = [
  { id: 'ust_1m', label: '1м' }, { id: 'ust_3m', label: '3м' }, { id: 'ust_6m', label: '6м' }, { id: 'ust_1y', label: '1г' },
  { id: 'ust_2y', label: '2г' }, { id: 'ust_3y', label: '3г' }, { id: 'ust_5y', label: '5л' }, { id: 'ust_7y', label: '7л' },
  { id: 'ust_10y', label: '10л' }, { id: 'ust_20y', label: '20л' }, { id: 'ust_30y', label: '30л' }
]

/** Снимок кривой на дату: по точке на срок. yearsAgo = 0 — последние значения, иначе значение на дату «столько лет назад» (последнее не позже неё). Нет данных — v: null. */
export function curveSnapshot(series, todayIso, yearsAgo = 0) {
  return CURVE_TENORS.map(({ id, label }) => {
    const pts = (series && series[id]) || []
    const v = yearsAgo > 0 ? valueAt(pts, isoAt(-yearsAgo, todayIso)) : (lastPoint(pts)?.v ?? null)
    return { id, label, v }
  })
}

// Шаг сетки «круглый»: 1, 2, 2.5 или 5 на степень десяти
function niceStep(raw) {
  const k = Math.pow(10, Math.floor(Math.log10(raw)))
  for (const m of [1, 2, 2.5, 5, 10]) if (m * k >= raw) return m * k
  return 10 * k
}

/**
 * График снимков кривой: сроки равномерно по горизонтали, доходность по вертикали. snaps — [{ key, color, points: curveSnapshot(...) }].
 * Возвращает { empty, xs: [{ x, label }], yTicks: [{ y, v }], lines: [{ key, color, d }], width, height, pad }. Пропуски (v: null) рвут линию.
 */
export function curveChart(snaps, { width = 1000, height = 220, pad = MACRO_PAD } = {}) {
  const vals = snaps.flatMap((s) => s.points.map((p) => p.v)).filter((v) => typeof v === 'number' && isFinite(v))
  const base = { width, height, pad, xs: [], yTicks: [], lines: [] }
  if (!vals.length) return { ...base, empty: true }
  let lo = Math.min(...vals), hi = Math.max(...vals)
  if (!(hi > lo)) { lo -= 0.5; hi += 0.5 }
  const margin = (hi - lo) * 0.08
  lo -= margin; hi += margin
  const step = niceStep((hi - lo) / 4)
  const yOf = (v) => pad.t + (1 - (v - lo) / (hi - lo)) * (height - pad.t - pad.b)
  const n = snaps[0].points.length
  const xOf = (i) => pad.l + (n > 1 ? (i / (n - 1)) * (width - pad.l - pad.r) : 0)
  const yTicks = []
  for (let t = Math.ceil(lo / step) * step; t <= hi + 1e-9; t += step) yTicks.push({ y: yOf(t), v: Math.round(t * 1000) / 1000 })
  const lines = snaps.map((s) => {
    let d = '', pen = false
    s.points.forEach((p, i) => {
      if (typeof p.v !== 'number' || !isFinite(p.v)) { pen = false; return }
      d += `${pen ? 'L' : 'M'}${xOf(i).toFixed(1)} ${yOf(p.v).toFixed(1)} `
      pen = true
    })
    return { key: s.key, color: s.color, d: d.trim() }
  })
  return { ...base, empty: false, xs: snaps[0].points.map((p, i) => ({ x: xOf(i), label: p.label })), yTicks, lines }
}

/** Рост реальных зарплат: годовой рост заработка минус годовая инфляция на тот же месяц (п. п.). Нет пары — null. */
export function realWageGrowth(wagesYoY, cpiYoY) {
  const w = lastPoint(wagesYoY)
  if (!w) return null
  const c = valueAt(cpiYoY, w.d)
  return c === null ? null : { d: w.d, v: Math.round((w.v - c) * 100) / 100 }
}

/** Цель ФРС по инфляции PCE, % годовых */
export const FED_INFLATION_TARGET = 2

/** Млрд долларов словами крупно: 5900 → «5,9 трлн $», 480 → «480 млрд $». Нет значения — «—». */
export function billionsText(v) {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  const a = Math.abs(v)
  return a >= 1000 ? `${numText(v / 1000, 2)} трлн $` : `${numText(v, 0)} млрд $`
}

/** Спред высокодоходных облигаций к Treasury (п. п.) словами: до 4 — спокойно, до 6 — настороженно, выше — стресс. null — нет данных. */
export const creditLevel = (hy) => (hy === null || hy === undefined || !isFinite(hy) ? null : hy < 4 ? 'calm' : hy <= 6 ? 'wary' : 'stress')
