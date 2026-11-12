import { ref } from 'vue'
import { apiGetJson } from '../utils/api.js'

// Макро-ряды НБУ (ставки, инфляция) из кеша сервера: GET /api/macro. Заполняются «Обновить»/«Разведать».
export function useMacro() {
  const series = ref({}) // id → [{ d: 'YYYY-MM-DD', v }]
  const updatedAt = ref(0)
  const error = ref('')

  const load = async () => {
    try {
      const r = await apiGetJson('/macro')
      series.value = r?.series || {}
      updatedAt.value = r?.updated_at || 0
      error.value = ''
    } catch (e) {
      error.value = String(e.message || e)
    }
  }

  return { series, updatedAt, error, load }
}
