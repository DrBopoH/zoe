import { ref } from 'vue'
import { apiGetJson, apiJson, apiSend, q } from '../utils/api.js'
import { errText } from '../utils/errText.js'

// Документы одного вида на странице дела (например карточки товаров ХимЛаба). Бэкенд хранит {id, kind, day, data}, здесь запись плоская:
// {id, day, ...поля}. Правки применяются после ответа бэкенда. get/send/sendRaw подменяются в тестах.
const META = new Set(['id', 'kind', 'day', 'workspace_id'])
const flat = (d) => ({ ...d.data, id: d.id, day: d.day || '' })
const body = (e) => Object.fromEntries(Object.entries(e).filter(([k]) => !META.has(k)))
const byDay = (l) => [...l].sort((a, b) => (a.day < b.day ? -1 : a.day > b.day ? 1 : a.id < b.id ? -1 : 1))

export function useWorkspaceDocs(wsId, kind, get = apiGetJson, send = apiJson, sendRaw = apiSend) {
  const docs = ref([])
  const error = ref('')
  let seq = 0

  const load = async () => {
    const my = ++seq
    try {
      const l = await get(`/workspaces/docs?id=${q(wsId())}&kind=${q(kind)}`)
      if (my !== seq) return
      docs.value = (Array.isArray(l) ? l : []).map(flat)
      error.value = ''
    } catch (e) {
      if (my === seq) error.value = errText(e)
    }
  }

  const add = async (e) => {
    const out = await send('POST', '/workspaces/docs', { workspace_id: wsId(), kind, day: e.day || '', data: body(e) })
    docs.value = byDay([...docs.value, flat(out)])
  }
  const update = async (e) => {
    await sendRaw('PUT', '/workspaces/docs', { workspace_id: wsId(), id: e.id, kind, day: e.day || '', data: body(e) })
    docs.value = byDay(docs.value.map((x) => (x.id === e.id ? { ...e } : x)))
  }
  const remove = async (id) => {
    await sendRaw('DELETE', `/workspaces/docs?id=${q(wsId())}&doc=${q(id)}`)
    docs.value = docs.value.filter((x) => x.id !== id)
  }

  return { docs, error, load, add, update, remove }
}
