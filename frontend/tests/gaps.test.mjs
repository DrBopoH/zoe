import test from 'node:test'
import assert from 'node:assert/strict'
import { gapFlags, gapLimit, tailGap } from '../src/utils/gaps.js'
import { buildMacroChart, gapAt, valueAt, curveChart, MACRO_PAD } from '../src/utils/macro.js'

const H = 3600000, D = 86400000
const T0 = Date.parse('2026-11-01T00:00:00Z')
const flagged = (t) => [...gapFlags(t)].map((f, i) => (f ? i : -1)).filter((i) => i >= 0)
const hours = (list) => list.map((h) => T0 + h * H)
const iso = (ms) => new Date(ms).toISOString().slice(0, 16)

test('часовой ряд: выпавший час и 36-часовая дыра — пропуски, обычные часы — нет', () => {
  // 0..9 часы подряд, час 10 выпал, 11..20 подряд, дальше молчание 36 часов, 57..66 подряд
  const hs = []
  for (let h = 0; h <= 9; h++) hs.push(h)
  for (let h = 11; h <= 20; h++) hs.push(h)
  for (let h = 57; h <= 66; h++) hs.push(h)
  const t = hours(hs)
  const f = flagged(t)
  assert.deepEqual(f.map((i) => (t[i] - T0) / H), [11, 57])
})

test('ряд торговых дней: выходные и длинные выходные — не пропуск, а неделя тишины — пропуск', () => {
  const t = []
  let d = Date.parse('2026-01-05T00:00:00Z') // понедельник
  for (let k = 0; k < 60; k++) {
    const dow = new Date(d).getUTCDay()
    if (dow !== 0 && dow !== 6) t.push(d)
    d += D
  }
  assert.deepEqual(flagged(t), [])
  // праздник в понедельник: пятница → вторник (4 суток) — торговый календарь, не пропуск
  const hol = t.filter((x) => new Date(x).getUTCDay() !== 1 || x !== Date.parse('2026-01-19T00:00:00Z'))
  assert.deepEqual(flagged(hol), [])
  // а девять суток без точек — пропуск
  const cut = t.filter((x) => x < Date.parse('2026-01-14T00:00:00Z') || x > Date.parse('2026-01-23T00:00:00Z'))
  const f = flagged(cut)
  assert.equal(f.length, 1)
  assert.equal(iso(cut[f[0]]).slice(0, 10), '2026-01-26')
})

test('месячный ряд: выпавший месяц — пропуск, обычные месяцы разной длины — нет', () => {
  const months = ['2025-01-31', '2025-02-28', '2025-03-31', '2025-04-30', '2025-05-31', '2025-06-30', '2025-08-31', '2025-09-30', '2025-10-31', '2025-11-30']
  const t = months.map((m) => Date.parse(`${m}T00:00:00Z`))
  const f = flagged(t)
  assert.equal(f.length, 1)
  assert.equal(months[f[0]], '2025-08-31')
})

test('смена режима (месяцы → недели) стыком не считается пропуском', () => {
  const t = []
  for (let k = 0; k < 30; k++) t.push(Date.parse('2010-01-01T00:00:00Z') + k * 30 * D)
  const last = t[t.length - 1]
  for (let k = 1; k <= 30; k++) t.push(last + k * 7 * D)
  assert.deepEqual(flagged(t), [])
})

test('короткий ряд судить не по чему, ряд без интервалов тоже', () => {
  assert.deepEqual(flagged(hours([0, 40])), [])
  assert.deepEqual(flagged([]), [])
  assert.deepEqual(flagged(hours([0, 0, 0, 0])), [])
})

test('хвост отчёта: свежий — нет, просроченный — да', () => {
  const rep = [0, 1, 2, 3].map((k) => Date.parse(`${2022 + k}-12-31T00:00:00Z`))
  assert.equal(tailGap(rep, Date.parse('2026-10-31T00:00:00Z')), false) // 10 месяцев после отчёта — год ещё не прошёл
  assert.equal(tailGap(rep, Date.parse('2028-06-30T00:00:00Z')), true)
  assert.equal(tailGap(rep, rep[3]), false)
  assert.equal(tailGap([T0], T0 + 100 * D), false)
  assert.ok(gapLimit(H) < 2 * H && gapLimit(D) >= 5 * D)
})

// ── на графике
const stamp = (ms) => new Date(ms).toISOString().slice(0, 16)
const hourly = (hs) => hs.map((h) => ({ d: stamp(T0 + h * H), v: 46 + (h % 5) * 0.1 }))
const VIEW = { x0: -0.01, x1: 0.005 }
const build = (series, ids, extra = {}) => buildMacroChart({ series, ids, view: { x0: -0.02, x1: 0.01 }, yRange: { lo: 40, hi: 50 }, todayIso: '2026-11-04', ...extra })

test('график: через дыру часового ряда идёт пунктир, сплошная линия рвётся', () => {
  const hs = []
  for (let h = 0; h <= 20; h++) hs.push(h)
  for (let h = 57; h <= 66; h++) hs.push(h)
  const c = build({ p2p_ask: hourly(hs) }, ['p2p_ask'])
  const line = c.lines[0]
  assert.match(line.dg, /^M[\d.]+ [\d.]+L[\d.]+ [\d.]+$/) // ровно один пунктирный отрезок
  assert.equal((line.d.match(/M/g) || []).length, 2) // сплошная линия — два куска
})

test('график: ряд без дыр — без пунктира', () => {
  const hs = []
  for (let h = 0; h <= 40; h++) hs.push(h)
  const c = build({ p2p_ask: hourly(hs) }, ['p2p_ask'])
  assert.equal(c.lines[0].dg, '')
  assert.equal((c.lines[0].d.match(/M/g) || []).length, 1)
})

test('график: ставка держит значение по смыслу, а отчёт, которого нет два года, — пунктиром', () => {
  const rate = [{ d: '2020-01-01', v: 10 }, { d: '2020-03-01', v: 11 }, { d: '2020-05-01', v: 12 }, { d: '2024-01-01', v: 8 }]
  const c1 = buildMacroChart({ series: { policy_rate: rate }, ids: ['policy_rate'], view: { x0: -7, x1: 0.1 }, yRange: { lo: 5, hi: 14 }, todayIso: '2026-11-04' })
  assert.equal(c1.lines[0].dg, '')
  const rep = [{ d: '2018-12-31', v: 5 }, { d: '2019-12-31', v: 6 }, { d: '2020-12-31', v: 7 }, { d: '2023-12-31', v: 9 }, { d: '2024-12-31', v: 10 }]
  const c2 = buildMacroChart({ series: { sh_revenue: rep }, ids: ['sh_revenue'], view: { x0: -8, x1: 0.1 }, yRange: { lo: 4, hi: 11 }, todayIso: '2026-11-04' })
  assert.match(c2.lines[0].dg, /^M[\d.]+ [\d.]+H[\d.]+/) // удержание через 2021–2022
  assert.ok(c2.lines[0].dg.split('M').length >= 3, 'ещё и просроченный хвост после 2024') // два пунктирных куска
  assert.ok(c2.lines[0].d.includes('V')) // скачок на новом отчёте остаётся сплошным
})

test('подсказка: внутри пропуска «нет данных», на самой точке и в обычном месте — значение', () => {
  const hs = []
  for (let h = 0; h <= 20; h++) hs.push(h)
  for (let h = 57; h <= 66; h++) hs.push(h)
  const pts = hourly(hs)
  assert.equal(gapAt('p2p_ask', pts, stamp(T0 + 30 * H), '2026-11-04'), true)
  assert.equal(gapAt('p2p_ask', pts, stamp(T0 + 57 * H), '2026-11-04'), false)
  assert.equal(gapAt('p2p_ask', pts, stamp(T0 + 10 * H + 30 * 60000), '2026-11-04'), false)
  assert.equal(gapAt('p2p_ask', pts, stamp(T0 - 5 * H), '2026-11-04'), false)
  assert.notEqual(valueAt(pts, stamp(T0 + 30 * H)), null) // значение «по последней точке» по-прежнему есть — решает подсказка
  assert.equal(gapAt('policy_rate', [{ d: '2020-01-01', v: 1 }, { d: '2020-02-01', v: 2 }, { d: '2020-03-01', v: 3 }, { d: '2024-01-01', v: 4 }], '2022-01-01', '2026-11-04'), false)
})

test('кривая Treasury: срок без данных между двумя известными — пунктир', () => {
  const snaps = [{ key: 'now', color: '#fff', points: [{ label: '1м', v: 4 }, { label: '3м', v: null }, { label: '6м', v: 4.2 }, { label: '1г', v: 4.3 }] }]
  const c = curveChart(snaps)
  assert.equal((c.lines[0].d.match(/M/g) || []).length, 2)
  assert.match(c.lines[0].dg, /^M[\d.]+ [\d.]+L[\d.]+ [\d.]+$/)
  const lead = curveChart([{ key: 'x', color: '#fff', points: [{ label: 'a', v: null }, { label: 'b', v: 1 }, { label: 'c', v: 2 }] }])
  assert.equal(lead.lines[0].dg, '') // пустое начало — не пропуск между точками
  void MACRO_PAD; void VIEW
})
