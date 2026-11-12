import { ref } from 'vue'
import { apiGetJson, apiSend } from '../utils/api.js'

// Скрытые виды IBAN из core.db — общие на весь экран (счета рисуются в разных местах, прокидывать пропсами незачем).
// Нет записи на бэкенде — он сам генерирует маску из цифр; тут только читаем и пишем свой псевдоним.
const masks = ref({}) // normID(iban) → { masked, custom }
let loading = null

const norm = (s) => String(s || '').replace(/[^a-z0-9]/gi, '').toUpperCase()

export const loadMasks = async () => {
  try {
    masks.value = (await apiGetJson('/iban-masks')) || {}
  } catch (err) {
    console.error('IBAN masks load error:', err)
  }
}

export function useIbanMasks() {
  if (!loading) loading = loadMasks()
  const maskOf = (iban) => masks.value[norm(iban)] || null
  // псевдоним-имя счёта; пустая строка — вернуть автоматическую маску
  const saveMask = async (iban, masked) => {
    const res = await apiSend('PUT', '/iban-masks', { iban, masked })
    masks.value = { ...masks.value, [norm(iban)]: await res.json() }
  }
  return { maskOf, saveMask }
}
