import { ref } from 'vue'
import { apiGetJson, apiJson, apiSend, q } from '../utils/api.js'
import { errText } from '../utils/errText.js'

// Дела (рабочие пространства): список с бэкенда, создание, правка и удаление. Ответы приходят вразнобой — засчитывается только последний запрос списка.
export function useWorkspaces(get = apiGetJson, send = apiJson, sendRaw = apiSend) {
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
  const create = async ({ name, color = '', image = '', page = '' }) => {
    const ws = await send('POST', '/workspaces', { name, color, image, page })
    list.value = [...list.value, ws]
    return ws
  }

  // Правит дело (имя, цвет, обложка, страница) и обновляет его в списке; ошибку пробрасывает наружу
  const update = async (id, { name, color = '', image = '', page = '' }) => {
    await send('PUT', '/workspaces', { id, name, color, image, page })
    list.value = list.value.map((w) => (w.id === id ? { ...w, name, color, image, page } : w))
  }

  // Удаляет дело; из списка убирается после ответа сервера
  const remove = async (id) => {
    await sendRaw('DELETE', `/workspaces?id=${q(id)}`)
    list.value = list.value.filter((w) => w.id !== id)
  }

  return { list, loading, error, load, create, update, remove }
}
