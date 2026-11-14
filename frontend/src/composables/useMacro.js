import { ref } from 'vue'
import { apiGetJson } from '../utils/api.js'

// Какие ряды даёт каждый источник, знает сервер (GET /api/macro/catalog): фронт свои списки id не держит. Спрашиваем один раз на все экраны.
let catalog = null
export const resetCatalog = () => { catalog = null }
export function feedSeries(feed, get = apiGetJson) {
  catalog ||= get('/macro/catalog').then((r) => r?.feeds || {}).catch((e) => { catalog = null; throw e })
  return catalog.then((feeds) => feeds[feed] || [])
}

// Ряды источника (ставки и инфляция — 'macro', курсы — 'fx', США — 'us', …) из кеша сервера: GET /api/macro. Заполняются «Обновить»/«Разведать».
// extra — ряды сверх источника (например инфляция США рядом с курсами). Курсы и США тяжёлые, поэтому у каждого экрана свой вызов.
export function useMacro(feed = 'macro', extra = []) {
  const series = ref({}) // id → [{ d: 'YYYY-MM-DD', v }]
  const updatedAt = ref(0)
  const error = ref('')

  const load = async () => {
    try {
      const ids = [...(await feedSeries(feed)), ...extra]
      const r = await apiGetJson(`/macro?series=${ids.join(',')}`)
      series.value = r?.series || {}
      updatedAt.value = r?.updated_at || 0
      error.value = ''
    } catch (e) {
      error.value = String(e.message || e)
    }
  }

  return { series, updatedAt, error, load }
}
