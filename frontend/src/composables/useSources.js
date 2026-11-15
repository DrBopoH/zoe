import { ref } from 'vue'
import { apiGetJson } from '../utils/api.js'

// Здоровье источников (GET /api/macro/sources): что обещает каждый источник, что лежит в кеше, свежесть рядов, последняя ошибка.
// Поле называется series, чтобы раздел грузился тем же правилом, что и остальные тяжёлые экраны рынка (пусто — читаем при первом открытии).
export function useSources() {
  const series = ref({})
  const error = ref('')
  const load = async () => {
    try {
      series.value = (await apiGetJson('/macro/sources')) || {}
      error.value = ''
    } catch (e) {
      error.value = String(e.message || e)
    }
  }
  return { series, error, load }
}
