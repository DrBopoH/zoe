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

// Сводный таймлайн: общая ось от самой старой даты среди ВСЕХ счетов до now, чтобы выбор счетов её не двигал.
// Каждый счёт кладёт на ось свои отрезки (cells: '1' данные, '2' снято без операций, '0' нет данных) прозрачностью 1/n,
// где n — число рисуемых счетов; штриховка (kind '2') идёт отдельным слоем поверх всех заливок.
// all — все счета; drawn — те, что рисуем (выбранные или все, если ничего не выбрано).
export const summaryLayers = (all, drawn, now = 0) => {
  const withAxis = (all || []).filter(a => a.axis_to > a.axis_from && a.cells)
  if (!withAxis.length) return { from: 0, to: 0, fills: [], hatches: [] }
  const from = Math.min(...withAxis.map(a => a.axis_from))
  const to = Math.max(now || 0, ...withAxis.map(a => a.axis_to))
  const span = to - from || 1
  const list = (drawn || []).filter(a => a.axis_to > a.axis_from && a.cells)
  const alpha = list.length ? 1 / list.length : 1
  const fills = []
  const hatches = []
  for (const a of list) {
    const color = accountLook(a).color
    const len = a.cells.length
    const t = (i) => (a.axis_from + (i / len) * (a.axis_to - a.axis_from) - from) / span * 100
    let i = 0
    while (i < len) {
      let j = i
      while (j < len && a.cells[j] === a.cells[i]) j++
      const kind = a.cells[i]
      if (kind === '1' || kind === '2') {
        const run = { left: t(i), width: t(j) - t(i), color, alpha }
        ;(kind === '1' ? fills : hatches).push(run)
      }
      i = j
    }
  }
  return { from, to, fills, hatches }
}
