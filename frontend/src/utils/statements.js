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
// Ось режется на отрезки по границам всех периодов. На каждом отрезке цвет — среднее цветов счетов, у которых там есть
// данные, а непрозрачность растёт с долей таких счетов (MIN_ALPHA..1): при десятках счетов бар остаётся читаемым.
// Штриховка (период снят, операций нет) — отдельные отрезки по тому же правилу, рисуются поверх заливок.
// all — все счета; drawn — рисуемые (выбранные или все, если ничего не выбрано).
const MIN_ALPHA = 0.35

const hexRgb = (h) => {
  const m = /^#?([0-9a-f]{6})$/i.exec(String(h || ''))
  if (!m) return [100, 116, 139]
  const n = parseInt(m[1], 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}
const meanColor = (colors) => {
  const sum = colors.map(hexRgb).reduce((a, c) => [a[0] + c[0], a[1] + c[1], a[2] + c[2]], [0, 0, 0])
  return `rgb(${sum.map(v => Math.round(v / colors.length)).join(' ')})`
}

export const summaryLayers = (all, drawn, now = 0) => {
  const ok = (a) => a.axis_to > a.axis_from && a.cells
  const axis = (all || []).filter(ok)
  if (!axis.length) return { from: 0, to: 0, fills: [], hatches: [] }
  const from = Math.min(...axis.map(a => a.axis_from))
  const to = Math.max(now || 0, ...axis.map(a => a.axis_to))
  const span = to - from || 1
  const list = (drawn || []).filter(ok).map(a => ({ a, color: accountLook(a).color }))
  const n = list.length
  if (!n) return { from, to, fills: [], hatches: [] }

  // Границы отрезков в долях оси
  const cuts = new Set([0, 1])
  for (const { a } of list) {
    for (let i = 0; i <= a.cells.length; i++) {
      const x = (a.axis_from + (i / a.cells.length) * (a.axis_to - a.axis_from) - from) / span
      if (x >= 0 && x <= 1) cuts.add(x)
    }
  }
  const xs = [...cuts].sort((p, q) => p - q)

  const fills = []
  const hatches = []
  const push = (arr, x0, x1, color, alpha) => {
    const last = arr[arr.length - 1]
    const w = (x1 - x0) * 100
    if (last && last.color === color && last.alpha === alpha && Math.abs(last.left + last.width - x0 * 100) < 1e-9) last.width += w
    else arr.push({ left: x0 * 100, width: w, color, alpha })
  }
  for (let k = 0; k + 1 < xs.length; k++) {
    const mid = from + ((xs[k] + xs[k + 1]) / 2) * span
    const data = []
    const empty = []
    for (const { a, color } of list) {
      if (mid < a.axis_from || mid >= a.axis_to) continue
      const kind = a.cells[Math.floor(((mid - a.axis_from) / (a.axis_to - a.axis_from)) * a.cells.length)]
      if (kind === '1') data.push(color)
      else if (kind === '2') empty.push(color)
    }
    const alphaOf = (c) => Math.round((MIN_ALPHA + (1 - MIN_ALPHA) * (c.length / n)) * 1000) / 1000
    if (data.length) push(fills, xs[k], xs[k + 1], meanColor(data), alphaOf(data))
    if (empty.length) push(hatches, xs[k], xs[k + 1], meanColor(empty), alphaOf(empty))
  }
  return { from, to, fills, hatches }
}
