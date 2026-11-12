# Ζωή

> Личное финансовое приложение: собирает выписки банков, брокера и биржи в одну картину капитала.
> *Ζωή (zoe) — personal finance app: bank statements + broker + exchange → one capital chart, history and a persons/aliases graph. Runs locally, single user.*

**Статус:** билд 1 (`v1.b1`, бета) — сбор данных из внешних источников. Версии ведутся по схеме `R.sB.C+T.hash`, см. [docs/WORKFLOW.md](docs/WORKFLOW.md) и [CHANGELOG.md](CHANGELOG.md).

## Что умеет

- **Источники данных:** Monobank (API), Raiffeisen (API и CSV), PrivatBank (PDF-выписка), NovaPay (CSV), брокерский отчёт ICU (PDF, облигации ОВДП), выписка Binance (CSV) плюс публичные минутные цены Binance. Ключ Binance не нужен.
- **Капитал в динамике:** свечной график по всем счетам (таймфреймы 1m…1W), история операций с бейджами видов, переводы между своими счетами не считаются доходом и расходом.
- **Лица и алиасы:** кто стоит за именем контрагента, какие у него счета и карты; связи редактируются руками и переживают сброс кеша.
- **Единый формат источников `.zoe`:** разобранные данные лежат на диске, кеш-базу можно стереть и восстановить одной кнопкой.

Всё работает локально: бэкенд слушает только `127.0.0.1`, данные не уходят никуда, кроме запросов к самим банкам и публичным ценам Binance.

## Быстрый старт

Нужны: Go (версия в `backend/go.mod`), [bun](https://bun.sh), `pdftotext` из poppler (для PDF Привата и отчётов ICU).

```bash
# бэкенд, порт 8080
cd backend && go run .

# фронтенд, http://localhost:5173
cd frontend && bun install && bun run dev
```

Токены банков не обязательны: выписки можно загружать файлами (Управление → Кэш → Выписки). Для синхронизации по API создай `backend/.env`:

```env
MONO_TOKENS=токен1,токен2     # или MONO_TOKEN=один_токен
RAIF_TOKENS=токен             # или RAIF_TOKEN=один_токен
```

### Настройки окружения (бэкенд)

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `MONO_TOKENS` / `MONO_TOKEN` | — | токены Monobank (список через запятую или один) |
| `RAIF_TOKENS` / `RAIF_TOKEN` | — | токены Raiffeisen |
| `PORT` | `8080` | порт API |
| `HOST` | `127.0.0.1` | адрес; для доступа с другого устройства `0.0.0.0` (и добавь его Origin в `ALLOWED_ORIGINS`) |
| `ALLOWED_ORIGINS` | `http://localhost:5173`, `http://127.0.0.1:5173` | список Origin фронта через запятую |
| `CACHE_DB_PATH` | `cache.db` | одноразовый кеш выписок |
| `CORE_DB_PATH` | `core.db` | постоянные данные: лица, алиасы, связи |
| `ECONOMIC_SOURCES_DIR` | `$XDG_DATA_HOME/economic/sources`, иначе `~/.local/share/economic/sources` | папка файлов-источников `.zoe` |

Фронтенд: `VITE_API_BASE` (по умолчанию `http://localhost:8080/api`).

## Структура репозитория

```
backend/    Go + SQLite (модуль economic): API, парсеры выписок, хранилище
frontend/   Vue 3 + Tailwind v4 + Vite (bun): три вкладки — Смета, Операционная, Лица
docs/       документация: USER.md, ARCHITECTURE.md, WORKFLOW.md
clc.sh      помощник для git и патчей (см. docs/WORKFLOW.md)
.github/    CI: тесты, сборка, проверки гигиены
```

## Документация

| Файл | Для кого |
|---|---|
| [docs/USER.md](docs/USER.md) | как пользоваться: вкладки, импорт выписок, лица и алиасы |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | как устроено: пакеты, базы, потоки данных, правила, ловушки |
| [docs/WORKFLOW.md](docs/WORKFLOW.md) | git-регламент, версии, ветки, `clc.sh`, CI |
| [CHANGELOG.md](CHANGELOG.md) | что и когда менялось |

## Тесты

```bash
cd backend && go test ./...
cd frontend && bun run test      # node --test tests/*.test.mjs
cd frontend && bun run build
```

Те же проверки (плюс `gofmt` и гигиена: нет баз, ключей, архивов и номеров карт) гоняет CI на каждый PR и на push в `dev`, `build`, `main`.

## Приватность

В репозитории нет баз, ключей, выписок и реальных данных: тестовые выписки и фикстуры анонимные. `cache.db`, `core.db`, `.env` и `.zoe`-файлы остаются у тебя на машине.
