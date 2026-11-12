import { ref } from 'vue'
import { apiGetJson, apiJson } from '../utils/api.js'
import { storageGet, storageSet } from '../utils/storage.js'
import { parseIsins } from '../utils/marketTable.js'

// Свой список бумаг, доступных в брокере (ICU), с ценой и доходностью с его экрана, и кривая доходности рынка.
// Всё хранится на сервере (core.db: списки «icu» и кривая «icu»), переживает смену браузера и «стереть кеш».
const LIST = '/market/lists?name=icu'
const CURVE = '/market/curve?name=icu'

export function useIcuList() {
  const isins = ref([])
  const prices = ref({}) // ISIN → цена за бумагу (с накопленным купоном)
  const ytms = ref({}) // ISIN → доходность с экрана брокера, эффективная годовая, %
  const curve = ref([]) // [{ years, ytm }]
  const error = ref('')

  const apply = (r) => {
    isins.value = r.isins || []
    prices.value = r.prices || {}
    ytms.value = r.ytms || {}
  }

  const save = async ({ isins: ids, prices: p = {}, ytms: y = {} }) => {
    apply(await apiJson('PUT', LIST, { isins: ids, prices: p, ytms: y }))
  }

  const load = async () => {
    try {
      apply(await apiGetJson(LIST))
      const old = parseIsins(storageGet('zoe.market.icu', '')) // список из первой версии (только браузер): переносим на сервер один раз
      if (!isins.value.length && old.length) { await save({ isins: old }); storageSet('zoe.market.icu', '') }
      curve.value = (await apiGetJson(CURVE)).points || []
      error.value = ''
    } catch (e) {
      error.value = String(e.message || e)
    }
  }

  const saveCurve = async (points) => {
    curve.value = (await apiJson('PUT', CURVE, { points })).points || []
  }

  return { isins, prices, ytms, curve, error, load, save, saveCurve }
}
