# Ζωή — фронтенд

Vue 3 (`<script setup>`), Tailwind v4, Vite, bun. Общее описание проекта: [../README.ru.md](../README.ru.md), устройство фронтенда: [../docs/ARCHITECTURE.ru.md](../docs/ARCHITECTURE.ru.md#10-фронтенд).

```bash
bun install            # зависимости (bun.lock фиксирует версии)
bun run dev            # http://localhost:5173, бэкенд ждёт на http://127.0.0.1:8080/api
bun run test           # node --test tests/*.test.mjs и vitest run
bun run build          # сборка в dist/
```

Адрес бэкенда меняется переменной `VITE_API_BASE` (по умолчанию `http://127.0.0.1:8080/api`).

```
src/App.vue            оболочка: шапка, боковое меню, страницы, опрос бэкенда
src/views/             страницы: Главная, Смета, Рынок (Macro*View), Учет
src/components/        панели, модалки, графики; ui/ — мелкие кирпичики
src/composables/       состояние и логика (useMacro, useChartView, usePaged, …)
src/utils/             чистые функции: форматтеры (numfmt), API-слой, навигация, математика графиков
tests/                 node --test (логика) и tests/components (vitest)
```
