import { ref, shallowRef } from 'vue'
import { apiGetJson } from '../utils/api.js'
import { countryList } from './useMacro.js'
import { countryIds } from '../utils/countries.js'

// Карточки стран: список и показатели берутся с сервера (реестр в /api/macro/catalog), ряды — из кеша (/api/macro) одним запросом.
export function useCountries() {
  const countries = ref([])
  const series = shallowRef({}) // ряды только читаются и заменяются целиком: без глубокой реактивности (см. useMacro)
  const updatedAt = ref(0)
  const error = ref('')

  const load = async () => {
    try {
      countries.value = await countryList()
      const ids = countryIds(countries.value)
      const r = ids.length ? await apiGetJson(`/macro?series=${ids.join(',')}`) : null
      series.value = r?.series || {}
      updatedAt.value = r?.updated_at || 0
      error.value = ''
    } catch (e) {
      error.value = String(e.message || e)
    }
  }
  return { countries, series, updatedAt, error, load }
}
