import { ref, computed } from 'vue'
import { apiGetJson, apiJson, apiSend, q } from '../utils/api.js'
import { errText } from '../utils/errText.js'

// Учёт одного дела: счета дела, бейджи на операциях и понедельные записи. Данные читаются с бэкенда, правки применяются после его ответа.
// get/send/sendRaw подменяются в тестах.
export function useWorkspaceLedger(wsId, get = apiGetJson, send = apiJson, sendRaw = apiSend) {
  const accountIds = ref([])
  const marks = ref(new Map()) // id операции → ключ бейджа
  const weeks = ref([])
  const entries = ref([]) // ручные (наличные) записи дела
  const params = ref(new Map()) // числовые параметры дела (ключ → значение), чего нет — страница берёт по умолчанию
  const foreign = ref(new Map()) // id операции → { id, name, color } дела, в котором она уже разобрана (соседние дела)
  const error = ref('')
  let seq = 0

  const load = async () => {
    const my = ++seq
    const id = q(wsId())
    try {
      const [a, m, w, pr, en] = await Promise.all([
        get(`/workspaces/accounts?id=${id}`),
        get(`/workspaces/marks?id=${id}`),
        get(`/workspaces/weeks?id=${id}`),
        get(`/workspaces/params?id=${id}`),
        get(`/workspaces/entries?id=${id}`)
      ])
      if (my !== seq) return
      accountIds.value = Array.isArray(a) ? a : []
      marks.value = new Map((Array.isArray(m) ? m : []).map((x) => [x.tx_id, x.badge]))
      weeks.value = Array.isArray(w) ? w : []
      entries.value = Array.isArray(en) ? en : []
      params.value = new Map((Array.isArray(pr) ? pr : []).map((x) => [x.key, x.value]))
      error.value = ''
    } catch (e) {
      if (my === seq) error.value = errText(e)
    }
    loadForeign(my, id)
  }

  // Операции, уже помеченные соседними делами: только для подсветки в истории, сбой не мешает странице
  const loadForeign = async (my, id) => {
    try {
      const [other, list] = await Promise.all([get(`/workspaces/marks?id=${id}&other=1`), get('/workspaces')])
      if (my !== seq) return
      const byId = new Map((Array.isArray(list) ? list : []).map((w) => [w.id, w]))
      const m = new Map()
      for (const x of Array.isArray(other) ? other : []) {
        const w = byId.get(x.workspace_id)
        if (w && x.workspace_id !== wsId()) m.set(x.tx_id, { id: w.id, name: w.name, color: w.color || '' })
      }
      foreign.value = m
    } catch { /* подсветка необязательна */ }
  }

  const linked = computed(() => new Set(accountIds.value))

  // включает/выключает счёт в деле
  const toggleAccount = async (accountId) => {
    const next = linked.value.has(accountId) ? accountIds.value.filter((x) => x !== accountId) : [...accountIds.value, accountId]
    await sendRaw('PUT', '/workspaces/accounts', { workspace_id: wsId(), accounts: next })
    accountIds.value = next
  }

  // ставит бейдж на операцию; тот же бейдж повторно — снимает
  const setMark = async (txId, badge) => {
    const cur = marks.value.get(txId)
    const next = cur === badge ? '' : badge
    await sendRaw('PUT', '/workspaces/marks', { workspace_id: wsId(), tx_id: txId, badge: next })
    const m = new Map(marks.value)
    if (next) m.set(txId, next)
    else m.delete(txId)
    marks.value = m
    if (next && foreign.value.has(txId)) { // операция переехала в это дело
      const f = new Map(foreign.value)
      f.delete(txId)
      foreign.value = f
    }
  }

  // записывает или заменяет неделю (week — понедельник)
  const setWeek = async (week, nominal) => {
    await sendRaw('PUT', '/workspaces/weeks', { workspace_id: wsId(), week, nominal })
    const rest = weeks.value.filter((x) => x.week !== week)
    weeks.value = [...rest, { week, nominal }].sort((a, b) => (a.week < b.week ? -1 : 1))
  }

  const removeWeek = async (week) => {
    await sendRaw('DELETE', `/workspaces/weeks?id=${q(wsId())}&week=${q(week)}`)
    weeks.value = weeks.value.filter((x) => x.week !== week)
  }

  // записывает параметр дела; сброс возвращает значение по умолчанию
  const setParam = async (key, value) => {
    await sendRaw('PUT', '/workspaces/params', { workspace_id: wsId(), key, value })
    params.value = new Map(params.value).set(key, value)
  }

  const resetParam = async (key) => {
    await sendRaw('DELETE', `/workspaces/params?id=${q(wsId())}&key=${q(key)}`)
    const m = new Map(params.value)
    m.delete(key)
    params.value = m
  }

  // ручные записи: добавить (id даёт сервер), поправить, удалить
  const sortEntries = (l) => [...l].sort((a, b) => (a.day < b.day ? -1 : a.day > b.day ? 1 : a.id < b.id ? -1 : 1))
  const addEntry = async (e) => {
    const out = await send('POST', '/workspaces/entries', { workspace_id: wsId(), day: e.day, badge: e.badge, amount: e.amount, note: e.note, tx_id: e.tx_id || '' })
    entries.value = sortEntries([...entries.value, out])
  }
  const updateEntry = async (e) => {
    await sendRaw('PUT', '/workspaces/entries', { workspace_id: wsId(), id: e.id, day: e.day, badge: e.badge, amount: e.amount, note: e.note, tx_id: e.tx_id || '' })
    entries.value = sortEntries(entries.value.map((x) => (x.id === e.id ? { ...e } : x)))
  }
  const removeEntry = async (id) => {
    await sendRaw('DELETE', `/workspaces/entries?id=${q(wsId())}&entry=${q(id)}`)
    entries.value = entries.value.filter((x) => x.id !== id)
  }

  return { accountIds, linked, marks, foreign, weeks, params, entries, error, load, toggleAccount, setMark, setWeek, removeWeek, setParam, resetParam, addEntry, updateEntry, removeEntry }
}
