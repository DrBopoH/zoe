import test from 'node:test'
import assert from 'node:assert/strict'
import { multipleAt, niceTicks, buildGrowthChart, autoView, zoomView, panView, timeTicks } from '../src/utils/growthChart.js'

const sc = (years, rate, base = 10) => ({ years, rate, base })

test('кратность: до погашения под ставку бумаги, дальше (если режим сравнения) под базу', () => {
  assert.ok(Math.abs(multipleAt(sc(2, 10), 2, false) - 1.21) < 1e-9)
  assert.ok(Math.abs(multipleAt(sc(2, 10), 5, false) - 1.21) < 1e-9) // реальный режим: после погашения рост стоит
  assert.ok(Math.abs(multipleAt(sc(2, 20, 10), 4, true) - 1.44 * 1.21) < 1e-9)
  assert.equal(multipleAt(sc(5, 10), 0, true), 1)
})

test('круглые деления шкалы без хвоста плавающей точки', () => {
  assert.deepEqual(niceTicks(0, 10, 5), [0, 2, 4, 6, 8, 10])
  assert.deepEqual(niceTicks(1, 4.5, 4), [1, 2, 3, 4])
  assert.deepEqual(niceTicks(5, 5), [5])
  assert.deepEqual(niceTicks(1, 1.8, 6), [1, 1.2, 1.4, 1.6, 1.8]) // раньше: 1.4000000000000001
  for (const v of niceTicks(1.0000001, 1.0000009, 5)) assert.equal(String(v).length < 14, true)
})

const today = '2026-01-01'
const items = [
  { id: 'L', color: 'red', top: false, sc: sc(2, 5) },
  { id: 'A', color: 'green', top: true, sc: sc(8, 16) },
  { id: 'B', color: 'lime', top: true, sc: sc(15, 14) }
]
const burnedItem = { id: 'OLD', rate: 10, startT: -3, endT: -1 }

test('окно по умолчанию: от сегодня до самой долгой из лучших; со сгоревшими — слева до самого раннего выпуска', () => {
  const v = autoView(items)
  assert.equal(v.x0, 0); assert.ok(v.x1 >= 15 && v.x1 < 16)
  assert.equal(autoView(items, [burnedItem]).x0, -3)
  assert.ok(autoView([{ id: 'S', top: true, sc: sc(1, 5) }]).x1 >= 10) // не уже горизонта
  assert.ok(autoView([{ id: 'Z', top: true, sc: sc(29, 5) }]).x1 < 16) // 29-летняя бумага окно не растягивает: дальше колесом
})

test('приближение держит якорь на месте и зажато границами', () => {
  const z = zoomView({ x0: 0, x1: 10 }, 0.5, 4)
  assert.ok(Math.abs(z.x1 - z.x0 - 5) < 1e-9)
  assert.ok(Math.abs((4 - z.x0) / (z.x1 - z.x0) - 0.4) < 1e-9) // якорь 4 лет остался на 40 % ширины
  assert.ok(zoomView({ x0: 0, x1: 0.3 }, 0.1, 0.1).x1 - zoomView({ x0: 0, x1: 0.3 }, 0.1, 0.1).x0 >= 0.25 - 1e-9)
  assert.ok(zoomView({ x0: 0, x1: 50 }, 10, 5).x1 - zoomView({ x0: 0, x1: 50 }, 10, 5).x0 <= 60 + 1e-9)
  assert.deepEqual(panView({ x0: 1, x1: 3 }, 0.5), { x0: 1.5, x1: 3.5 })
})

test('деления времени: годы на широком окне, месяцы на узком', () => {
  const wide = timeTicks(0, 12, today)
  assert.deepEqual(wide.map((t) => t.label), ['2026', '2028', '2030', '2032', '2034', '2036', '2038']) // шаг 2 года
  const narrow = timeTicks(0, 1, today)
  assert.deepEqual(narrow.map((t) => t.label), ['02.2026', '05.2026', '08.2026', '11.2026']) // шаг 3 месяца
  assert.equal(timeTicks(0, 0.5, today).length, 6) // на узком окне — каждый месяц
})

test('график: линии до погашения сплошные, дальше пунктир до горизонта; масштаб по лучшим', () => {
  const c = buildGrowthChart({ items, amount: 1000, today })
  const a = c.lines.find((l) => l.id === 'A'), b = c.lines.find((l) => l.id === 'B'), low = c.lines.find((l) => l.id === 'L')
  assert.notEqual(a.dashed, ''); assert.equal(b.dashed, '') // у B погашение дальше горизонта
  assert.ok(Math.abs(a.mat.v - 1000 * Math.pow(1.16, 8)) < 1e-6)
  assert.ok(a.mat.x > c.nowX && a.mat.x < b.mat.x) // 8 лет левее 15
  assert.ok(low.mat.y > a.mat.y) // меньше сумма — ниже на экране
  assert.ok(c.horizonX > c.nowX)
  assert.ok(c.hi >= b.mat.v) // шкала вмещает самую большую из лучших
})

test('сгоревшие бумаги: серая линия от выпуска до погашения, окно уходит в прошлое', () => {
  const c = buildGrowthChart({ items, matured: [burnedItem], amount: 1000, today })
  assert.equal(c.burned.length, 1)
  assert.equal(c.view.x0, -3)
  assert.ok(c.burned[0].end.x < c.nowX) // всё левее «сегодня»
  assert.ok(Math.abs(c.burned[0].end.v - 1000 * Math.pow(1.1, 2)) < 1e-6)
})

test('масштаб по вертикали следует за окном; пустой список и бумага дольше 30 лет не ломают построение', () => {
  const full = buildGrowthChart({ items, amount: 1000, today })
  const zoomed = buildGrowthChart({ items, amount: 1000, today, view: { x0: 0, x1: 2 } })
  assert.ok(zoomed.hi < full.hi) // ближе к сегодняшнему дню суммы меньше
  assert.equal(buildGrowthChart({ items: [], today }).lines.length, 0)
  const c = buildGrowthChart({ items: [{ id: 'X', top: true, color: 'red', sc: sc(40, 10) }], amount: 100, today })
  assert.equal(c.lines[0].mat, null)
  assert.ok(c.view.x1 <= 31)
})
