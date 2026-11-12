import { ref } from 'vue'
import { apiSend } from '../utils/api.js'
import { feedOf } from '../utils/marketNav.js'

// Кнопка «Обновить» раздела рынка: запускает опрос источника этого раздела (POST /market/sync?source=…),
// ход и итог идут отдельным окном и тостами. section — функция, возвращающая ключ раздела.
export function useMarketRefresh(section) {
  const startError = ref('')
  const refresh = async () => {
    startError.value = ''
    try {
      await apiSend('POST', `/market/sync?source=${feedOf(section())}`)
    } catch (e) {
      startError.value = String(e.message || e)
    }
  }
  return { refresh, startError }
}
