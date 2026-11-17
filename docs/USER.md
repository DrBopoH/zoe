**English** · [Русский](USER.ru.md) · [Українська](USER.uk.md)

# Ζωή — user guide

Ζωή shows all your capital as one picture: bank cards, a brokerage account with OVDP bonds, and Binance. Data comes from statements and bank APIs, and everything is stored on your computer.

> The interface is in Russian. On-screen labels are given as they appear in the app, with a translation in parentheses on first mention.

- [First launch](#first-launch)
- [How data gets into the app](#how-data-gets-into-the-app)
- [Home](#home)
- [The "Смета" (Smeta) page](#the-смета-smeta-page)
- [The "Рынок" (Market) page](#the-рынок-market-page)
- [Uchet → History](#uchet--history)
- [Uchet → Persons](#uchet--persons)
- [The candlestick chart](#the-candlestick-chart)
- [Operation badges](#operation-badges)
- [The "Выписки" (Statements) window](#the-выписки-statements-window)
- [Scout (API sync)](#scout-api-sync)
- [OVDP bonds and the brokerage account](#ovdp-bonds-and-the-brokerage-account)
- [FAQ](#faq)

## First launch

Start the backend and the frontend (see the [README](../README.md#quick-start)) and open `http://localhost:5173`. **Home** ("Главная") opens. In the header, on the left, is the side-menu button, next to it the Ζωή brand (a click returns to Home), and on the right the **РАЗВЕДАТЬ** (Scout) button. The side menu consists of the **Архив** (Archive) section with the pages **Смета** (Smeta), **Рынок** (Market) and **Учет** (Uchet, "accounting"; the old interface lives here until the design is rebuilt) and the **Управление** (Management) section (the "Выписки" window). In "Учет", on the left, behind the main grid, there is a list of sections (in a window from 1380px wide with labels, from 1240px as a column of icons, on a narrower one as a row on top): **История** (History: accounts, chart, operations) and **Лица** (Persons). The **РАЗВЕДАТЬ** button is in the header and in the "Выписки" window.

While there is no data, the pages are empty. There are two ways:

1. **By files:** Management → Cache ("Кэш") → **Выписки** (Statements) → **Добавить** (Add) and choose a statement file.
2. **By API:** put Monobank/Raiffeisen tokens into `backend/.env` and press **РАЗВЕДАТЬ**.

## How data gets into the app

| Source | How to get it | What to load |
|---|---|---|
| Monobank | API token | the РАЗВЕДАТЬ button (no CSV needed for Mono) |
| Raiffeisen | API token or CSV statement | РАЗВЕДАТЬ or a `.csv` file |
| PrivatBank | PDF card statement | a `.pdf` file |
| NovaPay | CSV statement | a `.csv` file |
| Binance | Wallet → Transaction History → Export (CSV) | a `.csv` file; coin prices come from Binance public data, no key needed |
| Binance P2P | Orders → P2P → Export (C2C order history) | a `.csv` file; supplements the statement of the same account, the "Обновить" (Update) mode of the Binance account |
| ICU broker (OVDP) | the trader's report ("Звіт торговця") as PDF | a `.pdf` file |

The app determines the file type itself. To parse a PDF, `pdftotext` (the poppler package) must be installed.

**File name = owner alias.** The name of the source file (for example "Ivan Ivanenko.csv") becomes the account's alias. A Person is later linked to the alias, so name files the way you want to see the owner. The name is set once, at the first import.

Binance: the time in the statement is read in the time zone from the file name (for example `(UTC+2)`). If the file is renamed and has no zone, the time is treated as UTC.

Binance P2P: in a regular statement a P2P purchase is visible only as "+88.85 USDT" on the Funding wallet, without the hryvnia amount. The hryvnia side (how much was paid, at what price, what fee) comes from a separate export of the order history. Load it as a regular `.csv` in the "Обновить" mode of the Binance account: it lands in the same account, does not change its name and profile, does not extend the time coverage and is stored as a separate `.zoe` file in the `binance` folder (in the "Выписки" window it is labeled "P2P"). The time is read in the zone from the file name. Orders are identified by number, so overlapping exports simply complement each other, and cancelled orders are kept with their status. After a cache reset ("Сброс кеша") the orders come back with the "Загрузить" (Load) button.

## Home

The page the Ζωή brand leads to: a capital tile and a candlestick chart across all accounts for all time. This is the beginning of the future interface; the rest is still in the "Archive".

## The "Смета" (Smeta) page

The overall picture, top to bottom: a header with **Капитал** (Capital) (in dollar equivalent: dollars as is, hryvnia, euro and Binance at the NBU rate; next to it, in small type, "(≈ ₴…)" is an approximate hryvnia equivalent; next to that the totals of the current month: income, expense and the result without transfers between your own accounts; until rates are loaded the amounts are in hryvnia — press "Обновить" on the "Курсы" (Rates) screen of the "Рынок" page); below, in one row, two blocks: on the left **Структура капитала** (Capital structure) (a ring by layers: cards and accounts, free money at the broker, OVDP bonds, crypto; the inner ring shows the share of hryvnia, dollar and euro; beside it a list with the amount and share of each layer and currency; on hovering a segment or a row the corresponding part is highlighted and its share is shown in the center) and on the right **Портфель ОВДП** (OVDP portfolio) as tiles (amount in bonds, average rate and time to maturity weighted by position value, real yield: the rate adjusted for year-on-year inflation, yield in dollars and real yield in dollars, the nearest payout in the title; positions without a rate or maturity in the ICU reference are not counted); below that, a candlestick chart across all accounts for all time. Transfers between your own accounts and internal movements are not included in the increment.

## The "Рынок" (Market) page

On the **stock market** charts (the "Биржа США" section, US stock exchange) there is a **Benner cycle** layer (1875): A — panic year, B — price peak, C — price bottom. A thin strip in the footer with letters (a dense color is the year itself, a pale one the period after it) and a column of the key year over the full height; on hover the kind of year is written under the chart. The other charts (rates, inflation, exchange rates, crypto, investment growth, capital candles) have no such layer: it only got in the way of reading there. This is a historical curiosity, not a signal: Benner himself forecast only until 1904, the rest was added later; it matches the stock market best (by feel, not by proof), and elsewhere it is noise or lag.

A table of bonds downloaded from public sources (a cache, nothing personal leaves). On the left, behind the grid, is a list of sections with groups (a click on a group title collapses it): **Ценные бумаги** (Securities) — **ОВДП** (the NBU registry and auctions) and **США** (US — Treasury auctions, CUSIP). In the section header: search by ISIN/CUSIP and name, a currency filter (for OVDP), the "hide matured" checkbox (on), and the count of securities. A click on a column header sorts (a second click changes the direction, empty values are always at the bottom). The **ОБНОВИТЬ** (Update) button polls only the selected source; progress is visible in the process window, the result as a toast. The **Коэффициент** (Coefficient) answers the question "which security grows capital best in the long run". The model: a 10-year horizon; a security holds its rate until maturity (coupons are reinvested at the same rate), and for the rest of the horizon the money is invested at the "base" rate, the lower quartile of the rates of active securities in the same currency (a cautious assumption that rates will fall over time). From this the average annual capital growth is obtained. The best security of a currency gets 100 %, the worst 0 %, the rest in between; the border color goes from red through yellow to green. The meaning: a high rate for a long term = the benefit is "locked in" for long, which is the best; a high rate for a short term is only slightly better than the base one, because you will have to reinvest into who knows what; a low rate for a long term is worse than the market, you lock in bad conditions. It is computed within one currency (rates in hryvnia, dollar and euro are incomparable), over the whole source list, so search and filters change neither color nor place. The security's rate: the auction yield, and if there is none, the coupon. Matured ones, ones without a maturity date and without a rate are not rated (a gray border). Simplifications: interest is annual compound, taxes and inflation are not taken into account; OVDP indexed to inflation with a low nominal percent look worse than they are in fact. Details (growth, term, base rate, place) are in the tooltip of the name. To the right of the name is a badge of the place in the currency ranking: **Топ 1** (gold), **Топ 10** (purple), **Топ 25** (light blue), **Топ 100** (gray). The value is also in the "Коэфф." column (you can sort by it). The **"Реал."** (Real) column is the yield adjusted for inflation in the currency of the security itself: hryvnia — NBU inflation, dollar — US, euro — eurozone (green — above inflation, red — below); it is present in any section where inflation is known. The columns **"В $"** (In $) and **"Реальная $"** (Real $) bring securities of any currency to the dollar: the dollar as is; a hryvnia one adjusted for the dollar's growth (a 5-year average per year, this is an assumption, not a forecast); a euro one for the euro's growth against the dollar; "Реальная $" subtracts US inflation as well (green — yield above inflation, red — below). US rates and inflation are needed from the "Курсы" and "Ставка ФРС" (Fed rate) screens (the "Обновить" button there). **Investment and chart.** The "вложение" (investment) field (10,000 by default, remembered in the browser) sets the amount: the "Через 10 л." (In 10 y.) column shows what it will grow into over the horizon, and the name's tooltip also has the amount at maturity. Above the table there is one chart: time horizontally (calendar years, the past on the left, a dashed vertical "today" and a second one "10 y."), the investment amount vertically. A line per security, its color is the border color (the coefficient). The line starts at today's amount and ends on the maturity day; if maturity is earlier than the 10-year horizon, a dashed line follows: the money lies at the base rate. The best five securities are thick, numbered 1–5 at the maturity point, with their legend under the chart; the others are thin in the background. The **mouse wheel** zooms in and out in time (the point under the cursor stays in place, the amount scale fits the best securities in the window), **dragging** sideways shifts in time, and up/down shifts the amount (within the extreme values); **Shift+wheel** scales the amount. While the amount has not been touched, it smoothly fits to the visible window; any movement in time brings this fitting back. A **double click** or "Сброс масштаба" (Reset zoom) returns the default window (up to 15 years ahead). **Только ICU** (ICU only) (the OVDP section): what can be bought at the broker, not the whole NBU registry. The "только ICU" checkbox counts places, "Топ 1", the chart and the gap only among the securities of your list; clearing it brings back the whole registry. The "список ICU" (ICU list) button opens a panel:

- **Screenshots.** Drop several screenshots of the ICU screen "Купити → ОВДП" (Buy → OVDP) into the area (or choose files, or paste with Ctrl+V). The program reads the ISIN, the displayed yield (YTM; SIM for short securities) and the price per security by itself, checks the ISIN against the NBU registry by maturity date and the price against the yield, and shows a table for review. The price can be corrected by hand, unneeded rows unchecked. "Дополнить список" (Add to the list) adds and updates, "заменить весь список" (replace the whole list) leaves only the securities from these screenshots. Recognition runs in the browser itself, the images go nowhere; the first time it starts in about 5 seconds (the model is loaded), then faster.
- **Yield curve.** If a security has no price (for example, it was cut off in the screenshot), its yield is estimated from the curve: it is built from neighboring securities with a known yield and from your "term in years → %" points (three to five points from the ICU screen). In the table such an estimate is marked "≈".
- **As text.** A fallback: paste "ISIN price" line by line.

The list, prices and curve are stored on the server (they survive a change of browser and cache clearing). The "Доходность" (Yield) column holds the yield by the ICU price: a price above par cuts the yield, and a coupon rate alone does not see this. The priority is: by price, then the YTM from the screen, then the estimate from the curve, then the registry rate. Securities already bought on the brokerage account have a green "N шт." (N pcs.) chip next to the name. In the bottom right corner of the chart, for each currency, it is shown right away by how many percent the 10-year result of Top 1 is above Top 5. Under the cursor you can see the date and the amounts of the best five on it. The **"сгоревшие на графике"** (matured on the chart) checkbox adds already matured securities as a gray dashed line from the issue date to maturity (growth at their rate); the issue date and the rate are needed, without them a security cannot be drawn. The chart is built for one currency, with the same filters and search as the table: with "ВСЕ" (ALL) the most numerous currency is taken (a note under the chart). If the table is empty, press "Обновить" or "Разведать". The data from here is substituted into the bond form by ISIN.

**NBU rate and inflation** are separate screens of the "Макро" (Macro) group (on the OVDP screen only a compact strip is above the growth chart: the discount rate ("облікова"), year-on-year inflation, the real rate; a click on it opens the screen). The charts show the whole data period: the rate as a step line, inflation as a polyline. The "Инфляция" (Inflation) screen also has the eurozone inflation chart (HICP, year to year, Eurostat, since 1997): the real yield of euro bonds is computed from it; "Обновить" pulls it together with the NBU data. Controls are as in the investment growth chart: the wheel zooms in time, dragging shifts (sideways in time, up and down along the scale), Shift+wheel scales the scale, a double click or "Сброс масштаба" (appears after zooming) returns the window; under the cursor, in the line under the chart, the values of the series on that date are visible. Single outliers in the data do not flatten the scale: it fits the main mass of points, and the outlier remains reachable by shifting. Data comes with the **ОБНОВИТЬ** button on the screen (or "Разведать"); until it is there, the block shows a hint. The OVDP table has a **"Реальная"** (Real) column: the security's yield in hryvnia ((1 + yield) / (1 + inflation) − 1, by the latest year-on-year inflation), green above zero, red below; you can sort by it, and for securities in foreign currency and without an estimate there is a dash.

### Macro sections

The "Макро" group in the left drawer. Each screen reads its own series and is updated by its own "Обновить" button (it polls only them), "Разведать" updates all sources at once. All data is public, no keys needed; the charts are controlled as described above (wheel, dragging, "Сброс масштаба"). Everywhere it is a picture, not a forecast and not advice. The "Ставка НБУ" (NBU rate) and "Инфляция" screens are described above.

### Rates

The "Курсы" section in the "Макро" group: official NBU rates of the dollar, euro and gold to the hryvnia (history since 1996), gold in dollars and a summary of "how much the dollar rose in price" and "what a hryvnia rate gives in dollars if the hryvnia keeps getting cheaper at the same pace". This is not a forecast. The "Обновить" button on this screen downloads only the rates. Below: a table of world currencies (rate, change over the year, rate, difference with the US rate), cross rates via the hryvnia, the dollar index and BIS real effective exchange rates.

### Countries

A country switcher and a card: the country's main indicators, one per line (the central bank rate, inflation, unemployment, GDP growth, 10-year yield, rate to the hryvnia, stock index, real exchange rate; what exactly is there depends on the country). A click on a line draws its chart. The registry has 22 entries: the USA, Ukraine, the eurozone, the United Kingdom, Switzerland, Japan, China, Russia, Egypt, Poland, Turkey, Norway, Sweden, Canada, South Korea, Australia, Brazil, Saudi Arabia, the UAE, Iran, India, Taiwan. For Ukraine the card also has NBU reserves and the money supply, and below it charts of inflation by product groups, producer prices and the balance of payments. The "Обновить" button in the card polls all sources of this country's indicators.

### Eurozone

ECB rates and €STR, the 2- and 10-year yield curve, inflation (headline and core), unemployment and GDP growth (ECB and Eurostat). Below is a comparison of member countries: Germany, France, Italy, Spain, the Netherlands (inflation, unemployment, 10-year yield, the spreads of Italy, France and Spain to Germany).

### Fed rate

The "Ставка ФРС" section in the "Макро" group: the US Fed rate, the yield of 10-year Treasuries and US inflation (FRED data), the real Fed rate and a comparison with Ukraine: whose real rate is higher. The "Обновить" button on this screen downloads only the US series.

### Treasury curve

Yields of US Treasury bonds by term (FRED): a snapshot of the curve from 1 month to 30 years today, a year and five years ago, a yield chart and the "10 years minus 3 months" spread with an inversion note.

### Real rates

What the market pays above inflation (the real TIPS yield), what inflation it prices in (breakeven and 5y5y), the term premium of 10-year Treasuries and the 30-year mortgage rate (Freddie Mac).

### US prices

Headline and core inflation, PCE (by which the Fed measures its 2 % target), producer prices and housing in CPI.

### US labor market

Monthly job growth, unemployment (official and U-6), jobless claims, job openings (JOLTS), wages versus inflation.

### US activity

Retail sales and industrial production (year on year), housing starts, house prices (Case–Shiller).

### US liquidity

How much money is in the system: net liquidity (the Fed balance sheet minus the Treasury account minus reverse repo), the M2 money supply and the SOFR rate.

### Credit and the dollar

What lenders demand for risk (high-yield and investment-grade bond spreads), the NFCI financial conditions, the broad dollar index and government debt to GDP.

### US economy

The "Экономика США" section in the "Макро" group (FRED data, the "Обновить" button polls only them): unemployment with the **Sahm rule** (the 3-month average minus the 12-month minimum; a rise of 0.5 pp and above has historically coincided with the start of a recession), real GDP growth quarter by quarter (two quarters of decline in a row are called a technical recession), the **yield curve** 10 years minus 2 years (below zero — "inverted", it has historically preceded recessions), the fear index **VIX** (up to 20 calm, 20–30 anxious, above 30 panic; the line is gray around 20, light blue above, amber when very low) and University of Michigan consumer sentiment (the line as a traffic light: the more optimistic people are, the greener; around 75 yellow, a red number below 65). The hints are how these indicators are read historically, not a forecast and not advice.

### Scorecard

A summary from already loaded series: the US economy regime by rules, a stress index, a traffic light of leading signals, real rates and carry of countries. This is a summary, not a forecast and not a trading signal: for each conclusion you can see what it was derived from.

### Commodities

Oil, gas, metals, grain, sugar and coffee (FRED, World Bank) and precious metals from NBU rates. The "$ / ₴" switch recalculates everything at the NBU dollar rate on the date of each point.

### US stock exchange

The "Биржа США" section in the "Макро" group (the "Обновить" button polls only it): the indices **S&P 500**, **NASDAQ Composite** and **Dow Jones** by weekly close (FRED data). In the S&P 500 header: the level, how far below the maximum and the change over the year. A separate **drawdown-from-peak** chart for all three: 0 % — the index is at the peak, −20 % — the "bear market" level. History: S&P 500 — since 1871 (the old part is Robert Shiller's monthly data, the monthly average, the last ~10 years the weekly close), NASDAQ — since 1971, Dow — about 10 years; the "maximum" is computed over the available history. Under each chart is the Benner cycle. This is a picture of the market, not a forecast and not advice.

### World equities

Indices by country (monthly, 2015 = 100) and an assessment of the US market: the Buffett indicator, CAPE and the S&P 500 dividend yield. CAPE and dividends are taken from Shiller's table, which has not been updated since 2023: this is history, not a current assessment.

### Company stocks

A list of US companies by capitalization (Nasdaq and SEC) with search by ticker and name. For a selected company: the price (Yahoo, monthly), capitalization over time (the price times the number of shares from SEC filings, adjusted for splits), annual revenue and profit, equity and assets, the ratios P/E, P/S and P/B. Price histories and filings are downloaded for the top 500 with the "Обновить" button, other companies on first opening.

### Crypto

The "Крипто" section in the "Макро" group (the "Обновить" button polls only it): weekly prices of Bitcoin, Ethereum, BNB and Solana in dollars (public Binance candles, the pair to USDT), **drawdown from the all-time high** (0 % — the price is at the peak, −50 % — half as low), the crypto market **fear and greed** index (the line is colored by value: at 50 it is semi-transparent gray, bluer toward fear, more amber toward greed; up to 25 — extreme fear, above 75 — extreme greed). In the Bitcoin header: the current price, how far below the maximum and the change over the year. This is a picture of the market, not a forecast and not advice.

### Crypto loop

Everything on the path "bank → P2P USDT → trading → withdrawal": the USDT price for hryvnia on Binance P2P and its spread, the overpayment relative to the NBU rate, network transfer fees, funding and open interest of futures, stablecoin issuance and the Bitcoin share. P2P prices and fees are snapshots: the backend takes them itself once an hour (configured with `MARKET_EVERY`) and on "Обновить", the history accumulates and is not erased.

### Purchasing power

What happens to a country's money over the chosen term (cumulative inflation), how the real exchange rate to the dollar changed, how many times prices in the country are cheaper or more expensive than American ones (PPP), the Big Mac Index, a comparison of two countries and the real yield. Data: the World Bank (annual), FRED (US prices monthly), The Economist (Big Mac); the whole history of a series, log scale.

### Sources

The health of each source: what it promises, what is in the cache, when it was updated, which series are stale, the last error. Below is a catalog of all series with search and CSV export. The button in the header polls all sources, the button in a row only one.

## Uchet → History

- **Connected accounts** ("Подключенные счета") — tiles of cards and accounts. A click on a tile selects a card (several are possible), a second click clears the selection. The tile color depends on the bank, a card shows the balance and the opening date ("с 09.2026", since 09.2026).
- **The chart** shows the selected cards; if nothing is selected, all accounts. A click on a candle narrows the history to its interval.
- **Operation history** — a table for the selected cards: date and time, operation, counterparty account, amount. At the top are totals: income, expense, net. The "Снять выбор" (Clear selection) button and the cross on the candle date plate clear the filters. A long history is loaded with the "Показать ещё" (Show more) button. When a candle is selected, in the table header, to the left of income and expense, there are **Капитал** (Capital) (the change over the period, close minus open, with a percentage of the open) and **Размах** (Range) (maximum minus minimum within the period, i.e. intraday volatility); on hover the open, close, min, max and the Binance contribution are visible. This shows that a candle moved even without operations: coin rates change capital by themselves.
- For a "non-bank" account (ICU, Binance) the card shows the composition: bonds with accrued coupon or Binance wallets by coins; the Binance balance is shown in dollars.

If a counterparty has a linked Person, the history shows its name and avatar instead of the raw name from the statement.

## Uchet → Persons

This is where the "who belongs to whom" graph is assembled. Basic concepts:

- **Alias** — a name as the bank writes it: the account owner's name or a counterparty's name.
- **Person** — a human (name and avatar) to whom aliases are linked.
- **Orphan** ("Бесхоз") — an alias that is not yet linked to anyone.
- **An account and a card in an alias:** green — confirmed by cache data, yellow — remembered manually and not confirmed by the cache.

What you can do:

- Drag an **orphan** onto a Person's card to link the alias.
- Click an IBAN or a card in a Person's card (or the icon in the "Бесхоз" row) to open the **link editor**: add or remove an IBAN and a card, create a new Person. Cards without an IBAN can be linked to the IBAN of this alias.
- Click an alias chip on a Person's card to open the **drum** (resolver). The drum scrolls through the cards of the alias itself that have a statement and shows the operations of the selected card. On the right are all cards and "orphan PANs": drag a card onto a counterparty row, and it will be linked to that counterparty's alias. This is needed when the bank did not show the recipient's number.
- A Person's card: edit the name and avatar (a URL can be pasted from the clipboard), set the account opening and closing dates, delete the Person. Persons' cards can be dragged; the card order is remembered in the browser.
- The hidden view of an IBAN: the number is shown with a flag and a bank icon; you can set your own account name.

Links are stored in the permanent database and **survive a cache reset**.

## The candlestick chart

- **Timeframes:** `1m`, `5m`, `15m`, `1h`, `4h`, `1D`, `1W`. Intraday windows are limited (there are too many minute candles): when scrolling left, more history is loaded. Daily and weekly show the whole history.
- **Mouse wheel** — time zoom (around the cursor), **Shift+wheel** — price scale zoom, **dragging** sideways — time scroll (after a throw the chart coasts by inertia), up and down — shifting the price scale. A double click or "Сброс масштаба" (appears after zooming) returns the window.
- **Log scale** — a switch in the corner of macro charts with positive values (prices, rates, gold): on a logarithmic scale equal distances are equal percentages of growth, so charts spanning decades and centuries do not get flattened. On stock and crypto prices it is on right away, on rates, inflation and drawdown it is unavailable (they have zeros and negatives).
- The price scale on the right fits the visible part; the tag with the last price is shown in green or red.
- **A click on a candle** selects it; the history below shows only the operations of this interval.
- On the 1D timeframe the chart continues forward to the last bond payout (markers of coupons and maturity, without candles).
- The selected timeframe is remembered in the browser.

## Operation badges

In history rows the kind of operation is shown by a tag, not words:

- **Binance:** `ИНВЕСТИЦИЯ` (investment) + a wallet (`SPOT`, `FUNDING`, `STRATEGY`, `FUTURES`) for deposits, P2P, withdrawals and bonuses; `EARN` for accruals; `SPOT → EARN` and similar for transfers between wallets; `ПОКУПКА`/`ПРОДАЖА` (buy/sell) for trades; `ОБМЕН` (swap), `ПЫЛЬ` (dust), `СТЕЙКИНГ` (staking) for conversions.
- **Transfers between your own accounts** are marked with the account's bank (for example `BLACK`) and count neither as income nor as expense.
- **Bonds:** purchase, sale, coupons and maturity are shown as reference rows, they do not affect capital (money moves into bonds and back).

Operations within a single day at the broker (there is no time in the report) are ordered logically: leaving a bank account, crediting the brokerage account, purchase, sale, coupon, maturity, withdrawal.

## The "Выписки" (Statements) window

Opens from the menu: Management → Cache → **Выписки**. Here is everything known about sources and whether they are in the cache database.

For each account you can see: whether it is in the cache (`● в кэше · N` — in cache, or `○ нет в кэше` — not in cache), how many sources and records, and a **coverage strip** over time (filled — the period is taken). Expanding "Источники" (Sources) shows individual files: type (CSV, PDF, report, Binance, API), period, number of records, creation date, a delete button.

Buttons:

| Button | What it does |
|---|---|
| **Добавить** (Add) | a new account from a file (`.csv` or `.pdf`, by dragging or clicking) |
| **Обновить** (Update) | in an account row: add a statement to exactly this account |
| **Загрузить** (Load) | put source files into the cache database (all accounts or the checked ones) |
| **Сбросить** (Reset) | clear the cache database (all accounts or the checked ones); source files and Persons are not touched |
| **Разведать** (Scout) | same as the header button: a walk through banks over API (all accounts) |
| the **дописывать в источники** (append to sources) checkbox | if on (the default), after a walk what was taken from the API is written into the source files by itself: after a reset the cache is restored from disk, and the taken periods are not downloaded again; the choice is remembered in the browser |

If no accounts are selected, the action applies to all. The number of selected accounts is shown in brackets on the button.

### File check on adding

Before writing, the file is parsed and compared with what already exists. The type, account, alias, period, number of records and how the file lays over existing operations (matched, new, missing from the file) are shown. Possible warnings:

- **Такой источник уже есть** (Such a source already exists) — the same data is already loaded, the file name does not matter. It cannot be written.
- **Файл относится к другому счёту** (The file belongs to another account) — with "Обновить" this is a refusal; for a new account use "Добавить".
- **Счёт уже есть** (The account already exists) — "Добавить" creates only new accounts, tick "Обновить существующий счёт этим файлом" (Update the existing account with this file).
- **Другой алиас** (Another alias) — the account already has an alias and the file is named differently: choose which name to keep.
- **Нет общих операций в общем периоде** (No common operations in the common period) — perhaps this is the wrong statement; you can confirm "Всё равно записать" (Write anyway).

Re-importing the same statement creates no duplicates.

### `.zoe` source files

Each import is saved as a `.zoe` file (with already parsed data inside) in the folder `~/.local/share/economic/sources` (changed with the `ECONOMIC_SOURCES_DIR` variable, the current folder is shown at the bottom of the window). Inside, the folder is laid out by card number. This is exactly why the cache can be reset and brought back with the "Загрузить" button. Next to it, in the "Экономика" (Economy) group, are files with macro series and stock data: if the cache is empty, the backend builds it from them at startup, and the "Загрузить" button does the same manually. It is worth keeping this folder as a backup: it holds your financial data.

## Scout (API sync)

The **РАЗВЕДАТЬ** (Scout) button (the header and the "Выписки" window) starts a walk through banks over API. It:

- polls all keys in parallel, within a key it keeps a pause between requests (Monobank — about a minute, Raiffeisen — 30 seconds, these are the banks' limits);
- takes the history of each account in windows from "now" backward only where it is not there yet; a re-run downloads only the tail;
- for Binance, builds a portfolio estimate by the minute from the loaded statement and public prices (long, 10–20 minutes);
- if the "дописывать в источники" checkbox is on (the "Выписки" window), at the end it writes what was taken into the source files by itself; in the final push "Снято" (Taken) you can see how many sources were updated;
- shows a progress panel: keys, accounts, strips of taken periods, an estimate of the remaining time;
- in parallel, as a separate process, updates public market data (OVDP and US Treasury bonds, rates, exchange rates, macro, stocks). While it runs, a separate "Рыночные данные" (Market data) window hangs at the bottom right with progress per source (a percentage or a running bar); afterwards — a toast per source ("ОВДП · НБУ — бумаг в кеше: 184" — securities in cache: 184 — or a red one with an error). A failed source does not hinder the others; no keys are needed for it.

On a network break the walk retries and then shows the "Нет ответа от бэкенда" (No response from the backend) plate without losing data. If there are no tokens and Binance is not loaded, the status is "ОТСУТСТВУЮТ КЛЮЧИ API" (API KEYS MISSING).

## OVDP bonds and the brokerage account

The ICU broker report contains neither rates nor maturity dates of bonds, so there is a **bond reference** (the editor opens from the link editor of the brokerage account, the "+ ОВДП" button): ISIN, name, maturity date, annual rate in percent, par value (1000 ₴ by default). The coupon is paid every 182 days, the dates are counted backward from maturity. When you enter an ISIN (12 characters), the **maturity** and **rate** fields (and the name) are filled in by themselves from the NBU registry, if it has already been downloaded with the "Разведать" button; check and save. If it is not in the cache — a hint and manual entry. If there is no entry of your own in the reference at all, coupons and accrued interest are computed by the terms from the registry, and your own entry always takes precedence. Accrued coupon (ACI), the value of positions and future payouts are computed on the fly from the journal and the reference.

A report whose operations do not agree with the balances is not imported: the report's summary totals serve as control. An unknown kind of operation gives an explicit error, not a guess.

## FAQ

**Data disappeared after a reset.** Press "Загрузить" in the "Выписки" window: the cache will be restored from source files. Persons and links are not touched by a reset.

**"Не удалось разобрать PDF" (Could not parse the PDF).** Check that `pdftotext` is installed and that it is a text Privat statement, not a scan.

**The same person under different names.** Link both aliases to one Person.

**The recipient's card was not determined.** Open the alias drum (the alias chip on a Person's card) and drag your card onto the counterparty row.

**I want to open the app from my phone.** Run the backend with `HOST=0.0.0.0` and add the frontend address to `ALLOWED_ORIGINS` (see the README). The frontend's host name must be in `ALLOWED_ORIGINS`, otherwise the backend answers 403 "Host not allowed". Remember that this is personal finance: do it only on a trusted network.

**Where my data is.** Two databases `cache.db` and `core.db` next to the backend, `.zoe` files in the sources folder (including macro series and stocks). They do not get into git.
