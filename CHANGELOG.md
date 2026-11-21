**English** · [Русский](CHANGELOG.ru.md) · [Українська](CHANGELOG.uk.md)

# Changelog

Versions: `R.sB.C+T.hash`. On `dev` there is no state letter (`v1.1.C+T`), the letter appears in `build`: `v1.b1.C+T`. Details — [docs/WORKFLOW.md](docs/WORKFLOW.md). Each entry is one named commit (one feature).

## Build 3 — polishing and finishing (plan, `v1.3.C+T`)

Polishing the user-friendly design and finishing QoL, builder and reference features.

- **v1.3.25** — Cases, Glovo "Смета": FOP taxes. Single tax and military levy are counted from "Выплата" operations, ESV is a fixed amount for every month since the first payout; defaults are FOP group 3, 2026 (5%, 1%, 1 902,34 ₴) and can be changed per canvas. A "Налоги ФОП" line sits before "В кармане", which is now after taxes, plus a table by month. Backend: table `workspace_params` and `/api/workspaces/params`; a canvas takes its parameters with it when deleted
- **v1.3.24** — Cases, Glovo page, "Смета": consolidation of the accounting. "Сведение" goes from the Glovo nominal through withheld and received (Доход + Выплата), cash tips and expenses with losses to "В кармане"; "Деньги по неделям" is a chart by week (income up, expenses and losses down, running total line, per-week hover). Read-only, uses the same badges and nominal as "Учёт"
- **v1.3.23** — Cases, Glovo page: the weekly nominal form is replaced by a scrollable week-by-week timeline. Each card holds the nominal (typed inline), what arrived the NEXT week (Доход/Выплата, since the money comes 2-4 days after the report), the delay in days and the withheld amount; tips are cash and not counted. A "Раньше" button adds earlier weeks, ‹ Сегодня › jump along the lane
- **v1.3.22** — Cases, the Glovo page: two parts, "Учёт" and "Смета" (the latter is empty so far). In "Учёт": the accounts of the canvas (selected on the cards), badges on the operations of these accounts (Расход, Убыток, Чай, Доход, Выплата; set in the opened history row, an operation belongs to one canvas), totals per badge and weekly entries of nominal income (any day of the week lands on its Monday). Backend: tables `workspace_accounts`, `workspace_marks`, `workspace_weeks` and `/api/workspaces/accounts`, `/marks`, `/weeks`; deleting a canvas deletes its accounting too. The history table got optional badges
- **v1.3.21** — Cases: clicking a card opens the page of the canvas. Pages are bound to cards by a key (the `page` field of a workspace, a registry in `workspacePages.js`, the choice is in the creation and edit window); the header is drawn from the card data (cover, name, back button). One page so far, an empty "Glovo"; a canvas without a page opens with a stub. Backend: the `page` column in `workspaces` (an existing database is migrated on start)
- **v1.3.20** — P2P charts and header: buying USDT is now green and selling red (was the other way round, which read counter-intuitively); the P2P spread is signed now: negative when selling is priced above buying
- **v1.3.19** — Cases: a pencil (edit in the creation window) and a trash (delete in two presses) on a card, shown on hover
- **v1.3.18** — Cases: the cover picture is set only by a link; picking a local file (with compression and drag-and-drop) is removed along with data-URL covers in the backend; the creation window lost its extra captions
- **v1.3.17** — Cases: the button moved from the side menu to the header, cards are smaller (about five per row), the list caption is removed, the cover can be set by a link (http/https) as well as by a file; the backend accepts such links
- **v1.3.16** — Cases: a new "Дела" section (workspaces). Canvas cards (cover of your colour or picture on top, a dark fade with the name at the bottom), a round "+" button with an SVG icon at the bottom right and a creation window with live preview. Backend: `workspaces` table in core.db and `/api/workspaces` (list, create, update, delete) with validation of name, colour and picture
- **v1.3.15** — Companies: the capitalization warning now tells the two causes apart. If the computed value is much higher than Nasdaq's (an ADR: SEC reports ordinary shares, the exchange prices a receipt worth several of them, TSMC = 5), it shows "≈ ×4.9, probably an ADR" instead of the text about several share classes, which explains only the opposite case
- **v1.3.14** — Charts: a step without data is now visible. Where a series was silent noticeably longer than its usual step (hourly P2P snapshots while the server was off, a skipped month, a company report that did not come out) the line goes dashed instead of joining the points as if nothing happened, the tooltip says "нет данных" there, and a Binance candle with no valuation in its period is hatched. The usual step is taken from the neighbouring points of the series itself, so weekends, holidays and mixed monthly/weekly series give no false dashes; rates still hold until they change
- **v1.3.13** — Candle chart: a P2P purchase of USDT (bank → Binance) no longer draws a phantom wick. The bank side and the Binance side of such a transfer are now matched by time and amount (the accounts are not linked in the data, and the amounts differ by the spread), so the transfer counts once in the candle and in Volume and does not change the colour
- **v1.3.12** — Candle chart: Volume histogram in a separate strip under the candles, the bars hang down from its line — inflow and outflow summed by absolute value; green when the inflow is bigger or equal (also zero), red when the outflow is bigger; a transfer between own accounts counts once and does not change the colour
- **v1.3.11** — Candle chart: moving averages as on Binance — MA(7) yellow, MA(25) pink, MA(99) purple (simple average of closes, a line starts from its Nth candle)
- **v1.3.10** — P2P deal size is now a setting: `P2P_MIN_DEAL_UAH` (default 1000 ₴) — adverts that cannot take a deal of at least that amount do not set the P2P price
- **v1.3.9** — Backend test of a cold start: a fresh install (both databases empty, no tokens or source files) answers every screen's read endpoint with 200 and valid JSON, never 5xx
- **v1.3.8** — Market charts: while series are loading, an empty panel says "Идёт загрузка…" (Loading…) instead of "Нет данных" (No data)
- **v1.3.7** — "Company stocks": the chosen company is remembered, after a reload the screen opens it instead of the first one in the list
- **v1.3.6** — `clc.sh build` repeats the merge while GitHub still counts the PR checks as queued (up to 30 tries, 10 s apart)
- **v1.3.5** — Toasts: pause between leaves cut from 0.5 to 0.2 s; the stack keeps its scroll place when toasts appear and leave (at the top it stays at the newest, at the bottom at the oldest, in the middle it does not move), the slide-in no longer draws a horizontal scrollbar
- **v1.3.4** — Toasts: new ones appear on top, the oldest one at the bottom leaves first (one queue in order of arrival, 0.5 s apart); the "Рыночные данные" (Market data) window: a finished source is one line "Name ✓" without a bar, finished ones on top, running ones at the bottom; in the sync window only a finished Binance collapses this way, banks keep their bar
- **v1.3.3** — Branches by modules (`<module>/<date>-<word>` instead of `wip/`, `clc.sh next` and `-b` take a module, new `clc.sh modules` command, workflow updated); toasts: the stack scrolls when it does not fit (before, the top ones were cut off), expired toasts leave one by one with a 0.5 s pause, each keeping its own lifetime
- **v1.3.2** — Statement import accepts archives (zip, gz, tar, tgz, bz2): statement files are taken from inside and loaded as one batch only if they all point to one source
- **v1.3.1** — Capital candles: a transfer between own accounts (two records from different banks) is applied as one step, so the day's peak and low no longer show a false wick from a double count

## Build 2 — Stable: macro and market data (`v1.s2.113+143`), 2026-11-17

Collecting public data from outside and showing it: OVDP and US bonds, rates, inflation and exchange rates, US macro, cards of countries and the eurozone (22 registry entries), exchanges, company stocks, the crypto loop, purchasing power and the health of sources. At the end of the build: a security audit (Host check, gzip limit), speeding up the "Курсы" (Rates) page, unified number and date formats.

- **v1.2.113** — The "Управление" (Management) menu: the "Выписки" (Statements) button in the shared button style, the extra yellow variant removed
- **v1.2.112** — Frontend: overrides for glob and minimatch in devDependencies, bun audit clean
- **v1.2.111** — Backend: macro_series is a clustered table (WITHOUT ROWID) with a cache migration, faster series reads
- **v1.2.110** — Frontend: all numbers and dates on screens go through shared formatters (a comma, no extra zeros)
- **v1.2.109** — Frontend: a single errText for errors and shared cached Intl formatters
- **v1.2.108** — Backend: fast gzip with a pool of compressors for JSON responses
- **v1.2.107** — Backend: Host header check (protection against DNS rebinding), a gzip unpacking limit in sources
- **v1.2.106** — Macro: sections opened before the write to the cache are reread on return (feed freshness marks)
- **v1.2.105** — Macro series without deep reactivity (shallowRef): the dollar index dozens of times faster
- **v1.2.104** — Macro charts: a dense line is thinned by columns, range sorting is faster
- **v1.2.103** — Macro charts: a numeric series index and window bisection instead of walking all points
- **v1.2.102** — The "Архив" (Archive) section in the side menu: Smeta, Market and Uchet moved there, the header has the brand and "Разведать" (Scout)
- **v1.2.101** — Stock capitalization: outliers of single units in SEC data are replaced by the cover or dropped
- **v1.2.100** — Log axis: powers of ten with a step at a huge span, labels do not stick together
- **v1.2.99** — The ruble to the hryvnia: after the NBU stopped it, the series is continued by a Yahoo cross rate
- **v1.2.98** — Source health: freshness thresholds by series frequency, rates by decision
- **v1.2.97** — Ukraine's balance of payments: the balance instead of the debit
- **v1.2.96** — Stock fixes: SEC frames 404, the price queue after the list, splits in filings
- **v1.2.95** — SEC frames with a 404 response are saved as empty
- **v1.2.94** — The "Акции компаний" (Company stocks) page: price, capitalization, filings, P/E, P/S, P/B
- **v1.2.93** — Stock filings from the SEC: revenue, profit, equity and assets
- **v1.2.92** — Stock data is written to the "Economy" files and restored when the cache is empty
- **v1.2.91** — Stock capitalization by month: the number of shares from SEC filings and Yahoo splits
- **v1.2.90** — Stocks: monthly price history from Yahoo, the top 500 on update and the rest on demand
- **v1.2.89** — Stocks: a list of companies from Nasdaq and the SEC with CIK and capitalization
- **v1.2.88** — Crypto loop: hourly points on the charts of P2P, fees and dominance, the scale down to hours
- **v1.2.87** — Purchasing power: Germany removed from the list (the same data as the Eurozone)
- **v1.2.86** — Purchasing power: the whole series history instead of a period choice, a log scale, small amounts
- **v1.2.85** — India: rate and inflation, the Sensex index, the rupee to hryvnia rate, the real exchange rate, purchasing power
- **v1.2.84** — Saudi Arabia, the UAE and Iran: Saudi rate and inflation, the Tadawul and Dubai indices, the riyal and dirham to hryvnia rates, Iran's annual inflation, real exchange rates, purchasing power
- **v1.2.83** — Australia and Brazil: rate, inflation, the ASX 200 and Bovespa indices, the Australian dollar to hryvnia rate, real exchange rates, purchasing power
- **v1.2.82** — the "Обновить" (Update) button in a country card polls all sources of its indicators (the rate, real rate, index), not only its own
- **v1.2.81** — fix: the Korea and Taiwan card has a stock index line, and the real exchange rates of both currencies are on the "Курсы" page
- **v1.2.80** — South Korea and Taiwan: Korea's rate and inflation (BIS), the KOSPI and TAIEX indices (Yahoo), real exchange rates of both currencies, Korea's purchasing power
- **v1.2.79** — requests to the NBU go no more than two at a time and with a pause: "Разведать" across all sources no longer hangs "key rates" with a timeout
- **v1.2.78** — the P2P price is also computed over fixed-amount ads: dust (under 1,000 ₴) is filtered out, not all twenty ads at once
- **v1.2.77** — requests to sources are retried on a timeout and a connection failure; a P2P error shows Binance's response
- **v1.2.76** — macro series are stored in `.zoe` files of the "Economy" group: the series cache is rebuilt from them (at startup and in "Загрузить" (Load)), "Сбросить кеш" (Reset cache) erases the whole cache database
- **v1.2.75** — shared interface blocks of macro screens: a card, "no data yet" and an amber warning — one component each instead of copies
- **v1.2.74** — a unified look of numbers and dates on all screens: a decimal comma, spaces between digit groups, the date DD.MM.YYYY from one module
- **v1.2.73** — "Рынок" (Market) section load errors are shown as a plate with a "Повторить" (Retry) button, action failures (persons, links, masks, clipboard, Binance data) as notifications, not a silent write to the console
- **v1.2.72** — crypto loop: snapshots (P2P, fees, dominance) accumulate every hour and are not erased on update; the history lies in `.zoe` files of the "Economy" group, a server restart does not shift the slot
- **v1.2.71** — `.zoe` files are laid out by the groups "Accounts" and "Economy": "Economy" holds datasets (series tables) for building the cache, statements do not touch them
- **v1.2.70** — the "Источники" (Sources) page: source health (series freshness, errors), a pause between requests to a host, series search, CSV export
- **v1.2.69** — the "Скоркард" (Scorecard) page: the US economy regime by rules, a stress index, a traffic light of leading signals, real rates and carry of countries
- **v1.2.68** — the "Покупательная способность" (Purchasing power) page: cumulative inflation, the real exchange rate, the price level (PPP), the Big Mac, a comparison of countries, the real yield
- **v1.2.67** — purchasing power data: price indices, PPP, income per capita for 15 countries, the Big Mac Index; formulas for cumulative inflation, the real exchange rate and the price level
- **v1.2.66** — the "Крипто-контур" (Crypto loop) section: the USDT price on P2P, the spread and overpayment relative to the NBU rate, network fees, funding and open interest, stablecoin issuance; single-point charts are drawn as a marker
- **v1.2.65** — the "Мировые акции" (World equities) section: indices of 12 countries, the Buffett indicator, CAPE and the S&P 500 dividend yield
- **v1.2.64** — the "Сырьё" (Commodities) section: oil, gas, metals, grain, sugar and coffee, silver, platinum and palladium from NBU rates, a dollar/hryvnia switch
- **v1.2.63** — "Курсы": a table of world currencies (rate, change over the year, rate, difference with the US rate), cross rates, the dollar index and BIS real effective exchange rates
- **v1.2.62** — Ukraine: inflation by product groups, producer prices and the balance of payments, charts under the country card
- **v1.2.61** — the Ukraine card: NBU reserves, the money supply (M0, M3 and its growth over the year), in the card and next to each other on the charts
- **v1.2.60** — country cards "Norway", "Sweden" and "Canada" (central bank rate, inflation, unemployment, 10-year yield), Norwegian and Swedish krone and Canadian dollar to hryvnia rates from the NBU, series labels in charts
- **v1.2.59** — country cards "Poland" (NBP rate, inflation, unemployment, 10-year yield) and "Turkey" (central bank rate, inflation, unemployment), zloty and Turkish lira to hryvnia rates from the NBU, series labels in charts
- **v1.2.58** — country cards "Russia" (the Bank of Russia key rate and inflation from BIS) and "Egypt" (annual inflation from the World Bank: BIS has none for Egypt, there is no central bank rate in open sources), ruble and Egyptian pound to hryvnia rates from the NBU, series labels in charts
- **v1.2.57** — the "Обновить" button in the header of "Рынок" sections became an icon of two arrows: it rotates on hover, after a press (at least 3 seconds) and while polling — gray and spinning
- **v1.2.56** — country cards "Japan" (the Bank of Japan rate, inflation, unemployment, the 10-year JGB yield, Nikkei 225) and "China" (rate and inflation), the Swiss 10-year yield, yen and yuan to hryvnia rates; sources `jp` and `cn` (BIS and FRED)
- **v1.2.55** — the "Рынок → Макро" tab: country cards "United Kingdom" (the Bank of England rate, SONIA, the 10-year Gilt, inflation, unemployment, GDP) and "Switzerland" (the SNB rate, inflation, unemployment), pound and franc to hryvnia rates; a shared country-bundle mechanism (sources `gb`, `ch`), the "Обновить" button in a card polls the selected country's source; the default backend address is 127.0.0.1 (localhost could go over IPv6 to someone else's program)
- **v1.2.54** — the "Рынок → Макро" tab: the "Еврозона" (Eurozone) section has a comparison of member countries (Germany, France, Italy, Spain, the Netherlands): inflation, unemployment, the 10-year yield and the spreads of Italy, France and Spain to Germany; the member countries live in the `ez` source and do not duplicate the eurozone card in "Страны" (Countries)
- **v1.2.53** — the "Рынок → Макро" tab: the sections "Страны" (a country switcher and a card: rate, inflation, core inflation, unemployment, GDP growth, 10-year yield, rate to the hryvnia; a click on a line draws a chart) and "Еврозона" (ECB rates and €STR, the 2- and 10-year curve, inflation, unemployment, GDP); a new source `ez` (ECB and Eurostat), a shared SDMX-CSV parsing; the country registry is served by the server in `/api/macro/catalog`
- **v1.2.52** — the "Рынок → Макро" tab: the sections "Ликвидность США" (US liquidity) (the M2 money supply, the Fed balance sheet, net liquidity, reverse repo, the Treasury account, SOFR) and "Кредит и доллар" (Credit and the dollar) (HY/IG spreads, NFCI financial conditions, the dollar index, government debt to GDP); two new FRED sources `usliq` and `uscredit`
- **v1.2.51** — the "Рынок → Макро" tab about the US: the sections "Цены США" (US prices) (headline and core inflation, PCE and its difference from the Fed's 2 % target, producer prices, housing), "Рынок труда США" (US labor market) (job growth, U-6 unemployment, jobless claims, vacancies, wages versus inflation) and "Активность США" (US activity) (retail, industry, construction, house prices); three new FRED sources `usprices`, `usjobs`, `usact`; the year-on-year change of indices is computed by one shared function
- **v1.2.50** — the "Рынок → Макро" tab: the sections "Кривая Treasury" (Treasury curve) (a curve snapshot 1 month–30 years today, a year and five years ago, a yield chart, the 10 years − 3 months spread with an inversion note) and "Реальные ставки" (Real rates) (the real TIPS yield, breakeven and 5y5y, the term premium, the 30-year mortgage); two new FRED sources `uscurve` and `usreal` with their own "Обновить" buttons
- **v1.2.49** — the "Выписки" window: accounts are grouped by statement file (a group header with bank/OVDP/HOT badges, a group checkbox, collapsing by an arrow, an indent and a line for nested cards), colored coverage strips by bank brands, a summary strip at the bottom "all accounts"/"selected N of M" (fill and hatch layers with transparency by the number of accounts with data), Binance identifiers are masked like an IBAN; on the backend source labels are an IBAN or an account number, not the service words `icu`/`binance`
- **v1.2.48** — ICU screenshot recognition: for a position without a price the price in the table is not invented (`price-none`, `price-bad`), the header of a neighboring block no longer replaces the date of the previous one (a phantom row), the ISIN is matched with the registry by the tail taking the maturity date into account, a weak match is marked `isin-weak`; the yield to maturity on a coupon day is computed from the current date inclusive; frontend: a test on a real phone screenshot
- **v1.2.47** — sync: after the `USDTUAH` pair was delisted (the last candle 12.11.2026) the hryvnia USDT rate is taken from the official NBU dollar rate: the Binance minute series is appended with it, the delisted pair's ticker is not used; a Mono bank 400 response on a fresh segment is a request error, not the end of history (before, the tail was silently lost), a failure to append the Binance minute series is visible in the sync panel
- **v1.2.46** — frontend: component tests on vitest (`CapitalStructure`, `MarketPanel`, `MarketHeader`, `SyncPanel`, `ToastStack`, the happy-dom environment), `bun run test` runs both them and `node --test`; CI: `golangci-lint` v2.15.0 (v2.5.0 is built on Go 1.25, while go.mod requires 1.27) and frontend tests
- **v1.2.45** — Binance P2P: the order time is aligned with the statement journal: "Buy" orders are matched with "P2P Trading" credits by amount (within 60 minutes), the zone shift is derived from pairs (a multiple of 15 minutes, at least two pairs are needed, strictly better than a zero shift) and applied on reading `GET /api/binance/p2p` (the header `X-Time-Shift-Minutes`), `.zoe` files and the database do not change; fix: `binance.Timeline` balances accumulate in whole steps of 1e-8, a closed position on large quantities (from 1e6 units) zeroes again; frontend: component tests on vitest, `@vue/test-utils` and happy-dom (`bun run test` runs both `node --test` and vitest)
- **v1.2.44** — technical pass 2: the list of series by source is kept by the backend (`GET /api/macro/catalog`), the frontend keeps no id lists of its own; daily FRED series are topped up by a tail from the last date; JSON responses are compressed by gzip; status polling is adaptive (less often when idle, stops on a hidden tab); a `golangci-lint` config; `binance.Series`, `raif.ParseCSV`, `db.initCore`, `ComputeState`, `UnlinkAccount`, `walkAccount`, `extractPageRows`, `loadCacheIndex` split into parts without changing the result
- **v1.2.43** — technical pass 1: `/api/macro` serves series from the cache with an SQL filter, FRED is downloaded in parallel, series are saved with a prepared statement
- **v1.2.42** — errors of starting an update and sync are shown by toasts that go away by themselves (a double click does not duplicate; "already running" is amber), a market source's red toast also goes away after 12 seconds; `clc.sh`: `done`/`merge` and `build` do not quit waiting but write the reason (conflicts, CI did not start, checks are running, no connection); exit on Ctrl+C
- **v1.2.41** — Binance P2P: the C2C order history (Orders → P2P → Export) is read and saved into the same Binance account as a separate `.zoe` source: the hryvnia amount, price, fee, status and time of each order, overlapping exports complement each other, after a cache reset "Загрузить" brings the orders back; reading `GET /api/binance/p2p`; the basis for computing real profit (the hryvnia side of P2P purchases)
- **v1.2.40** — Smeta redesign: "Структура капитала" (Capital structure) became a ring chart (layers outside, currencies inside, highlight and the share in the center on hover) and stands in one row with the "Портфель ОВДП" (OVDP portfolio) tiles; the layer colors are as before
- **v1.2.39** — Smeta redesign: a header with capital and month totals (income, expense, net), "Структура капитала" across the full width (a strip, layer tiles, a currency strip), "Портфель ОВДП" as tiles in one row; the layer colors are as before
- **v1.2.38** — fix: Eurostat sometimes gives gzip without a Content-Encoding header (the body starts with `1f 8b`): the response is recognized by the signature and unpacked, the request goes with `Accept-Encoding: identity`
- **v1.2.37** — fix: the request to Eurostat goes with an explicit `Accept: text/csv` and a clear `User-Agent`, parse errors show the beginning of the response
- **v1.2.36** — fix: eurozone inflation is read from the new Eurostat set `prc_hicp_minr` (the whole history since 1997 in one request), CSV parsing is tolerant of rows of different length
- **v1.2.35** — eurozone inflation (HICP, year to year) from Eurostat: a chart on the "Инфляция" (Inflation) screen, "Обновить" pulls it together with the NBU data; the "Реал." (Real) column in bond tables is computed against the inflation of the security's currency (hryvnia — NBU, dollar — US, euro — eurozone) and is in any section; the order of changelog entries is fixed
- **v1.2.34** — consumer sentiment is colored as a traffic light by value: red when low, yellow around 75, green when high; the index has no log-scale switch
- **v1.2.33** — VIX is colored by value the same way as the fear and greed index: gray around 20, light blue above (fear), amber when very low (complacency)
- **v1.2.32** — fix: the fear and greed index is colored by value (the middle is semi-transparent gray, light blue toward fear, amber toward greed), the hint about 0 and 100 in the header is removed, the index has no log-scale switch
- **v1.2.31** — the logarithmic value scale is built into the shared chart engine (`useChartView`, `chartView.js`): a "Лог. шкала" (Log scale) switch on macro charts with positive values, on in advance on stock prices (S&P 500, NASDAQ, Dow) and crypto; shifting, zooming and vertical animation work the same
- **v1.2.30** — US stock exchange: S&P 500 has history since 1871 (Robert Shiller's monthly data from an open dataset, then the FRED weekly close); NASDAQ since 1971, Dow about 10 years
- **v1.2.29** — fix: the Benner cycle stayed only on the US stock exchange charts (on the others it got in the way of reading), the colors are bright again: red A, yellow B, green C
- **v1.2.28** — US stock exchange: the source `stocks` and the "Биржа США" (US stock exchange) screen in the "Макро" group: S&P 500, NASDAQ Composite and Dow Jones (FRED without a key, the last close of the week), in the S&P 500 header: the level, the distance to the maximum, the change over the year; drawdown from the maximum for all three indices; each chart has the Benner cycle; "Обновить" starts only this data. For S&P 500 and Dow FRED gives about 10 years of history, for NASDAQ — since 1971; the response format of crypto (Binance, fear and greed) was checked with a live request
- **v1.2.27** — crypto: the source `crypto` and the "Крипто" (Crypto) screen in the "Макро" group: weekly prices of Bitcoin, Ethereum, BNB and Solana (public Binance candles without a key, the whole history in one request per coin), drawdown from the all-time high (computed locally), the fear and greed index (alternative.me); in the Bitcoin header: the price, the distance to the maximum, the change over the year; each chart has the Benner cycle; "Обновить" starts only this data
- **v1.2.26** — US economy from FRED: the source `usecon` and the "Экономика США" (US economy) screen in the "Макро" group: unemployment (with the Sahm rule: a rise of 0.5 pp above the year's minimum is a recession signal), real GDP growth quarter by quarter (two quarters of decline in a row — a technical recession), the 10 y. − 2 y. yield curve (below zero — inverted), the VIX fear index and University of Michigan consumer sentiment; each chart has the Benner cycle; the hints are labeled as a historical reading, not a forecast; "Обновить" starts only these series, the series are read only on their own screen
- **v1.2.25** — fix: the Smeta colors by the meaning of currencies: dollar green, hryvnia (cards and accounts) light blue, OVDP purple, crypto orange, euro gray; the previous coloring (hryvnia green, crypto purple) was confused with the dollar
- **v1.2.24** — fix: the Benner cycle became calmer: the colors are muted, the background fill only for the key year and barely noticeable (periods between years without a fill), the footer strip is thinner; the strip with letters remains the main thing
- **v1.2.23** — the Benner cycle became a shared chart layer (`components/BennerBands.vue`, `utils/benner.js`): a chart gives only `xOf(ms)` and the window edges in time, the layer itself draws columns over the full height (the key year A/B/C brightly, the period between key years in the faded color of the previous phase) and a strip with letters in the footer; connected to macro charts, the investment growth chart and capital candles (`msToSlot`/`slotToMs` in `utils/candleMath.js` convert candle slots to time); the hint under the cursor distinguishes the year and the period after it
- **v1.2.22** — the dollar as a common measure: the Smeta capital is computed in dollar equivalent (the dollar as is, the hryvnia and euro at the NBU rate, Binance at the rate of its own scout), next to it in small type an approximate hryvnia equivalent "(≈ ₴…)"; "Структура капитала" shows amounts in $ and a "По валюте" (By currency) breakdown (the first step toward a layered cake); in the "Рынок" tables (OVDP and US) new columns "В $" (yield in dollars: a hryvnia security adjusted for the average devaluation over 5 years, euro via EUR/USD, dollar as is) and "Реальная $" (the same adjusted for US inflation) with green/red color for any currency; "Портфель ОВДП" gets "Доходность в $" and "Реальная в $"; without NBU rates Smeta stays in hryvnia as before
- **v1.2.21** — Smeta: "Структура капитала" (cards and accounts, free money at the broker, OVDP, crypto — amounts and shares as a strip) and "Портфель ОВДП" (average rate and term by position value, real yield adjusted for inflation, the nearest payout); the data from accounts and the ICU journal, the backend did not change
- **v1.2.20** — capital candles on the shared chart engine: wheel zoom with an anchor under the cursor, Shift+wheel — the price scale, dragging sideways and by price, inertia, a double click and "Сброс масштаба" (instead of "Сброс (N%)"); loading history on the left does not move the window; the engine got window bounds (`bounds`), inertia and a sideways touchpad gesture. Returned what was lost in the merge from v1.2.15: in the history header, with a candle selected, "Капитал" (close − open) and "Размах" (maximum − minimum) — intraday volatility, visible even without operations
- **v1.2.19** — macro charts without the buttons "5 y. / 10 y. / From 2007 / All": the default window is the whole data period, zoom by wheel and dragging, "Сброс масштаба" appears only after zooming; `RANGES` and the "from date" window in `macroWindow` are removed
- **v1.2.18** — the US from FRED (a public CSV without a key): the source `us` and the "Ставка ФРС" (Fed rate) screen in the "Макро" group: the Fed rate (effective since 1954 and the top of the target corridor since 2008), the 10-year Treasury yield (weekly values), US inflation y/y and m/m (computed from the CPI index), the real Fed rate and the "Украина и США рядом" (Ukraine and the US side by side) block (real rates of the NBU discount rate and the Fed); "Обновить" starts only the US; the series are read only on this screen
- **v1.2.17** — NBU rates: a new source `fx` (the dollar, euro and gold to the hryvnia for the whole history since 1996, windows of 10 years; the rate before the 1996 redenomination is converted from karbovanets to hryvnia) and the "Курсы" screen in the "Макро" group: the dollar and euro, gold in hryvnias and in dollars (the hryvnia price at the rate of the same date), the average annual rise of the dollar over 1/5/10 years and the estimate "the hryvnia rate in dollars at the same devaluation"; "Обновить" starts only rates; heavy series are read only on this screen
- **v1.2.16** — navigation of the "Рынок" tab: a drawer with collapsible groups "Ценные бумаги" (Securities) (OVDP, US) and "Макро" (NBU rate, Inflation); the rate and inflation have their own screens with charts (rate: the discount rate, overnight certificates and loans; inflation: y/y headline and core since 2007, m/m separately) and an "Обновить" button; on the OVDP screen instead of a big macro chart there is a compact strip "Облікова · Инфляция · Реальная ставка" with links to the screens; the "Обновить" button on the market, lost in v1.2.12, is fixed; a test ensures every `@click` of the template is declared in the script
- **v1.2.15** — the shared chart engine: the time window, the wheel (zoom with an anchor under the cursor), Shift+wheel (value zoom), dragging, a double click, hover and the hybrid vertical with a smooth return were moved out of the growth chart into `utils/chartView.js` and `composables/useChartView.js`; both the investment growth chart and the rate and inflation chart (it used to be static) are now on it; vertical fitting cuts rare outliers by percentiles (a single faulty inflation point no longer flattens the scale), the outliers themselves are reachable by shifting; in the macro block the buttons 5 y. / 10 y. / All, a hint with values at the hover date and zoom reset
- **v1.2.14** — NBU rates and inflation: the discount rate, the rate on overnight deposit certificates and overnight loans (the whole history, only the dates of value change in the cache), the headline and core consumer price index (monthly, y/y and to the previous month); a new source "Rates and inflation · NBU" in "Обновить" and "Разведать", `GET /api/macro`; on the "Рынок" tab a block with the rate and inflation (a chart 5 years / 10 years / all, the real rate) and the "Реальная" column in the OVDP table: the security's yield in hryvnia adjusted for the latest y/y inflation
- **v1.2.13** — the ICU list without manual entry: screenshots of the broker screen are read right in the browser (tesseract.js, locally), ISINs are matched with the NBU registry by maturity date, the price with the shown yield, a table for review before saving; the yield of securities without a price is estimated by a curve (neighboring securities and your own points, on the server `/market/curve`); the list stores each security's price and YTM
- **v1.2.12** — the "Рынок" tab: downloaded bonds as a table, two sections (OVDP · NBU, US) on the left behind the grid; search by ISIN/CUSIP and name, a currency filter, hiding matured ones, sorting by any column, the security coefficient for long-term accumulation (average annual capital growth over 10 years: the rate for the security's term, then at a cautious base: the lower quartile of the currency's rates): the name in a frame with a color from red to green and the Top 1 / 10 / 25 / 100 badges, the "Через 10 л." (In 10 y.) column (what the investment will grow into, the amount is entered at the top) and a growth chart on top (a line per security, the best 5 with dots; wheel zoom, dragging shift; a dashed line after maturity up to the 10-year horizon; a manual amount shift and zoom with a smooth return to fitting, the Top 1/Top 5 gap by currency in the corner, the "только ICU" checkbox with its own ISIN list stored on the server, with a per-security price for computing the real yield (places and Top 1 among those available at the broker), the "N шт." chip at already bought securities, the "сгоревшие на графике" checkbox shows matured ones as a gray dashed line from issue to maturity), the "Обновить" button polls only the selected source
- **v1.2.11** — changelog: the entry numbers of navigation and the capital chart are shifted by one (v1.2.9 → v1.2.10, v1.2.8 → v1.2.9), the code did not change
- **v1.2.10** — navigation: "Операционная" (Operational) and "Лица" (Persons) moved to the "Учет" tab; the section list (History / Persons) stands on the left behind the main grid and does not shift it: labels from 1380px, a column of icons from 1240px, on a narrower window as a row on top; a tab saved from the old version opens in the same place; the Ζωή brand in the header and the page title (ECONOMIC remains only a service name: folders, variables)
- **v1.2.9** — fix: the capital chart no longer goes empty after Smeta → Operational → Persons → Smeta (tabs in KeepAlive reset scrolling, the candle window was computed from the old position)
- **v1.2.8** — the "Выписки" window: a repeat PR of the v1.2.7 fix, nothing changed in the files
- **v1.2.7** — the "Выписки" window: "Разведать" is separated from the cache buttons by empty space, the "дописывать в источники" (append to sources) checkbox under it, a square to the right of the label
- **v1.2.6** — "Разведать" moved from the header to the Smeta tab and the "Выписки" window; instead of "Сохранить с API" (Save from API) a "дописывать в источники" checkbox: after the walk what was taken from the API is written into source files by itself (`/sync/full?save=1`)
- **v1.2.5** — autofill of the "+ ОВДП" form by ISIN from the downloaded NBU registry: maturity, rate, name are substituted into empty fields
- **v1.2.4** — the market data process window: while an update runs, a separate window hangs with progress per source (a percentage or a running bar), the result stays as toasts; a source can report a percentage (`Reporter`)
- **v1.2.3** — market data as a separate process: "Разведать" starts it in parallel with the banks, each source has its own status (`market` in `/sync/status`) and its own toast
- **v1.2.2** — a bond from the reference can be deleted even with journal entries; the layout of the account card with buttons (`actions`)
- **v1.2.1** — market bonds in the cache: the registry and auctions of OVDP (NBU open data) and US Treasury auctions (FiscalData); downloaded OVDP terms are substituted into the ICU account calculation if there is no own entry in the reference

## Build 1 — Beta: data collection (`v1.b1.29+29`), 2026-11-12

Collecting data from personal sources: banks (monobank, Raiffeisen, PrivatBank, NovaPay), the ICU broker, Binance; persons, aliases and account links; the "Операционная" (Operational) tab and the capital candlestick chart. Cut: the merge commit `Build 1 beta` on the `build` branch and the tag `v1.b1.29+29`.

### 2026-11-12 — after the first push (`v1.1.30`–`v1.1.20`)
- **v1.1.30** — README, CHANGELOG, user and technical documentation, the workflow
- **v1.1.29** — `clc.sh build` waits for the checks of the very PR it opened (before, it could watch the checks of the `dev` branch)
- **v1.1.28** — `clc.sh build` merges `dev` into `build` directly, without temporary branches
- **v1.1.27** — the build freeze as a separate merge commit in `build`
- **v1.1.26** — the `clc.sh build` command: a build cut from the last named commit of `dev`
- **v1.1.25** — a 30 s timeout for `pdftotext` (`pkg/pdftext`), the dead export `panKey` removed
- **v1.1.24** — frontend: shared `fmtN`/`formatPeriod` in `formatters.js`, pure chart math in `utils/candleMath.js` (+ tests)
- **v1.1.23** — frontend refactoring:
  - a single API layer `utils/api.js` (the backend address from `VITE_API_BASE`, errors with the backend's text, file upload);
  - safe storage `utils/storage.js` instead of direct `localStorage`; the order of person cards survives a reload;
  - the bank registry `utils/banks.js` (MFO codes, card prefixes, order) instead of duplicates in `ibanMask.js` and `personLayout.js`;
  - `usePaged` — paged display of long tables; the first frontend tests (`banks`, `storage-api`) run in CI
- **v1.1.22** — backend refactoring:
  - a shared kit for statement parsers (`pkg/stmtkit`);
  - an HTTP response layer (`httpx.go`) and a route table (`routes.go`) with unified CORS, a route test;
  - big files split by zones: `api/` (clients, walk, accounts, persons, import, Binance, ICU) and `db/` (persons, aliases, links);
  - `gofmt` in CI went from a warning to mandatory
- **v1.1.21** — GitHub Actions: the build and the PR flow in `clc.sh` (`pr`, `merge`, `done`)
- **v1.1.20** — GitHub Actions: backend tests, the frontend build and hygiene checks (no databases and keys, no 16-digit numbers outside allowed places)

### `v1.1.19`–`v1.1.1` — the stream before the first push
The history was rewritten before publication: these commits got their numbers retroactively and a common timestamp. The order is from newer to older.
- **v1.1.19** — Git tooling: `land` and `next`, short hashes in named commits
- **v1.1.18** — Transfers between own accounts:
  - recognition (`db/own.go`): both sides of a transfer are a movement inside capital, not income or expense;
  - the bank tag and the owner person in history; the NovaPay badge
- **v1.1.17** — NovaPay source: parsing the CSV statement (`pkg/novapay`), the bank theme and the IBAN mask
- **v1.1.16** — Tabs remember their state and scroll position
- **v1.1.15** — Binance coin exchange badges:
  - Convert, converting "dust" to BNB and staking (`binance/swaps.go`);
  - the person in the "Операция" (Operation) column
- **v1.1.14** — The Binance minute series is appended after a new statement (the tail is recomputed, not the whole series)
- **v1.1.13** — IBAN masks:
  - an automatic hidden view `UA44-322001-0000-9976`: the country flag, the bank icon, the first four and last four digits (a Binance profile is `BN-0041`);
  - a click reveals and copies the full number; a pencil sets your own account name (empty — return to the automatic mask);
  - bank logos (`public/banks`)
- **v1.1.12** — A price scale and the last-price tag on the candlestick chart
- **v1.1.11** — Counterparty in history:
  - the person's name and avatar instead of the raw description, an account chip;
  - the account name survives a cache reset
- **v1.1.10** — Unified `.zoe` sources and the "Выписки" (Statements) window:
  - the `.zoe` format (`pkg/source`): one file is one import or a snapshot of one account for a period, already parsed data inside; storage `<folder>/<card>/<date>_<hash>.zoe`, duplicates are dropped by fingerprint;
  - the "Выписки" window: the source inventory, a coverage strip, selective reset, add and update;
  - source checks (`sourcecheck.go`) and a debug section in the menu
- **v1.1.9** — Portfolios on account cards:
  - positions of bonds and Binance wallets (Spot, Funding, Earn, Strategy, Futures): quantity, valuation in ₴, result;
  - the currency symbol before the amount (`₴ 1 234,50`)
- **v1.1.8** — Binance operation badges:
  - operation kinds and wallets; transfers between wallets (Spot ↔ Funding / Strategy / Futures, Earn) as a "from → to" row with no effect on capital;
  - Earn accruals and spot trades (two journal rows → one trade)
- **v1.1.7** — The Operational tab:
  - candles are loaded in chunks and accumulate in the cache by timeframe, scrolling back does not go to the backend for what is already loaded;
  - a choice of accounts the chart is built from; a click on a candle selects its basket;
  - bond operations in history, the order of records within one second (`historyOrder.js`)
- **v1.1.6** — The Binance statement journal:
  - the Binance profile (BN + user id) as an account; the Strategy and Futures wallets are part of the portfolio;
  - the API key removed completely: movements come only from the statement, prices from public minute candles; the balance is computed from the minute series (`binance/series.go`, `ledger.go`)
- **v1.1.5** — Statement import instead of manual entry and API history:
  - Binance CSV (`binance/statement.go`): the time zone from the file name, movements that are not in the API (Strategy, Futures, staking, dust, cashback);
  - the ICU broker report (PDF, `pkg/icu`, shared `pkg/pdftext`): the account and journal reach the cache the same way as bank statements; manual accounts and the manual journal removed
- **v1.1.4** — Binance sync through the cache:
  - in the "Разведать" (Scout) panel a Binance client with stage strips (deposits, withdrawals, conversions, trades, minute prices);
  - walked periods are written to the cache at once, an interrupted sync continues from them; request limits (429/418) are survived by waiting;
  - wallet trades
- **v1.1.3** — Binance:
  - portfolio valuation in ₴ by the last snapshot (coin → USDT → ₴) and the "Портфель · Binance" (Portfolio · Binance) window;
  - minute history backwards: known movements rolled back from current balances, the "unexplained" part shown separately;
  - candle timeframes 1m / 5m / 15m / 1h / 4h / 1D / 1W
- **v1.1.2** — Bond payout markers and a forward time scale on the chart
- **v1.1.1** — The ICU broker account and repository tooling:
  - the account and journal in `core.db` (the balance is computed from the journal on read), reconciling records with cache operations;
  - OVDP bonds: ISIN, maturity, rate, face value; coupons every 182 days, the accrued coupon and the position value are computed on the fly; a coupon chart;
  - `clc.sh` — a bridge between the repository and the chat (the first version: patches, rollback, branches)

### Before the versioning workflow (3–10 November 2026, no number)
- **`3846612`** (10.11) — The link resolver, cleanup:
  - an alias drum modal (`AliasDrumModal`, `CardDrum`): the drum selects the alias's card, under it is its history (`TxResolveTable`); a card or an orphan PAN is dragged onto a counterparty row;
  - mirror hints removed;
  - cleanup of shared components (modals, alias chips, styles)
- **`5051188`** (10.11) — Re-linking an IBAN lifts the bans from its cards:
  - the "card → IBAN" link (`iban_pans` in `core.db`) is a fact about the account and survives a cache reset;
  - a network failure during the walk: the key's other accounts are not polled, the panel says "no connection with the bank"
- **`fa39e9d`** (10.11) — The sync panel, the source cache, the account lifecycle:
  - the "Разведать" panel (`progress.go`, `SyncPanel`, `SyncTimeline`): clients → accounts → a timeline of what was taken; coverage is stored in `cache.db` (`sync_coverage`);
  - the source cache: statement files on disk, freshness by the content's sha256, the folder is the card number mask; the "Загрузить" (Load) button parses them again;
  - the account lifecycle: opening — by the start of history, closing — the account vanished from the bank's response (`life.go`, `AccountLifeModal`), a manual edit takes priority;
  - sources: Raiffeisen (API and CSV in cp1251), PrivatBank PDF statements (checksum verification), an import modal;
  - two stores: `cache.db` (a disposable cache) and `core.db` (persons, aliases, links, manual edits), the "operation → person" resolver on read;
  - the IBAN/card link editor (`LinkEditorModal`), autoscroll while dragging, list jump buttons;
  - the interface kit: `UiButton`, `UiModal`, `UiIcon`, animations, toasts, a side menu with sections
- **`b14e5ac`** (09.11) — Persons as components: `PersonCard`, `OrphanTable`, `usePersons`
- **`0f64b4c`** (09.11) — Persons: cards with editing of name and avatar (paste from the clipboard), dragging aliases from the orphan table; cache reset (`/api/sync/wipe-cache`); the database split into `cache.db` and `core.db`
- **`c0708fd`** (09.11) — Vue architecture breakdown and persons: `App.vue` split into the tabs "Смета" (Smeta), "Операционная", "Лица" (Persons), `AppHeader`, `CandleChart`; on the backend persons, aliases, account binding, manual account details, automatic operation parsing
- **`ba4e536`** (03.11) — The first cut: the monobank client (client data, statement, waiting on the request limit), SQLite (accounts and operations), the API (accounts, operations, status, a full history walk, credit limit), Vue 3 + Vite + Tailwind: one page with a capital candlestick chart
