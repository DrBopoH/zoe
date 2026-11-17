**English** · [Русский](README.ru.md) · [Українська](README.uk.md)

# Ζωή

> Personal finance app: collects statements from banks, a broker and an exchange into one picture of your capital.
> *Ζωή (zoe) — personal finance app: bank statements + broker + exchange → one capital chart, history and a persons/aliases graph. Runs locally, single user.*

**Status:** build 2 (`v1.2`, macro and market data) is closed as stable `v1.s2.113+143`; next comes build 3 — interface polishing and finishing QoL, builder and reference features. Build 1 (`v1.b1`, beta) — data collection from banks, the broker and Binance — is frozen. Versions follow the `R.sB.C+T.hash` scheme, see [docs/WORKFLOW.md](docs/WORKFLOW.md) and [CHANGELOG.md](CHANGELOG.md).

## What it does

- **Data sources:** Monobank (API), Raiffeisen (API and CSV), PrivatBank (PDF statement), NovaPay (CSV), the ICU broker report (PDF, OVDP bonds), the Binance statement (CSV) plus public Binance minute prices. No Binance key needed.
- **Capital over time:** a candlestick chart across all accounts (timeframes 1m…1W), an operation history with kind badges; transfers between your own accounts count neither as income nor as expense.
- **Persons and aliases:** who stands behind a counterparty name, which accounts and cards they have; links are edited by hand and survive a cache reset.
- **Market and macro (build 2):** OVDP bonds (NBU registry and auctions) and US Treasuries with a coefficient for long-term accumulation; NBU rates, inflation and exchange rates; US macro (Fed, Treasury curve, real rates, prices, labor, activity, liquidity, credit, scorecard); cards for countries and the eurozone (22 registry entries); commodities, world equities, US company stocks (SEC), crypto and the crypto loop, purchasing power; a "Sources" page with the health of each source. All from public APIs, no keys needed.
- **Unified `.zoe` source format:** parsed data lives on disk, the cache database can be wiped and restored with one button; macro series and stock data are stored there too, in the "Economy" group.

Everything runs locally: the backend listens on `127.0.0.1` only and checks the Host header, and personal data goes nowhere except requests to the banks themselves. The app fetches market data itself from public sources without keys (NBU, Ministry of Finance, FiscalData, FRED, ECB, Eurostat, BIS, World Bank, Yahoo, SEC, Nasdaq, Binance and others; the full list is in `backend/pkg/market`).

## Quick start

You need: Go (version in `backend/go.mod`), [bun](https://bun.sh), `pdftotext` from poppler (for Privat PDFs and ICU reports).

```bash
# backend, port 8080
cd backend && go run .

# frontend, http://localhost:5173
cd frontend && bun install && bun run dev
```

Bank tokens are optional: statements can be uploaded as files (Management → Cache → Statements). For API sync create `backend/.env`:

```env
MONO_TOKENS=token1,token2     # or MONO_TOKEN=single_token
RAIF_TOKENS=token             # or RAIF_TOKEN=single_token
```

### Environment settings (backend)

| Variable | Default | Purpose |
|---|---|---|
| `MONO_TOKENS` / `MONO_TOKEN` | — | Monobank tokens (comma-separated list or a single one) |
| `RAIF_TOKENS` / `RAIF_TOKEN` | — | Raiffeisen tokens |
| `PORT` | `8080` | API port |
| `HOST` | `127.0.0.1` | address; for access from another device use `0.0.0.0`, and then the frontend's host name must be in `ALLOWED_ORIGINS`, otherwise the backend answers 403 "Host not allowed" |
| `ALLOWED_ORIGINS` | `http://localhost:5173`, `http://127.0.0.1:5173` | comma-separated list of frontend Origins |
| `CACHE_DB_PATH` | `cache.db` | disposable statement cache |
| `CORE_DB_PATH` | `core.db` | permanent data: persons, aliases, links |
| `MARKET_EVERY` | `1h` | how often the backend itself takes crypto-loop snapshots (P2P price, network fees, dominance); `30m`, `2h`, etc., `0` turns it off |
| `ECONOMIC_SOURCES_DIR` | `$XDG_DATA_HOME/economic/sources`, otherwise `~/.local/share/economic/sources` | folder of `.zoe` source files |

Frontend: `VITE_API_BASE` (default `http://127.0.0.1:8080/api`).

## Repository structure

```
backend/    Go + SQLite (module economic): API, statement parsers, storage
frontend/   Vue 3 + Tailwind v4 + Vite (bun): Home and "Archive" in the side menu (Smeta, Market, Uchet)
docs/       documentation: USER.md, ARCHITECTURE.md, WORKFLOW.md
clc.sh      git and patch helper (see docs/WORKFLOW.md)
.github/    CI: tests, build, hygiene checks
```

## Documentation

| File | For whom |
|---|---|
| [docs/USER.md](docs/USER.md) | how to use it: tabs, statement import, persons and aliases |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | how it is built: packages, databases, data flows, rules, pitfalls |
| [docs/WORKFLOW.md](docs/WORKFLOW.md) | git regulation, versions, branches, `clc.sh`, CI |
| [CHANGELOG.md](CHANGELOG.md) | what changed and when |

## Tests

```bash
cd backend && go test ./...
cd frontend && bun run test      # node --test tests/*.test.mjs and vitest (components)
cd frontend && bun run build
```

The same checks (plus `gofmt`, `golangci-lint` and hygiene: no databases, keys, archives or card numbers) are run by CI on every PR and on push to `dev`, `build`, `main`.

## Privacy

The repository contains no databases, keys, statements or real data: test statements and fixtures are anonymous. `cache.db`, `core.db`, `.env` and `.zoe` files stay on your machine.
