import { ref } from 'vue'
import { apiGetJson, apiJson } from '../utils/api.js'
import { errText } from '../utils/errText.js'

// Дела (рабочие пространства): список с бэкенда и создание. Ответы приходят вразнобой — засчитывается только последний запрос списка.
export function useWorkspaces(get = apiGetJson, send = apiJson) {
  const list = ref([])
  const loading = ref(false)
  const error = ref('')
  let seq = 0

  const load = async () => {
    const my = ++seq
    loading.value = true
    try {
      const r = await get('/workspaces')
      if (my === seq) { list.value = Array.isArray(r) ? r : []; error.value = '' }
    } catch (e) {
      if (my === seq) error.value = errText(e)
    } finally {
      if (my === seq) loading.value = false
    }
  }

  // Создаёт дело и добавляет в конец списка; ошибку пробрасывает наружу — окно создания остаётся открытым и показывает тост
  const create = async ({ name, color = '', image = '' }) => {
    const ws = await send('POST', '/workspaces', { name, color, image })
    list.value = [...list.value, ws]
    return ws
  }

  return { list, loading, error, load, create }
}
