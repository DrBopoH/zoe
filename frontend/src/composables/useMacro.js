import { ref, shallowRef } from 'vue'
import { apiGetJson } from '../utils/api.js'
import { withTicks } from '../utils/macro.js'

// Какие ряды даёт каждый источник, знает сервер (GET /api/macro/catalog): фронт свои списки id не держит. Спрашиваем один раз на все экраны.
let catalog = null
export const resetCatalog = () => { catalog = null }
const loadCatalog = (get) => (catalog ||= get('/macro/catalog').then((r) => ({ feeds: r?.feeds || {}, countries: r?.countries || [], snapshots: r?.snapshots || [] })).catch((e) => { catalog = null; throw e }))
export const feedSeries = (feed, get = apiGetJson) => loadCatalog(get).then((c) => c.feeds[feed] || [])
// Реестр стран с сервера: [{cc, name, currency, metrics: {policy: 'fed_funds', …}}] — какой ряд отвечает за какой показатель карточки
// Ряды-снимки (источник отдаёт только «сейчас»): сервер копит их по часам, список — тоже оттуда
export const snapshotIds = (get = apiGetJson) => loadCatalog(get).then((c) => c.snapshots)
export const countryList = (get = apiGetJson) => loadCatalog(get).then((c) => c.countries)

// Ряды источника (ставки и инфляция — 'macro', курсы — 'fx', США — 'us', …) из кеша сервера: GET /api/macro. Заполняются «Обновить»/«Разведать».
// extra — ряды сверх источника (например инфляция США рядом с курсами). Курсы и США тяжёлые, поэтому у каждого экрана свой вызов.
// Часовые точки рядов-снимков из GET /api/macro/ticks: ряд без точек или с ошибкой остаётся дневным
export async function loadTicks(ids, get = apiGetJson) {
  const snaps = (await snapshotIds(get)).filter((id) => ids.includes(id))
  const out = {}
  await Promise.all(snaps.map(async (id) => { out[id] = await get(`/macro/ticks?series=${id}`).catch(() => []) }))
  return out
}

export function useMacro(feed = 'macro', extra = [], { ticks = false } = {}) {
  // shallowRef: ряды — сотни тысяч точек, которые только читаются и заменяются целиком; глубокая реактивность делала каждое чтение через Proxy (индекс доллара ≈ в 35 раз медленнее)
  const series = shallowRef({}) // id → [{ d: 'YYYY-MM-DD', v }]
  const updatedAt = ref(0)
  const error = ref('')

  const load = async () => {
    try {
      const ids = [...(await feedSeries(feed)), ...extra]
      const r = await apiGetJson(`/macro?series=${ids.join(',')}`)
      series.value = ticks ? withTicks(r?.series || {}, await loadTicks(ids)) : r?.series || {}
      updatedAt.value = r?.updated_at || 0
      error.value = ''
    } catch (e) {
      error.value = String(e.message || e)
    }
  }

  return { series, updatedAt, error, load }
}
