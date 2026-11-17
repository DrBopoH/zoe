import { ref } from 'vue'
import { apiGetJson, q } from '../utils/api.js'

// Раздел «Акции компаний»: список компаний (поиск по тикеру и названию, на сервере) и одна открытая компания.
// Ответы приходят вразнобой (печатают быстрее, чем отвечает сервер): засчитывается только последний запрос каждого вида.
export const STOCK_LIST_LIMIT = 40

export function useStocks(get = apiGetJson) {
  const list = ref([])
  const detail = ref(null)
  const loading = ref(false)
  const error = ref('')
  let listSeq = 0
  let openSeq = 0

  const search = async (text = '') => {
    const my = ++listSeq
    try {
      const r = await get(`/market/companies?q=${q(text.trim())}&limit=${STOCK_LIST_LIMIT}`)
      if (my === listSeq) { list.value = Array.isArray(r) ? r : []; error.value = '' }
    } catch (e) {
      if (my === listSeq) error.value = String(e.message || e)
    }
  }

  const open = async (ticker) => {
    if (!ticker) { detail.value = null; return }
    const my = ++openSeq
    loading.value = true
    try {
      const r = await get(`/market/stock?ticker=${q(ticker)}`)
      if (my === openSeq) { detail.value = r; error.value = '' }
    } catch (e) {
      if (my === openSeq) { detail.value = null; error.value = String(e.message || e) }
    } finally {
      if (my === openSeq) loading.value = false
    }
  }

  return { list, detail, loading, error, search, open }
}
