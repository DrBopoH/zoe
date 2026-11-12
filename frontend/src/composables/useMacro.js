import { ref } from 'vue'
import { apiGetJson } from '../utils/api.js'
import { MACRO_IDS } from '../utils/macro.js'

// Макро-ряды НБУ (ставки, инфляция) из кеша сервера: GET /api/macro. Заполняются «Обновить»/«Разведать».
// ids — какие ряды читать (по умолчанию ставки и инфляция; курсы тяжёлые и грузятся отдельным вызовом с FX_IDS)
export function useMacro(ids = MACRO_IDS) {
  const series = ref({}) // id → [{ d: 'YYYY-MM-DD', v }]
  const updatedAt = ref(0)
  const error = ref('')

  const load = async () => {
    try {
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
