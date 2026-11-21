import { ref, computed } from 'vue'
import { apiGetJson, apiJson, apiSend, q } from '../utils/api.js'
import { errText } from '../utils/errText.js'

// Учёт одного дела: счета дела, бейджи на операциях и понедельные записи. Данные читаются с бэкенда, правки применяются после его ответа.
// get/send/sendRaw подменяются в тестах.
export function useWorkspaceLedger(wsId, get = apiGetJson, send = apiJson, sendRaw = apiSend) {
  const accountIds = ref([])
  const marks = ref(new Map()) // id операции → ключ бейджа
  const weeks = ref([])
  const params = ref(new Map()) // числовые параметры дела (ключ → значение), чего нет — страница берёт по умолчанию
  const error = ref('')
  let seq = 0

  const load = async () => {
    const my = ++seq
    const id = q(wsId())
    try {
      const [a, m, w, pr] = await Promise.all([
        get(`/workspaces/accounts?id=${id}`),
        get(`/workspaces/marks?id=${id}`),
        get(`/workspaces/weeks?id=${id}`),
        get(`/workspaces/params?id=${id}`)
      ])
      if (my !== seq) return
      accountIds.value = Array.isArray(a) ? a : []
      marks.value = new Map((Array.isArray(m) ? m : []).map((x) => [x.tx_id, x.badge]))
      weeks.value = Array.isArray(w) ? w : []
      params.value = new Map((Array.isArray(pr) ? pr : []).map((x) => [x.key, x.value]))
      error.value = ''
    } catch (e) {
      if (my === seq) error.value = errText(e)
    }
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

  return { accountIds, linked, marks, weeks, params, error, load, toggleAccount, setMark, setWeek, removeWeek, setParam, resetParam }
}
