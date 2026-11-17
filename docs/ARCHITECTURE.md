**English** · [Русский](ARCHITECTURE.ru.md) · [Українська](ARCHITECTURE.uk.md)

# Ζωή — technical documentation

A document for the developer (and for Claude): how the project is built, where things are, which rules must not be broken. The description matches the code as of build 2 (`v1.2.113`). If the code has diverged from the document, the code is right: fix the document.

> The interface is in Russian; on-screen labels are quoted as they appear in the app, with a translation where it matters.

- [1. Principles](#1-principles)
- [2. Repository map](#2-repository-map)
- [3. Backend](#3-backend)
- [4. Data and databases](#4-data-and-databases)
- [5. Sources and import](#5-sources-and-import)
- [6. API sync](#6-api-sync)
- [7. Persons, aliases, links](#7-persons-aliases-links)
- [8. The ICU broker and bonds](#8-the-icu-broker-and-bonds)
- [9. Binance](#9-binance)
- [10. Frontend](#10-frontend)
- [11. Tests and CI](#11-tests-and-ci)
- [12. Invariants and pitfalls](#12-invariants-and-pitfalls)
- [13. Recipes: how to add](#13-recipes-how-to-add)

## 1. Principles

1. **Local and single-user.** The backend listens on `127.0.0.1`, checks the Host header (protection against DNS rebinding), CORS lets through only the listed Origins; the texts of internal errors are returned to the client (this is deliberate: the app is not public).
2. **Two databases of different value.** `cache.db` is disposable and replenishable, `core.db` holds what must not be lost (persons, aliases, links, manual edits). Parsers and sync write only to the cache. The only "cache → static" path is `syncAliases` (see [§7](#7-persons-aliases-links)).
3. **The source of truth is outside the database.** Everything imported also lies in `.zoe` files; a cache reset loses no data (§5).
4. **Idempotency.** Re-importing the same or an overlapping statement creates no duplicates: operation IDs are stable (`stmtkit.StableID`).
5. **Computed on read.** The ICU balance, accrued coupon, Binance operation kind, "own account" and `resolved_user_id` are not written to the database but computed on the fly by the current rules.
6. **An explicit error instead of a guess.** An unknown ICU operation kind, a non-empty corporate-actions section, an unclear file give an import error, not a heuristic.
7. **One mechanism per job.** Before new code check whether such a path already exists (HTTP helpers, the bank registry, `usePaged`, `stmtkit`).

## 2. Repository map

```
backend/
  main.go                 startup: env → Storage → Server → http.Server, graceful shutdown
  pkg/api/                HTTP: routes, handlers, sync, import, progress
  pkg/db/                 storage (two SQLite databases) and all domain read logic
  pkg/source/             the .zoe source file format
  pkg/stmtkit/            shared by statement parsers: stable IDs, kopecks, Kyiv time
  pkg/mono/  pkg/raif/    bank API clients (raif also has a CSV parser)
  pkg/privat/             PrivatBank PDF statement parser
  pkg/novapay/            NovaPay CSV parser
  pkg/icu/                ICU broker report parser (PDF)
  pkg/binance/            Binance statement, journal, portfolio valuation, public prices
  pkg/market/             public data, read-only: OVDP bonds (NBU) and US Treasury (FiscalData), macro series (NBU, FRED, ECB and Eurostat, BIS, World Bank, Yahoo), country cards, company stocks (Nasdaq, SEC), the crypto loop
  pkg/pdftext/            a wrapper over pdftotext (with a 30 s timeout)
frontend/
  src/App.vue             shell: header, side menu, pages, backend polling, data loading
  src/views/              Home, Smeta, Market (bonds and the `Macro*` macro screens), Uchet (History, Persons)
  src/components/         panels, modals, tables, chart; ui/ — small building blocks
  src/composables/        state and logic (useCapitalChart, usePersons, usePaged, …)
  src/utils/              pure functions: formatters, alias graph, bank registry, API layer
  tests/                  node --test (logic, `*.test.mjs`) and vitest (components, `tests/components`)
.github/workflows/ci.yml  CI
clc.sh                    git/patches (see WORKFLOW.md)
```

Not committed: `cache.db*`, `core.db*`, `.env`, `*.zoe`, statements, archives, `.claude`, `.commits/`, `node_modules`, `dist`. CI checks part of this itself (the `hygiene` job).

## 3. Backend

### 3.1 Startup (`main.go`)

`godotenv.Load()` (optional) → bank tokens from `MONO_TOKENS`/`MONO_TOKEN`, `RAIF_TOKENS`/`RAIF_TOKEN` via `api.ClientsFromEnv` → `db.NewStorage(cacheDBPath, coreDBPath)` → `api.NewServer(storage, monoClients, raifClients)` → `SetBinance(binance.NewClient())` → `SetMarket(market.NewClient())` → `RestoreEconomy` and `RestoreStocks` (an empty series or stocks cache is rebuilt from the files of the "Экономика" (Economy) group) → `RunMarketSchedule` (crypto-loop snapshots every `MARKET_EVERY`, an hour by default, `0` turns it off) → `ALLOWED_ORIGINS` → `http.Server` (`ReadHeaderTimeout` 10 s, `ReadTimeout` 2 min for a statement upload, `IdleTimeout` 2 min). On Ctrl+C/SIGTERM the server waits for requests (10 s) and closes both databases (WAL flush). Environment variables: see the [README](../README.md#environment-settings-backend).

### 3.2 HTTP layer (`pkg/api`)

- `routes.go` — **the single route table**; CORS (`EnableCORS`) is attached once to the whole mux, so a new route cannot be "forgotten" unprotected.
- `httpx.go` — common responses: `writeJSON`, `writeOK` (`{"status":"success"}`), `writeMsg`, `fail`, `failErr` (500, the text in the response), `readJSON` (body limit 1 MB, answers 400 by itself on error), `allow` (method check → 405). Statement bodies are read by their own handlers (limit 32 MB).
- Host: `EnableCORS` first of all checks the Host header (`hostAllowed`). `localhost`, `127.0.0.1`, `::1` and the hosts from `ALLOWED_ORIGINS` count as ours; a foreign name bound to 127.0.0.1 (DNS rebinding) gets 403 "Host not allowed" even without an Origin. Tests go to `127.0.0.1`, not to `example.com`, which `httptest` sets by default.
- CORS: a request **without** `Origin` is let through (curl, proxies); a request with an unknown Origin gets 403.
- gzip: `gzipJSON` compresses JSON in responses to GET if the client accepts gzip (fast level, the compressor pool `gzPool`).

Routes (all under `/api`):

| Group | Route | Purpose |
|---|---|---|
| Accounts | `/accounts` | list of accounts (with a portfolio for ICU/Binance) |
| | `/accounts/life` | manual opening/closing dates |
| | `/accounts/update`, `/accounts/credit-limit` | manual IBAN/PAN/limit edits |
| | `/transactions` | operations |
| ICU | `/icu/ledger` | journal, positions, payouts (read) |
| | `/icu/bonds` | bond reference (GET, POST create/update, DELETE `?isin=`) |
| Market | `/market/lists?name=icu` | own ISIN list (GET → `{name, isins, prices, ytms}`, PUT `{isins, prices, ytms}` replaces it whole; ISINs uppercase, no repeats; `prices` — an optional per-security price from the broker, `ytms` — the yield shown by the broker, % (effective annual); stored by token `ISIN`, `ISIN=price`, `ISIN~ytm` or `ISIN=price~ytm`); lives in `core.db` (`market_lists`), "wipe cache" does not touch it |
| Market | `/market/curve?name=icu` | the market yield curve set by the user (GET → `{name, points:[{years, ytm}]}`, PUT `{points}` replaces it whole; term 0–50 years, yield 0–100 %, points by term); `core.db`, table `market_curves` |
| Market | `/market/bonds`, `/market/sync` | downloaded OVDP and US Treasury (GET, `?source=&q=`); POST — update from public APIs (`?source=` one or several sources comma-separated, without the parameter — all; valid ids are checked by `market.IsSource`, the list is given by `GET /macro/catalog`) |
| Market | `/macro` (the US stock exchange, source `stocks`, `market/stocks.go`: `stock_sp500|nasdaq|dow` — FRED `SP500`, `NASDAQCOM`, `DJIA` through the shared `fred()` and `weekly()` (the last close of an ISO week); for SP500 and DJIA FRED gives ~10 years, for NASDAQCOM since 1971; the SP500 history before the FRED window is filled in by `shillerSP500()` — a monthly CSV `raw.githubusercontent.com/datasets/s-and-p-500/main/data/data.csv` (`ShillerBase`, columns Date, SP500; zero values are skipped), `stitchHistory` takes from it only points earlier than the first weekly one; a history failure does not kill FRED; a failure of one index does not kill the others; Stooq is closed by a JS bot check (we do not bypass it); drawdown `drawdown`/`athInfo` on the frontend; the screen `MacroStocksView.vue`, series of the `stocks` source; Dow has no full history); (crypto, source `crypto`, `market/crypto.go`, `BinanceBase`/`FNGBase`: `crypto_btc|eth|bnb|sol` — weekly close of Binance candles `GET /api/v3/klines?symbol=BTCUSDT&interval=1w&limit=1000` (arrays `[time ms, open, high, low, close, …]`, numbers as strings), `crypto_fng` — `GET api.alternative.me/fng/?limit=0&format=json` (`data[].value`, `data[].timestamp`, the last value of the week); tolerant parsing (a number as a string or as a number); a failure of one coin does not kill the others; drawdown `drawdown` and `athInfo` are computed on the frontend; the screen `MacroCryptoView.vue`, series of the `crypto` source; the response format has NOT been verified by a live curl from the sandbox — check: `curl -s 'https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=1w&limit=2'` and `curl -s 'https://api.alternative.me/fng/?limit=2&format=json'`); (US economy from FRED, source `usecon`, `market/usecon.go`: `us_unrate` (monthly), `us_gdp` (real GDP growth, quarterly, % annualized), `us_spread` (T10Y2Y, the last value of the week), `us_vix` (the last value of the week), `us_sentiment` (UMCSENT); the same `c.fred`, a failure of one series does not kill the others; the screen `MacroUsEconView.vue`, series of the `usecon` source are read only on it; pure signals `sahmRule`, `technicalRecession`, `curveState`, `vixLevel` in `utils/macro.js`); (the US from FRED, source `us`, `market/us.go`, `FREDBase`: `fed_funds` (effective rate, monthly), `fed_target` (the top of the corridor, only change dates), `ust_10y` (the last value of the week), `us_cpi_yoy`, `us_cpi_mom` (computed from the CPIAUCSL index); the "." gap in the CSV is skipped; a failure of one series does not kill the others); (exchange rates: `usd_uah`, `eur_uah`, `xau_uah` — daily, hryvnias per unit, gold per troy ounce; source `fx`, `market/fx.go`; before 2.9.1996 the NBU gives karbovanets, divided by 100000; request rates separately via `?series=`: these are thousands of points) | NBU macro series from the cache (GET `?series=policy_rate,cpi_yoy`; empty — all): `{series:{id:[{d,v}]}, updated_at}`; the series `policy_rate`, `overnight_cd`, `overnight_loans` (only the dates of value change: valid until the next point), `cpi_yoy`, `cpi_mom`, `core_yoy`, `core_mom` (monthly, `d` is the first of the month) |
| Market | `/market/companies` | GET `?q=&limit=`: companies for the "Акции компаний" (Company stocks) section (Nasdaq and SEC), the largest by capitalization first; `q` — part of a ticker or name |
| Market | `/market/stock` | GET `?ticker=AAPL`: a company, its monthly prices, splits and capitalization by month; if there is no history or it is older than a day, it is pulled from Yahoo at once, a ticker not in the list — 404 |
| Market | `/macro/catalog` | which series each source provides (`feeds`) and the country registry (`countries`: which series is responsible for which indicator of the card) |
| Market | `/macro/ticks` | GET `?series=p2p_ask&since=unix`: hourly points of the crypto-loop snapshot series `[{t,v}]` |
| Market | `/macro/sources` | source health: what each promises, what is in the cache, when it was updated, which series are stale, the last error |
| Binance | `/binance` | what the cache remembers: a breakdown by coins and the limits of the minute series (it polls nothing itself) |
| | `/binance/candles` | POST: portfolio valuation candles by time buckets |
| | `/binance/p2p` | C2C order history (GET), empty until an export is loaded. The time is aligned with the journal: `binance.DetectShift` checks completed purchases against "P2P Trading" credits of the same amount (within an hour after order creation) and finds a shift that is a multiple of 15 minutes, if two files were read in different zones (the file name remains the main source of the zone: the shift is looked for only when it really improves the match, at least 2 pairs). The shift is in the `X-Time-Shift-Minutes` header; the source file and the database do not change |
| | `/binance/moves`, `/binance/trades`, `/binance/swaps` | transfers between wallets, trades, swaps for the history |
| Sync | `/sync/status` | state, bank progress, the market process (`market`), `stats.data_version` |
| | `/sync/full[?save=1]` | POST: start "Разведать" (Scout) (409 if already running); `save=1` — after the walk write what was taken into source files |
| | `/sync/wipe-cache` | POST: stop the walk and clear the cache |
| | `/sync/import-csv` | POST multipart: import of a statement file |
| Sources | `/sources` | inventory of sources with coverage strips |
| | `/sources/inspect` | POST multipart: file check, writes nothing |
| | `/sources/load`, `/sources/reset`, `/sources/save-api` | load into the cache / reset the cache / save an API snapshot |
| | `/sources/file` | DELETE: delete a source file |
| Persons | `/users` | Persons (GET/POST/PUT/DELETE) |
| | `/users/alias` | POST link an alias, DELETE unlink (`?alias=`) |
| | `/users/link-account` | POST link an account or card, DELETE unlink (`?identifier=`) |
| | `/alias-structures` | remembered alias structures, including orphan ones |
| | `/iban-masks` | GET hidden IBAN views; POST/PUT set your own account name (an empty value returns the automatic mask) |

### 3.3 Data version

`Storage.Version()` grows on any write (cache or static). It is served in `/sync/status` as `stats.data_version`. The frontend polls the light status every 2 seconds and reloads heavy data only when the version changes.

## 4. Data and databases

SQLite (the `modernc.org/sqlite` driver, no CGO), opened with `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=10000`, `foreign_keys=1` via DSN (applied to every pool connection). Writes are serialized by the mutexes `cacheMu` and `coreMu`.

### cache.db — the disposable cache

| Table | Contents |
|---|---|
| `accounts` | accounts: id, name, balance, type, currency, IBAN, PAN mask |
| `transactions` | operations: time (unix), description, counterparty (name, IBAN, EDRPOU), MCC, amounts, fee, cashback, `is_hold` |
| `sync_coverage` | taken time segments per account (`From == HistoryStart` means "history taken down to the bottom") |
| `icu_flows` | the ICU broker report journal |
| `market_bonds` | bonds from public sources: `source` (`nbu`/`ust`), `id` (ISIN/CUSIP), maturity, rate, par value, yield and the date of the last auction |
| `binance_ledger` | the Binance statement journal, a row as is |
| `binance_p2p` | Binance C2C order history (order number, side, fiat amount, price, quantity, fee, status, time), idempotent by order number |
| `binance_minutes` | Binance portfolio valuation by the minute, ₴ |
| `macro_series` | the macro series cache: `series`, `dt` (YYYY-MM-DD), `value`, `updated_at`; key `(series, dt)`, a `WITHOUT ROWID` table (clustered, series reads twice as fast); the old cache is migrated by `migrateMacroClustered` without losing series; restored from the files of the "Экономика" group |
| `market_companies` | US companies (Nasdaq and SEC): ticker, name, exchange, CIK, sector, capitalization, price |
| `stock_prices`, `stock_splits`, `stock_split_known` | monthly prices (close and with dividends), splits, the "ticker splits downloaded" mark |
| `stock_facts`, `stock_frames`, `stock_fetched` | SEC filing facts (frames like `CY2024Q4I`), which frames were downloaded and when, when the ticker history was downloaded |
| `binance_state` | service keys (the state of the last "Scout") |

### core.db — permanent data

| Table | Contents |
|---|---|
| `users` | Persons: id, name, role, avatar |
| `user_aliases` | alias → Person (`alias_key = lower(trim(alias))`, the key is computed by Go so Unicode does not diverge from SQLite) |
| `alias_accounts` | the remembered structure of an alias: alias → IBAN → PAN (lives for orphan aliases too) |
| `alias_bans` | shadow bans: what the cache must not pick up for a linked alias |
| `iban_pans` | the global fact "this card belongs to this IBAN" |
| `iban_masks` | the hidden view of an IBAN; `custom=1` means your own account alias |
| `account_overrides` | manual account edits: IBAN, PAN, credit limit, opening and closing dates |
| `bonds` | bond reference (ISIN, maturity, rate, par value) |
| `market_lists` | own market security lists: `name` (`icu`), `isins` — tokens `ISIN[=price][~ytm]` separated by spaces, `updated_at` |
| `market_curves` | the own market yield curve: `name`, `points` (JSON `[{years, ytm}]`), `updated_at` |

`initCore` can migrate the old schema (`user_accounts`, aliases without `alias_key`).

### Account life cycle

Bank APIs do not give opening and closing dates. Opening = the oldest operation, but only when the history has been taken in full. Closing = the account disappeared from the bank's response (the date is the last operation). A manual date in `account_overrides` takes precedence over the statement; there can be no operations after the closing date.

## 5. Sources and import

### The `.zoe` format (`pkg/source`)

One file — one import (or API snapshot) of one account for one period. Inside is **already parsed data**, not the original PDF/CSV: "Загрузить" (Load) does not parse again but puts the ready data into the database. Fields: `Name` (the source file name = alias, set once), `Created`, `Hash` (a fingerprint of the content without `Name`/`Created`/`Origin`; the same data under another name is a duplicate), `Kind` (`bank`, `binance`, `binance-p2p`, `icu`, `binance-series`, `dataset`), `Group` (empty or `accounts` — "Счета" (Accounts); `economy` — "Экономика" (Economy)), `Origin` (`csv`, `pdf`, `report`, `binance`, `api`), `AccountID`, `Card` (the folder key), the period (`From`/`To`), `Spans` (segments already taken from the API: after a source is loaded, sync does not poll them), plus data: `Txs` (banks), `Rows` (Binance), `Flows` (ICU), `Series` (the Binance minute series). The "Экономика" group keeps series tables (`dataset`: macro series and stock data) without an account: a file is identified by a set (`Card`) and the set name inside; the cache is rebuilt from them (`RestoreEconomy`, `RestoreStocks`, the "Загрузить" button). A file is written atomically (a temp file, then rename) with permissions `0600`; JSON larger than 1 MB is gzip-compressed (both kinds are read), unpacking is limited to 256 MB (`maxUnpacked`, protection against a gzip bomb). The sources folder is split by card number (`516875******8629`, not by IBAN: one IBAN can carry two PANs); `ECONOMIC_SOURCES_DIR` overrides the path.

### Import pipeline

1. `POST /sources/inspect` (multipart) → `parseSource` determines the type by content: `binance.IsStatement` → Binance; `binance.IsP2P` → C2C order history (kind `binance-p2p`, the same `binance` account, without a profile: it has no user identifier); `icu.IsReport` → ICU; PDF → Privat; `novapay.IsStatement` → NovaPay; otherwise Raiffeisen CSV. Nothing is written. The response describes the account, the period, the overlap with what exists and a list of `problems`: `other_account`, `exists`, `duplicate` (block), `alias`, `no_match` (require a decision).
2. `POST /sync/import-csv` applies the user's decisions (`mode`, `account`, `alias`, `force`) via `enforceImportRules`, saves the `.zoe` file and calls `applySource` — **the single path** for both import and "Загрузить".
3. `applySource` writes to the cache and updates the static data via `syncAliases`.

`migrateLegacy` converts old "raw" files of the source cache into `.zoe`; originals go to `_legacy`, what could not be parsed stays and goes into the report.

### Identifiers

- A bank account: `<type>_<IBAN>`, and without an IBAN `<type>_<card>`.
- ICU: `icu_<digits of the contract number>` (`db.ICUAccountID`): the report has no IBAN, the contract is the only stable key.
- Binance: `BN<user id>`; the profile is taken from the statement.
- Operations: `stmtkit.StableID(prefix, key, seen)`. The file row number is **not part** of the key (a row shift between exports would create duplicates); identical operations within a minute differ by a sequence suffix. The key format must not be changed: a re-import would create duplicates.

### Parsers

| Package | Input | Notes |
|---|---|---|
| `privat` | PDF (via `pdftext`) | the operation amount is taken as in the card column; fees are not recalculated |
| `raif` | CSV | do not change the fee semantics |
| `novapay` | CSV | the full name, tax ID and passport from the header are not read; the account, card, balance are taken |
| `icu` | PDF (text layer) | the totals of section 1 serve as control sums; a mismatch = import refusal |
| `binance` | statement CSV | all rows are read as is; the meaning of a row is decided by `ledger.go`; the time zone from the file name |

Time in statements is Kyiv time (`stmtkit.Kyiv`), independent of the server's zone. Parsers and chart math must not depend on the machine's time zone; when editing time code, also run the tests under `TZ=America/New_York` and `TZ=Asia/Tokyo`.

## 6. API sync

"Разведать" (`POST /sync/full[?save=1]`) starts `runFullCrawler`: each Monobank and Raiffeisen key is walked in parallel (`crawlBank` with `bankSpec`), then, if a Binance statement exists, the minute valuation series is built (`crawlBinance`). The banks have different limits: Monobank — a 61 s pause between requests of one key, a response limit of 500 operations; Raiffeisen — 30 s, the response limit is unknown.

**Auto-write into sources.** With `save=1`, after the walk (`saveToSources`) the same `saveAPISources` as in `/sources/save-api` is called: what was taken from the API is written into source files (a repeat without new data spawns nothing). A cancelled walk (a cache reset) writes nothing. The result goes into `last` (`autosave`, `saved`, write errors — in the common error list). Frontend: `useSync.js` (the shared `isSyncing`, the `zoe.sync.autosave` checkbox, on by default), `SyncButton.vue` in the header (`AppHeader`) and in the "Выписки" window, the start is `startSync` from `App`; a dot pulses at the Ζωή brand while sync runs.

**The account walk (`walkAccount`)** goes from "now" backward in windows (30 days) only where `sync_coverage` does not know the answer:

- a place already taken → a jump without a request; `From == HistoryStart` → the whole history, the end;
- a window that abuts what is taken is trimmed at its edge;
- a fresh cache = one request (the tail); lagging by a month = windows over the gap; an interrupted download continues from the last gap;
- a series of empty windows declares the start of history; an empty taken segment counts toward the "empty tail".

Errors are classified (`errKind`): the network (`errOffline`, the other accounts of the key are not polled) differs from a bank refusal; a cache reset cancels the walk (`errStopped`) and is not counted as a failure. `fetchWithRetry` retries. Structured progress (`progress.go`: client → accounts → timeline) is served in `/sync/status`; the frontend parses nothing from the status string.

`/sync/wipe-cache` first stops a walk in progress (`abortSync`), then clears the cache; `core.db` is not affected.

## 7. Persons, aliases, links

Glossary:

- **Alias** — a name from the bank (of an owner or counterparty); the key is `lower(trim)`. An extension in an alias (`Name.csv`) means "from an import file" and is **deliberately kept**: this way the IBANs of such an alias are not merged with a same-named API alias.
- **A Person** knows only the names of its aliases; everything else goes through the alias.
- **The effective structure of an alias** = (static ∪ cache) − bans. Green: confirmed by the cache; yellow: static only.
- **A shadow ban** forbids the cache to pick up a link the user removed by hand. It is lifted when the alias is unlinked or the link is manually restored.
- **An orphan PAN** — a card that is not among your own accounts. **A pseudo-alias** — a counterparty without a name, known only by a card.
- **The resolver** — the alias modal (`AliasDrumModal`): a drum of own cards, an operations table, all cards on the right; a card is dragged onto a counterparty row.

**The IBAN ↔ PAN link** is a global fact (`iban_pans`), not tied to an alias. An alias's IBAN automatically pulls its cards (from `iban_pans` and the cache) minus bans. `UnlinkAccount` bans the IBAN and, in cascade, its cards; `LinkAccount` must lift bans on both the IBAN and its cards (the regression is closed by the test `TestRelinkIbanRestoresPans`).

**Cache → static**: `syncAliases` is the only path. Only for aliases already linked to a Person; only what the cache confirms; bypassing shadow bans. It is called at startup and after import/sync.

**Card comparison**: `idKey` (the first 4 + last 4 digits); for masks like `414949****72` a weak key `looseKey` (BIN 6 + the last 2). **The Go implementation (`db/util.go`) and the JS one (`frontend/src/utils/personLayout.js`) must match**; change them only as a pair.

**The cache index** (`loadCacheIndex`, mirrored on the frontend by `buildCacheIndex`): the account name at the bank → its IBAN and all PANs; an operation's counterparty (`counter_name`, otherwise the contact from the description) → `counter_iban`. Comparison is exact, no LIKE.

**"Own account"** (`db/own.go`): the counterparty's number is taken from `counter_iban` and from the operation text (name/description); the operation's own account does not count; if there is no number, it is looked up among the alias's accounts and cards, but only when among them there is exactly one account of ours, other than the operation's account. The result is `Transaction.OwnAccountID` (on read). A transfer between own accounts is not included in income/expense, neither in the history nor in Smeta.

## 8. The ICU broker and bonds

- The account data is only the broker report: the journal in `icu_flows`, the balance is stored nowhere and is computed from the journal and the bond reference (`ComputeState`).
- Journal entry kinds (`icu_flows`): `deposit`, `withdraw`, `buy`, `sell`; a purchase debits money with the accrued coupon (`isin`, `qty`). Coupons and maturities are **not stored** in the journal: they are computed from the bond reference.
- The `bonds` reference (in `core.db`, one for everyone): ISIN, name, maturity `YYYY-MM-DD`, rate (% per annum), par value (1000 ₴ by default). A coupon is paid every 182 days, dates are counted backward from maturity (`couponPeriodDays`), the coupon size = par × rate / 100 / 2. Payouts (`PayEvent`, kind `coupon` or `redemption`) are computed on read; `Done` — already passed.
- Linking to a Person — the usual way, through an alias by the file name.
- On the frontend: `historyOrder.js` sets a logical order within a day (the report has no time: a noon placeholder, hidden in the table), `useCapitalChart` assembles bond operations into reference rows (`memo`), which are visible in the history but not included in income/expense.

### The shared chart engine (`utils/chartView.js`, `composables/useChartView.js`)

- The interactivity of SVG charts is written once. `utils/chartView.js` is pure math: `zoomWindow`/`panWindow` (the X-axis window with an anchor and span limits), `panY`/`zoomY` (a manual vertical within the extremes), `easeRange` (a step of smooth catch-up of the drawn vertical toward the target), `quantile` and `robustRanges` (fitting the vertical by percentiles so an outlier does not flatten the scale, plus `ext` — the extremes over all data).
- Line color by value: in `SERIES` a series has a `gradient { from, to, stops:[[fraction, color, opacity]] }` field, `buildMacroChart` gives `line.gradient` (a vector in chart pixels), `MacroPanel` draws a `linearGradient` in userSpaceOnUse; currently `crypto_fng` (light-blue fear — gray middle — amber greed) and `us_vix` (gray around 20, light blue above, amber below 14) are colored this way, `us_sentiment` — a traffic light red (≤ 50) → yellow (75) → green (≥ 100); `noLog` on a series hides the log-scale switch.
- The value scale is built into the engine: `scaleOf('linear'|'log')` (`fwd`/`inv`), `logTicks` (divisions m×10^k), `robustRanges({ log })`, `scaleTicks(mode, lo, hi)` in `growthChart.js`. The engine's vertical (`yShown`, `target`, `ranges`) lives in scale space (with log — the natural logarithm), so shifting, zooming, smooth fitting and inertia know nothing about the scale; `vAt` returns an ordinary value. A chart receives `scaleMode` and converts values itself through `fwd` (`buildMacroChart({ scale })`, `macroRanges({ log })`). `MacroPanel` provides a switch and a `log` prop (the starting scale); `canLog` — all values > 0. `GrowthChart` and `CandleChart` are linear for now (the `scale` option is not passed), it can be enabled the same way.
- `useChartView({ width, height, pad, autoX, ranges(view, scale), spans, resetOn, bounds, inertia, scale, canLog })` holds the window and vertical state, smooth animation (rAF), the wheel, dragging, hover, reset, and converts a mouse pixel into data (`tAt`, `vAt`). A chart itself gives only data and bounds: the default window (`autoX`), the vertical ranges for the window (`ranges(view)`) and a function that builds the lines. The X axis is years from today (as in investment growth), so the same `timeTicks` and `niceTicks` draw the labels on different charts.
- The Benner cycle layer: `components/BennerBands.vue` (`layer="bg"` — columns behind the data, `layer="strip"` — a strip in the footer) + `utils/benner.js` (`bennerPhase`, `bennerSpans`); the layer is shared and suits any chart with a time axis (`xOf(ms) → px`, the window edges `fromMs`/`toMs`, margins; candles convert slots via `msToSlot`/`slotToMs`), but it is connected only to the stock-exchange charts: `MacroPanel` has the `benner` prop `false` by default, `MacroStocksView` turns it on, `GrowthChart` and `CandleChart` do not use the layer. The colors are bright (#fb7185 / #fbbf24 / #34d399). This is a curiosity, not a signal (labeled in the interface).
- Connected: `GrowthChart.vue` (investment growth), `MacroPanel.vue` (macro series) and `CandleChart.vue` (capital candles). A trap: the `dragY` variable is declared before `watch(target, …, { immediate: true })`, otherwise TDZ in dev. Optional features: `bounds()` — the X window bounds (`clampWindow`), `inertia` — a sideways throw coasts by inertia (`inertiaStep`); a horizontal touchpad gesture moves the window, `eventT(e)` gives the X axis under an event (a click without movement), when the window shifts under a still mouse the tooltip follows it.

### NBU macro (`market/macro.go`, `db/macro.go`, `GET /api/macro`)

- FRED series (sources `us`, `usecon`, `stocks`) are downloaded by the shared `Client.fredAll` (`market/fredall.go`): a batch in parallel, no more than `fredParallel` (4) requests at once, the result and errors in job order, progress by the number of ready series. A new FRED series is one line `fredJob{FRED id, note, our series id}` and its own transformation. Daily series without recalculation (DGS10, T10Y2Y, VIXCLS, NASDAQCOM, DJIA) are topped up by a tail: `Client.LastDate` (by default `Storage.MacroLastDate`, set in `Server.SetMarket`) gives the last date of the series in the cache, `tailStart` takes the Monday of the week two weeks before it and asks FRED for `&cosd=`; such a series goes into `Result.Tail`, and `SaveMacroSeries` replaces it only from the first date of the tail. Series computed from an index (US inflation) and ones stitched with history (S&P 500), as well as small monthly series, are downloaded in full. An empty cache — the full history.
- Which series each numeric source provides is declared by the source itself (`market.Feed.Provides`) and served by `GET /api/macro/catalog` (`{feeds:{fx:[…]}}`); the frontend (`useMacro('fx')`) takes ids from there and keeps no lists of its own.
- `SaveMacroSeries` writes a series in one transaction with a prepared statement (150k points ≈ 1 s). `MacroSeries(ids)` selects series in SQL (`WHERE series IN …`): a request for one series on a 150k-point table ≈ 11 ms instead of ≈ 290 ms when the whole table was read; with more than 200 identifiers the whole table is read and selected in Go.
- The `macro` source ("Rates and inflation · NBU, Eurostat") is in `Feeds()`. The addresses were verified by a live curl: `bank.gov.ua/NBUStatService/v1/statdirectory/key?json` — all rates in one request (daily since 1992; `id_api`: `KEY_PolicyRate`, `KEY_RateOvernightCD`, `KEY_RateOvernightLoans`, the field `value`), and `…/inflation?period=m&start=YYYYMMDD&end=YYYYMMDD&json` — only in windows (as a whole the server answers 400 "надто великий dataset" — too large a dataset); windows of 10 years from 2000, the data starts in 2007.
- Eurozone inflation is part of the `macro` source (`market/eu.go`, `Client.EurostatBase`, series `eu_cpi_yoy`, monthly, the date is the first of the month): Eurostat SDMX-CSV `…/eurostat/api/dissemination/sdmx/2.1/data/prc_hicp_minr/M.RCH_A.TOTAL.EA?format=SDMX-CSV` (the new dataset: the whole history since 1997 up to the last month in one request; columns `TIME_PERIOD` ("2026-10"), `OBS_VALUE`; parsing is tolerant of rows of different length; verified by a live curl, the last point October 2026). Since February 2026 the ECB ICP set and Eurostat `prc_hicp_manr` are frozen at December 2025 — we do not use them. The request goes with an explicit `Accept: text/csv` and a browser-like `User-Agent` (`getHeaders`), `Accept-Encoding: identity`, and gzip without a Content-Encoding header (Eurostat answers like this: the body starts with `1f 8b`, Go does not unpack it itself) is recognized by the signature and unpacked in `eurostatCSV`; the error of the `macro` source on failure contains the beginning of Eurostat's response. Frontend: `inflationFor(currency, series)` in `utils/macro.js` (UAH — `cpi_yoy`, USD — `us_cpi_yoy`, EUR — `eu_cpi_yoy`) gives the "Реал." column of the bond table the inflation of the security's currency; `eu_cpi_yoy` is part of the `macro` source (a light set), the chart is on the "Инфляция" screen.
- Rates are compressed to the dates of value change (`changePoints`). From inflation only total rows are taken: `ku` is empty, `mcrd081 = Total`, `mcrk110` is empty, the name "Індекс споживчих цін" (headline) or "Базовий …" (core); `tzep` `DTPY_` = y/y, `PCPM_` = to the previous month; the rest (producer prices, regions, categories, cumulative `PCCM_`/`PCCP_`) is discarded and counted in `Skipped`.
- A failure of one part does not erase the other: what was downloaded is saved, the error goes into `Result.Error`. A series in the database is replaced as a whole (`SaveMacroSeries`), other series are not touched.
- Frontend: `composables/useMacro.js` (reads `/macro`, rereads by `data_version`), `utils/macro.js` (`valueAt`, `realRate` by Fisher, `macroSummary`, `buildMacroChart`), `components/MacroPanel.vue` (the Benner cycle layer, the `benner` prop; the `MACRO_PAD.b` field 36 gives room for the strip); the "Реальная" column in `MarketView` (`signed` in `COLUMNS`: zero and minus are not counted as "empty" when sorting).

### Market bonds (`pkg/market`, `db/market.go`, `api/handlers_market.go`)

- Sources: `https://bank.gov.ua/depo_securities?json` (the registry, OVDP are selected by the ISIN prefix `UA4000`), `https://bank.gov.ua/NBU_ovdp?json&date=DD.MM.YYYY` (Ministry of Finance auction results; the date strictly `DD.MM.YYYY`, a day without an auction gives `[]`; the last 14 days are tried) and `…/v1/accounting/od/auctions_query` of FiscalData (US auctions for 3 years, CUSIPs without duplicates, the later auction wins).
- No keys, read-only requests. The update is **on demand** (the only exception — crypto-loop snapshots: `RunMarketSchedule` itself starts the `chain` source every `MARKET_EVERY` on a time grid, a missed slot is taken at startup, on failure a retry after 2 minutes), as a separate process alongside the bank walk: it starts together with "Разведать" (`POST /api/sync/full`, even if the banks have no keys) or by itself (`POST /api/market/sync[?source=nbu|ust]`, an immediate response, 409 if already running). The list — `GET /api/market/bonds[?source=&q=]`.
- The sources are "feeds" (`market.Feed`: `ID`, `Title`, `Fetch`, the list `Client.Feeds()`): they run **in parallel** and know nothing about each other, a failed one does not hinder another. A new source (the Fed, etc.) is one more entry in `Feeds()`, without new routes and toasts. A securities feed fills `Fetch` (`[]Security` → `market_bonds`), a numeric-series feed — `Series` (`[]SeriesPoint` → `macro_series`, `Storage.SaveMacroSeries`), a feed has a `Unit` ("securities"/"points") for the process window and the toast; `?source=` is validated by `market.IsSource`.
- The state is the `market` key in `/api/sync/status` (`running`, `started_at`, `feeds[]` with `state` `queued|fetch|done|error`, `fetched`, `skipped`, `error`, and `last` with the result). It is **not mixed** with the bank `progress`: its own result, its own lock and its own cancel context. A cache reset (`abortSync`) cancels this process too and waits for it up to 3 s so that it does not write data into a clean database.
- Progress: `Feed.Fetch(ctx, rep Reporter)`; the source reports `rep(pct, note)` ("auctions: 07.11", 46 %). In `feeds[]` this is `pct`/`note`: with a percentage the frontend draws a fill, without one (`pct` 0) — running bars. For long downloads (for example, a monthly US rate over many years) the source just calls `rep`.
- The process window: `MarketPanel.vue` (in `ToastStack`) hangs while `market.running`, like the bank sync window, but separate; the cross silences exactly this run (`started_at`). The display logic is `utils/marketPanel.js` (`marketPanelOpen`, `feedView`).
- Frontend: `utils/marketNotice.js` decides what to show by `market.last.finished_at` (a result from before a page reload is not shown); each source has its own toast: green on a timer or red until closed.
- Parsing is tolerant: a field is searched under several names, `null`/empty — no value, a number accepts `16,44`. A record without an identifier is skipped (`Result.Skipped`). Formats were checked against documentation, not a live response, so when a source's fields change the list of names in `nbu.go`/`ust.go` is edited.
- Saving is `ON CONFLICT … DO UPDATE`: an empty value does not overwrite a known one (the registry gives the terms, the auction — the yield). Matured securities stay. `WipeCache` erases this table too.
- The link with ICU: `bondTerms()` takes the `core.bonds` reference and adds downloaded hryvnia OVDP with a rate > 0 and a maturity date; **your own entry always takes precedence over a downloaded one**. The reference editor (`ListBonds`, `/api/icu/bonds`) shows only your own entries.
- Autofill: the "+ ОВДП" form in the reference editor (`IcuBondsEditor.vue`), on a 12-character ISIN, requests `GET /api/market/bonds?source=nbu&q=<ISIN>`, looks for an **exact** match (`utils/bondLookup.js`) and substitutes only empty fields: maturity, rate, name; what was typed by hand is not overwritten, a zero-coupon one does not fill the rate. Not in the cache — the hint "press «Разведать» or enter manually".
- The other build 2 sources (the US from FRED, countries and the eurozone, commodities, world equities, company stocks, crypto and the crypto loop, purchasing power) are built the same way: one feed, their own series, registration in `Feeds()`; the series catalog and the country registry are served by `GET /api/macro/catalog`, the source health — by `GET /api/macro/sources`.

## 9. Binance

The source of movements is only the official statement (Wallet → Transaction History → Export); there is no API key. Public prices are taken from `api.binance.com` (minute candles, `Prices`, `Symbols`); the client respects the limits and retries requests.

- `p2p.go` reads the C2C order history (Orders → P2P → Export): positionally, all orders as is, the maker and taker fee is summed in the coin; `Qty − Fee` arrives in the wallet (this is the number that lies as the P2P row in the statement journal, a few minutes after the order creation). This is a supplement to the statement: its own `.zoe` of kind `binance-p2p`, it does not change the account name, profile and axis coverage, orders in the cache (`binance_p2p`) survive only until a reset and come back with "Загрузить".
- `statement.go` reads all rows; `ledger.go` decides what a row means for the portfolio: external movements (deposit, withdrawal, P2P, bonus, dividend) become account operations, the rest (trades, fees, conversions, dust, staking) stays an internal wallet movement.
- Wallets: Spot, Funding, Strategy, USD-M Futures — **all four** count as part of the portfolio at market price, so the chart moves as bots and futures work. Earn has no rows of its own in the statement and is visible by subscription/redemption.
- `moves.go` assembles transfers between wallets: two journal rows (minus in one, plus in the other) are paired by coin and amount within a window of a few seconds; a lone half is completed by the operation name. These are reference rows of the history ("from → to"), they do not change capital. `swaps.go` — swaps (Convert, dust → BNB, staking), `trades.go` — purchases and sales.
- `series.go`: balances are known from the journal at any moment (`Timeline`), the portfolio history = a valuation of balances by minute candles of coins and the USDT/UAH pair. The series is signed with the journal hash (`Sig`): the journal changed — the series is stale. After a new CSV the series is cut from the minute of the first **new** row and redrawn; a repeat of the same CSV does not touch the series; a new row earlier than the start of the series — the series from scratch.
- The account balance = the last point of the series. On the Binance card it is shown in dollars with a hryvnia equivalent (the USDT/UAH rate from the last "Scout").
- "Сброс кеша" (Cache reset) wipes Binance too; "Загрузить" brings the journal back from `.zoe`, the next "Scout" builds the valuation. The series is saved into `.zoe` as `binance-series` (collection takes 10–20 minutes).

## 10. Frontend

Vue 3 (`<script setup>`), Tailwind v4, Vite, bun. No router and no store: state lives in `App.vue` and composables; tabs are in `KeepAlive`.

### 10.1 Data flow

```
setInterval 2 s → GET /sync/status → did stats.data_version change?
                                     └─ yes → fetchData(): /accounts /transactions /users …
```

`App.vue` holds `accounts`, `transactions`, `users`, the sync progress and passes them to tabs as props. Toasts (`useToasts`) show the sync result. Loss of connection to the backend is a separate plate, not "sync failed".

### 10.2 API layer (`utils/api.js`)

The only place with `fetch`: `API_BASE` (`VITE_API_BASE`, by default `http://127.0.0.1:8080/api`: `localhost` could go over IPv6 to someone else's program), `apiSend(method, path, body)` (JSON), `apiUpload` (multipart), `apiGetJson`, `apiJson`, `q` (= `encodeURIComponent`). A non-2xx error turns into an `Error` with the backend's text. There must be no direct `fetch` in components.

### 10.3 Browser storage (`utils/storage.js`)

`storageGet/storageSet/loadJSON/saveJSON` — safe wrappers over `localStorage` (a private window, cleared data: they do not throw). Used only for conveniences: the active tab (`economic_active_tab`), the timeframe (`zoe.candle.tf`), the card order (`persons_card_order`). Everything important to keep lives on the backend.

### 10.4 Pages (`views/`)

There are no top tabs in the header: the header is the side menu, the Ζωή brand (return to Home) and "Разведать" (Scout). Navigation is `utils/nav.js`: `HOME_TAB` (`home`), `TOP_TABS` (`home`, `dashboard`, `market`, `data`) and `ARCHIVE_PAGES` (Смета (Smeta), Рынок (Market), Учет (Uchet): pages of the old interface, living in the "Архив" (Archive) section of the side menu until the design is rebuilt in build 3).

| Tab | Key | Purpose |
|---|---|---|
| Главная (Home) | `home` | `HomeView`: a capital tile and a candlestick chart across all accounts (`useCapitalChart`, `CandleChart`); the beginning of the future interface |
| Смета (Smeta) | `dashboard` | the Capital tile (in $ when rates are known: `fxRates`/`toUsd` in `utils/dollar.js`, next to it ≈ ₴), `CapitalStructure` (capital layers by account and a currency breakdown: `capitalStructure(accounts, rates)` in `utils/capital.js`, no backend needed: the broker and Binance have a `portfolio` in `accounts`), `BondPortfolio` (`portfolioStats`: average rate and term weighted by position `value` from `/icu/ledger` → `positions`, real yield by Fisher with inflation from `useMacro`) and a chart across all accounts for all time |
| Рынок (Market) | `market` | `MarketView` (the shell) and `BondsView`: a table of the bond cache (`/market/bonds`), the OVDP (`nbu`) and US (`ust`) sections in `SectionRail`; the table logic is in `utils/marketTable.js` (filter, sort, labels, the coefficient `scoreBonds`: average annual growth over 10 years (`growthOf`: the security's rate for its term, the remainder at the lower quartile of the currency's rates (`quantile`)), normalization between the worst and best security of a currency, places and the Top 1/10/25/100 badges, the color `scoreColor` 0°…120° HSL; the "only ICU" filter (`parseIsins`, `restrictTo`; the ISIN list is on the server, `/market/lists?name=icu`; scores are computed over the narrowed pool), the pieces already bought (`parseIcuList` (ISIN and price from pasted text), `curveAt` and `resolveRates` (the broker security's yield: by price → by the shown YTM → by a curve from known securities and user points; each has a `source`), screenshots: `utils/icuOcrRun.js` (tesseract.js in the browser: the core, worker and `eng` model are in `node_modules`, connected via `?url`, the model lands in the build as `ocr/eng.traineddata.gz`, the naming rule is in `vite.config.js`; three passes with different threshold and scale, cropping the header and bottom bar of a tall phone frame) and `utils/icuOcr.js` (pure text parsing: a card = a line with YTM/SIM, the ISIN is checked against the registry by maturity date and Hamming distance, the price is chosen from glued digit runs by matching the YTM, `mergeEntries` merges passes and frames), the panel `components/IcuIntake.vue`, the state `composables/useIcuList.js`, `ytmFromPrice` (the effective yield by the broker price: coupons twice a year backward from maturity, price with accrued coupon; checked against the ICU YTM within 0.05 pp), in the "only ICU" mode `scoreBonds(list, today, rates)` takes it instead of the auction/coupon one; `holdingsOf` by the positions of `/icu/ledger` of brokerage accounts, the "N шт." (N pcs.) chip at the name); the growth chart: math and coordinates in `utils/growthChart.js` (`multipleAt`, `autoView`, `zoomView`, `panView`, `timeTicks`, `yRanges`, `panY`, `zoomY`, `cleanMatured`, `buildGrowthChart`), drawn by `components/GrowthChart.vue`: one SVG, years from today horizontally (the past on the left), the amount vertically, scale by the best 5 in the window, the rest up to 150 lines in the background, matured ones as a gray dashed line from `issued` to `maturity`; wheel/dragging/double click; the vertical is hybrid: `yRanges` gives `auto` (for the window) and `ext` (extremes), a manual shift/Shift+wheel is kept in `ext`, and the drawn range catches up with the target through rAF; the Top 1/Top 5 gap over 10 y. by currency is computed by `MarketView` and passed as the `spreads` prop) |
| Учет (Uchet) | `data` | `DataView`: a list of sections beyond the left edge of the main grid (labels from 1380px, icons from 1240px, below as a row on top), the selected screen on the right |

The section list is the shared component `SectionRail` (it supports groups `{key,label,icon,children}`: a header with a chevron only in wide mode, a group's openness is remembered under `groups-key`, the group of the selected section opens by itself; flat lists work as before). Market: the `MarketView` shell (a drawer, `useMacro` once, `KeepAlive` for `BondsView`) + the screens `BondsView` (the `nbu`/`ust` section as a prop), `MacroRatesView`, `MacroInflationView` on the universal `MacroPanel` (props `ids`, `unit`, `title`, the `summary` slot; the default window is the whole period, then the wheel, dragging and "Сброс масштаба" (Reset zoom)); the section tree and `feedOf` are `utils/marketNav.js` (a section has a `feed` field: rate and inflation — `macro`, "Курсы" — `fx`, "Ставка ФРС" — `us`; `MacroFxView` reads series with a separate `useMacro('fx')` only on its own screen), the other macro screens (`Macro*View.vue`) are built the same way (`MarketHeader` and `MacroPanel`s), and the full list of sections, their `feed` and icons is kept only by `MACRO_SECTIONS` in `utils/marketNav.js` (it is not duplicated here); "Обновить" (Update) is `composables/useMarketRefresh.js` (`POST /market/sync?source=`, `macro` for the macro screens) (labels / a column of icons / a row on top), the section choice and scroll position are `composables/useSection.js`; "Uchet" and "Market" use it (the market section key `economic_market_section`, refresh by `data_version`, starting the poll `POST /market/sync?source=`). The "Uchet" sections (key in `economic_data_section`): **История** (History) (`history`, `OperationalView`: account tiles with multi-select + chart + operation history) and **Лица** (Persons) (`persons`, `PersonsView`: the alias graph, orphans, the link editor, the drum; a count of people next to the title). Both screens are in their own `KeepAlive`, each has its own scroll position. The old values `accounts`/`entities` in `economic_active_tab` are converted by `utils/nav.js` (`resolveNav`) and immediately rewritten in the new form. A new top-level page = an entry in `TOP_TABS` and `ARCHIVE_PAGES` (`utils/nav.js`) and a branch in `App.vue`; a new data section = an entry in `SECTIONS` in `DataView`.

Components with scrolling or size measuring inside `KeepAlive` must resync in `onActivated`: a hidden block loses `scrollLeft` without an event (this is how `CandleChart` was fixed; it does not use scrolling now).

The side menu is built from the registry `utils/sections.js` (currently two sections: "Архив" — a navigator section, `pages: ARCHIVE_PAGES`, a click switches the app page; and "Управление" (Management) → `DebugSection`, which holds the "Выписки" window). A new section = one entry in the registry.

### 10.5 The alias graph (`utils/personLayout.js`)

The main client-side logic: from `users`, `alias-structures`, accounts and transactions a graph is built — for each Person, the bank groups, IBANs, cards, confirmation, orphan PANs. It contains a mirror of the backend (`idKey`, `looseKey`, `isPanLike`, `buildCacheIndex`): **change together with `db/util.go` and `db/resolve.go`**. `resolveRows.js` prepares the resolver rows (outgoing: only transfers to people; incoming: everything except legal entities and shops; a bank as a "counterparty" is a fee, not a person). `personName.js` substitutes a counterparty alias with the Person: who belongs to whom is decided by the backend (`resolved_user_id`), the frontend only substitutes.

### 10.6 The candlestick chart

- `composables/useCapitalChart.js` assembles the input: hryvnia accounts, their operations, crypto accounts, bond operations (`bondOps`) and payouts (`payEvents`). If `pickedIds` is empty — all accounts (Smeta), otherwise the selected cards (Operational).
- `components/CandleChart.vue` is the reactive part: timeframes, loading Binance candles in chunks (a cache per timeframe), selecting a candle by click. Zoom, shift, inertia and the vertical are driven by the shared engine (`useChartView`): the X axis is slots from the right edge of the data (`candleBounds`, `candleWindow`, `candleSpans`, `visibleSlots`, `candleYRanges` in `utils/candleMath.js`), so loading history on the left does not move the window; only visible candles plus a spare slot are drawn; the vertical by candles in the window, a manual shift and Shift+wheel within the extremes; the svg width is measured by a `ResizeObserver` (a hidden tab in `KeepAlive` gives 0 — the previous one is kept). The wheel over it is handled by the component itself (`data-wheel="own"`).
- `utils/candleMath.js` — **pure math without Vue and DOM** (the timeframes `TFS`, day/week starts, buckets, "nice" scale steps, labels, `candleMove` — the move of the selected candle for the history header: the capital change and range from `open/close/high/low` of the `select` event); covered by tests, including under three time zones.
- Timeframes: `1m 5m 15m 1h 4h` (intraday windows with a candle-count limit and loading deeper) and `1D`, `1W` (the whole history). An operation "in the future" falls into the last bucket.

### 10.7 Other

- `utils/banks.js` — **the bank registry**: the MFO code from the IBAN, card number beginnings (BIN), the monogram, the name, the order. A new bank = an entry here plus a color scheme in `bankTheme.js`.
- `utils/numfmt.js` — shared number and date formatters (`numText`, `fixedText`, `intText`, `ruFormat`, `dmy`: a decimal comma, spaces between digit groups, the date DD.MM.YYYY); all screens format through them. `utils/errText.js` — a single error text (`errText(e)`). `utils/feedStamps.js` — feed freshness marks for `MarketView`.
- `utils/formatters.js` — money (the currency symbol **before** the number: `₴ 1 234,50`; the delta without a symbol, in small subscript), dates (`formatRuDate`, `formatTxDate`, `formatTxTime`), `fmtN`, `formatPeriod`, `plural`.
- `utils/txTags.js` — operation badges (the Binance scheme: kind + wallet; a transfer "from → to").
- `composables/usePaged.js` — "Показать ещё" (Show more) for long tables (page 100 in history, 150 in the resolver).
- `composables/useDrum.js`, `useDragAutoScroll.js`, `useWheelScroll.js` — the drum, autoscroll at the edges while dragging, the wheel over sliders.
- Modals: `ImportCsvModal` (add/update, with a check), `StatementsModal` (the "Выписки" window), `LinkEditorModal`, `AliasDrumModal`, `AccountLifeModal`, `IcuBondsEditor`.

### 10.8 Performance rules (audit `v1.2.103`–`v1.2.112`)

- Macro series (`useMacro`, `useCountries`) live in `shallowRef`, not `ref`: deep reactivity wrapped every point in a Proxy, and reading series (the dollar index, "Курсы") was dozens of times slower. A series is replaced as a whole, not changed in place.
- `utils/macro.js`: a numeric series index (`indexOf`, cached in a `WeakMap` with a check of length and the last point) and window bisection (`firstFrom`) instead of walking all points; a dense line is thinned by columns (`thinColumns`); `robustRanges` sorts a typed array.
- `Intl.NumberFormat` is created once per module (≈29 µs to create versus ≈0.5 µs to reuse): do not create formatters in loops and templates.
- `MarketView` holds `feedStamps(() => dataVersion)`: a section opened before the data was written to the cache is reread on return.
- `macro_series` is a clustered table (`WITHOUT ROWID`, §4), JSON responses are compressed by fast gzip with a pool (§3.2).

## 11. Tests and CI

- Backend: `cd backend && go test ./...` — dozens of scenarios in `pkg/db` (links, bans, masks, ICU, bonds), parsers, Binance (transfers, swaps, prices), HTTP (`routes_test.go`: every route behind CORS, method protection, refusal on a malformed and a huge body, parsing of env lists), sync (`sync_test.go`). Test statements and reports are anonymous.
- Frontend: `cd frontend && bun run test` = `node --test tests/*.test.mjs && vitest run`: pure logic (the API layer, the bank registry, chart math, macro, formats) on `node --test`, components on vitest with happy-dom (`tests/components`). On Node 22 the glob `tests/*.test.mjs` is needed, not a directory. `package.json` has `overrides` for `glob` and `minimatch` (vulnerabilities of transitive devDependencies): `bun audit` is clean, update `bun.lock` together with them.
- `.github/workflows/ci.yml` — three jobs on every PR and push to `dev`, `build`, `main`:
  - `backend`: `gofmt -l` check, `go vet`, `golangci-lint` (v2.15.0, config `backend/.golangci.yml`), `go test ./... -count=1`, `go build`;
  - `frontend`: `bun install --frozen-lockfile`, `bun run test` (node and vitest), `bun run build`;
  - `hygiene`: no databases, keys, archives, `.env`, `.commits/`, `.claude` in git; no keys and tokens (`AKIA…`, `ghp_…`, private keys); no 16-digit numbers that look like a card number in `backend` and `frontend/src` (test stubs are in an allowlist inside `ci.yml`).

The pre-"done" check rule: `go test ./...`, `golangci-lint run`, `bun run test` and the frontend build must pass.

## 12. Invariants and pitfalls

**Do not change without an explicit reason**

- Fee handling in parsers (Privat: `Amount` by the card column; Raiffeisen fee semantics).
- The file extension in an imported alias.
- The `StableID` key format: otherwise a re-import creates duplicates.
- The Go ↔ JS pairs: `idKey`, `looseKey`, `isPanLike`, the cache index.
- `core.db` — do not delete or recreate without an explicit request; migrations are additive only.
- "Стереть" (Erase) in the "Выписки" window requires a second click ("Точно?" — Sure?): this is protection against an accidental reset.

**Decided (do not return without a reason)**

- Hints "the other side of a transfer by time and amount": several variants were tried, they are superfluous and confusing.
- Auto-pulling a PAN on the frontend duplicated the backend and did not cure the bug: the source of truth is the backend.
- A bank in the role of a counterparty (PrivatBank, Monobank) is not a person (`isBankCounter`).

**Known assumptions**

- ICU: sell, withdraw and fees have no report samples; what the parser does not know, it rejects with an error.
- Binance: the names of staking-withdrawal operations are assumed.
- The USDT/UAH rate is frozen after the date of the last "Scout".
- NovaPay: a credit card has not yet seen statements.

**Hidden coupling**

- Two databases and `syncAliases`: any code that writes to `core.db` from the import path breaks the principle of §1.2.
- ICU time is a placeholder (noon), the order within a day is set by `historyOrder.js`.
- A network error during sync: a retry of the same interval with a pause, then offline; the UI is not reset.

## 13. Recipes: how to add

**A new bank as a statement source**
1. A parser in `backend/pkg/<bank>/`: return `[]mono.StatementItemDTO`, IBAN, card, balance; IDs via `stmtkit.StableID`; an `IsStatement(raw)` marker for auto-detection.
2. A branch in `parseSource` (`api/sourcefile.go`) and, if needed, an account type.
3. Frontend: an entry in `utils/banks.js` (MFO, BIN) and a color in `utils/bankTheme.js`; a logo in `frontend/public/banks/<MFO>.svg`.
4. Anonymous fixtures and tests for re-import (no duplicates) and for period overlap.

**A new API route.** A handler in `pkg/api/handlers_*.go` with `allow`/`readJSON`/`writeJSON`; an entry in the `routes.go` table (CORS is already on the whole mux); a test in `routes_test.go` if needed.

**A new side-menu section.** A panel component from `UiGroup`; an entry `{ key, label, component }` in `utils/sections.js`.

**A new Binance operation kind.** A rule in `ledger.go` (an external or internal movement) → a kind in `Kind*` → a badge in `txTags.js` → a test in `binance`.
