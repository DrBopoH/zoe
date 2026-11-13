import { apiSend } from '../utils/api.js'
import { feedOf } from '../utils/marketNav.js'
import { pushRequestError } from './useToasts.js'

// Кнопка «Обновить» раздела рынка: запускает опрос источника этого раздела (POST /market/sync?source=…),
// ход и итог идут отдельным окном и тостами. section — функция, возвращающая ключ раздела.
// Отказ запуска (связь, «уже идёт») — тоже тост: он сам уходит, а не висит плашкой над разделом.
export function useMarketRefresh(section) {
  const refresh = async () => {
    try {
      await apiSend('POST', `/market/sync?source=${feedOf(section())}`)
    } catch (e) {
      pushRequestError('Не удалось обновить', e)
    }
  }
  return { refresh }
}
