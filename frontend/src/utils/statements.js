// Окно «Выписки»: как показать счета из /api/sources — группы по имени файла, банк счёта, подпись.
import { BANKS } from './banks.js'
import { detectBankKey } from './personLayout.js'
import { bankTheme } from './bankTheme.js'

// «Volunets Taras.pdf (privat)» → «Volunets Taras.pdf»: хвост в скобках срезается, только если это тип самого счёта
// (privat, raif, black, fop, icu, binance …). Любые другие скобки в имени — часть имени.
export const stripTag = (name, type) => {
  const s = String(name || '')
  const m = s.match(/^(.*?)\s*\(([^()]*)\)\s*$/)
  return m && type && m[2].trim().toLowerCase() === String(type).toLowerCase() ? m[1] : s
}

// Банк счёта: тип банка из выписки (privat, raif …) главнее всего; у счетов Mono тип — вид карты (black, fop), банк по МФО из IBAN
export const bankKeyOf = (a) => {
  const type = String(a.type || '').toLowerCase()
  if (BANKS[type]) return type
  if (type === 'binance') return 'binance'
  return detectBankKey(a.label || '', { type }, [])
}

// Цвет и значок счёта: цвет красит таймлайн покрытия, значок (PRIVAT, RAIF, OVDP, HOT …) — заголовок группы
export const accountLook = (a) => {
  const key = bankKeyOf(a)
  const theme = bankTheme(key, a.type)
  return { key, color: theme.color, tag: theme.tag, tagStyle: theme.tagStyle }
}

// Значки группы: один файл выписки — как правило один банк, но если типы разные, покажем каждый один раз
export const groupBadges = (g) => {
  const seen = new Map()
  for (const a of g.accounts) {
    const l = accountLook(a)
    if (l.tag && !seen.has(l.tag)) seen.set(l.tag, { tag: l.tag, style: l.tagStyle })
  }
  return [...seen.values()]
}

// Группы по имени файла (без хвоста-тега): один файл выписки с несколькими счетами — одна группа.
// Порядок групп — по имени, счета внутри — как пришли.
export const groupAccounts = (accounts) => {
  const by = new Map()
  for (const a of accounts || []) {
    const name = stripTag(a.name || a.id, a.type)
    if (!by.has(name)) by.set(name, { name, accounts: [] })
    by.get(name).accounts.push(a)
  }
  return [...by.values()].sort((x, y) => x.name.localeCompare(y.name, 'ru'))
}
