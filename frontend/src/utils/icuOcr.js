// Разбор распознанного текста скриншота экрана брокера («Купити → ОВДП»): карточка на бумагу, в ней ISIN, YTM (или SIM
// для коротких), дата погашения и цена за бумагу. Распознавание путает цифры, поэтому ничего не принимается на веру:
// ISIN сверяется с реестром НБУ по дате погашения, цена — с показанной доходностью.
import { ytmFromPrice } from './marketTable.js'

const DIGIT_LIKE = { O: '0', o: '0', Q: '0', D: '0', l: '1', I: '1', i: '1', '|': '1', '!': '1', S: '5', s: '5', B: '8', b: '6', g: '9', q: '9', Z: '2', z: '2' }
const toDigits = (s) => [...s].map((c) => (/\d/.test(c) ? c : DIGIT_LIKE[c] ?? c)).join('')

const dateToIso = (d, m, y) => `${y}-${m}-${d}`
const DATE_RE = /(\d{2})\.(\d{2})\.(\d{4})/
const YTM_RE = /\b(YTM|SIM)\s*([\d.,]+)\s*%/i

const hamming = (a, b) => (a.length !== b.length ? 99 : [...a].reduce((n, c, i) => n + (c !== b[i] ? 1 : 0), 0))

// Варианты цены из строки «Кількість: 9 566 1 038,68 ₴» → [38.68, 1038.68, ...]. Количество слева от цены и пробелы в тысячах
// не отличить по виду, а распознавание ещё и вставляет лишние пробелы («1 1 1 7,72»), поэтому берём все склейки цифр
// справа налево; какая из них настоящая, решает проверка по YTM и разумному диапазону.
export function priceCandidates(line) {
  const part = String(line || '').split(':').slice(-1)[0].replace(/,\s*(\d)\s+(\d)(?!\d)/g, ',$1$2') // «077,1 9» → «077,19»
  const toks = part.match(/\d+(?:,\d+)?/g)
  if (!toks) return []
  let m = -1
  toks.forEach((t, i) => { if (/^\d*,\d{2,3}$/.test(t)) m = i })
  if (m < 0) return []
  const [int, frac] = toks[m].split(',')
  const out = []
  let whole = int
  for (let k = m - 1; k >= -1 && out.length < 6; k--) {
    const v = Number(`${whole}.${frac.slice(0, 2)}`)
    if (Number.isFinite(v) && !out.includes(v)) out.push(v)
    if (k >= 0) whole = toks[k] + whole
  }
  return out
}

// Первый вариант, который выглядит как цена (для проверок и подсказок); окончательный выбор — в parseIcuScreen
export const priceFromLine = (line, nominal = 1000) => priceCandidates(line).find((v) => v >= nominal * 0.8 && v <= nominal * 1.3) ?? null

// Эффективная годовая из простой (SIM у бумаг короче года): рост за срок 1 + s·t, в годовые
export const simpleToEffective = (s, years) => (years > 0 && years < 1 ? (Math.pow(1 + (s / 100) * years, 1 / years) - 1) * 100 : s)

// Найти бумагу реестра по распознанному ISIN и дате погашения. Дата точнее ISIN: цифры в ней читаются надёжно.
function matchBond(token, iso, registry) {
  const tail = toDigits(String(token || '').replace(/[^A-Za-z0-9|!]/g, '')).slice(-10)
  const byDate = iso ? registry.filter((b) => b.maturity === iso) : []
  const pool = byDate.length ? byDate : registry
  let best = null, bestD = 99, ties = 0
  for (const b of pool) {
    const d = hamming(tail, String(b.id).slice(2))
    if (d < bestD) { best = b; bestD = d; ties = 1 } else if (d === bestD) ties++
  }
  if (best && (byDate.length ? true : bestD <= 2)) {
    const flags = []
    if (bestD > 0) flags.push('isin-fixed')
    if (ties > 1) flags.push('isin-ambiguous')
    return { bond: best, flags }
  }
  if (/^\d{10}$/.test(tail)) return { bond: { id: `UA${tail}`, maturity: iso || '' }, flags: ['not-in-registry'] }
  return null
}

/**
 * text — распознанный текст одного скриншота; registry — бумаги реестра НБУ [{id, maturity, coupon_rate, nominal}].
 * Возвращает [{ isin, maturity, kind: 'YTM'|'SIM', shown (как на экране), ytm (эффективная, %), price|null, flags[] }].
 * flags: isin-fixed (ISIN исправлен по реестру), isin-ambiguous, not-in-registry, price-bad (цена отброшена, остался YTM),
 * price-none (цены на экране не нашли).
 */
export function parseIcuScreen(text, registry, today) {
  const lines = String(text || '').split(/\r?\n/)
  const starts = []
  lines.forEach((l, i) => { if (YTM_RE.test(l)) starts.push(i) })
  const out = []
  starts.forEach((i, k) => {
    const end = k + 1 < starts.length ? starts[k + 1] : lines.length
    const m = lines[i].match(YTM_RE)
    const kind = m[1].toUpperCase()
    const shown = Number(m[2].replace(',', '.'))
    if (!(shown > 0 && shown < 100)) return
    const token = lines[i].slice(0, m.index).split(/[\s\\/"'~_-]+/).filter((t) => t.length >= 9).pop() || ''
    const block = lines.slice(i + 1, end).join('\n')
    const d = block.match(DATE_RE) || (i > 0 ? lines[i - 1].match(DATE_RE) : null)
    const iso = d ? dateToIso(d[1], d[2], d[3]) : ''
    const hit = matchBond(token, iso, registry)
    if (!hit) return
    const b = hit.bond
    const flags = [...hit.flags]
    const nominal = b.nominal > 0 ? b.nominal : 1000
    const years = b.maturity ? (Date.parse(`${b.maturity}T00:00:00Z`) - Date.parse(`${today}T00:00:00Z`)) / (365.25 * 86400000) : 0
    const ytm = kind === 'SIM' ? simpleToEffective(shown, years) : shown
    let price = null
    const cands = []
    for (const l of block.split('\n')) if (/:/.test(l)) cands.push(...priceCandidates(l))
    if (!cands.length) flags.push('price-none')
    else {
      const inRange = cands.filter((v) => v >= nominal * 0.8 && v <= nominal * 1.3)
      if (kind === 'YTM' && b.maturity) {
        // цена и YTM с одного экрана обязаны сходиться: выбираем вариант, который даёт показанную доходность
        let bestD = 0.4
        for (const v of inRange) {
          const y = ytmFromPrice({ maturity: b.maturity, coupon_rate: b.coupon_rate, nominal }, v, today)
          if (y !== null && Math.abs(y - shown) <= bestD) { bestD = Math.abs(y - shown); price = v }
        }
      } else if (inRange.length === 1) price = inRange[0] // у SIM проверить нечем: берём только однозначную цену
      if (price === null) flags.push('price-bad')
    }
    out.push({ isin: b.id, maturity: b.maturity || iso, kind, shown, ytm, price, flags })
  })
  return out
}

// Лучшая из нескольких записей одной бумаги (разные проходы распознавания, перекрывающиеся скриншоты): с ценой лучше, чем без;
// при равенстве — без пометок о проблемах с ISIN. Порядок бумаг — по первому появлению.
export function mergeEntries(lists) {
  const best = new Map()
  const score = (e) => (e.price !== null ? 4 : 0) + (e.flags.includes('isin-ambiguous') || e.flags.includes('not-in-registry') ? 0 : 1)
  for (const list of lists) {
    for (const e of list) {
      const cur = best.get(e.isin)
      if (!cur || score(e) > score(cur)) best.set(e.isin, e)
    }
  }
  return [...best.values()]
}
