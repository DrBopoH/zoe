import test from 'node:test'
import assert from 'node:assert/strict'
import { COMMODITIES, COMMODITY_GROUPS, groupIds, convertSeries, commodityView, commodityRows, priceText } from '../src/utils/commodities.js'
import { SERIES } from '../src/utils/macro.js'

const usd = [{ d: '2025-01-01', v: 40 }, { d: '2026-01-01', v: 42 }]

test('доллары в гривны по курсу даты; гривны в доллары обратно', () => {
  const brent = [{ d: '2025-06-01', v: 80 }, { d: '2026-02-01', v: 90 }]
  assert.deepEqual(convertSeries(brent, usd, 'usd', 'uah'), [{ d: '2025-06-01', v: 3200 }, { d: '2026-02-01', v: 3780 }])
  assert.deepEqual(convertSeries(brent, usd, 'usd', 'usd'), brent)
  assert.deepEqual(convertSeries([{ d: '2026-02-01', v: 8400 }], usd, 'uah', 'usd'), [{ d: '2026-02-01', v: 200 }])
  assert.deepEqual(convertSeries([{ d: '2026-02-01', v: 8400 }], usd, 'uah', 'uah'), [{ d: '2026-02-01', v: 8400 }])
})

test('центы за фунт: в долларах делятся на 100, в гривнах тоже и умножаются на курс', () => {
  const sugar = [{ d: '2026-02-01', v: 20 }]
  assert.deepEqual(convertSeries(sugar, usd, 'cent', 'usd'), [{ d: '2026-02-01', v: 0.2 }])
  assert.ok(Math.abs(convertSeries(sugar, usd, 'cent', 'uah')[0].v - 8.4) < 1e-9)
})

test('нет курса на дату — точка пропускается; нет данных — пусто', () => {
  assert.deepEqual(convertSeries([{ d: '2024-01-01', v: 80 }], usd, 'usd', 'uah'), [])
  assert.deepEqual(convertSeries(null, usd, 'usd', 'uah'), [])
  assert.deepEqual(convertSeries([{ d: '2026-02-01', v: 80 }], [], 'usd', 'uah'), [])
})

test('вид экрана: ключи COMMODITIES, металлы из xau_uah/xag_uah', () => {
  const series = { usd_uah: usd, xau_uah: [{ d: '2026-02-01', v: 84000 }], com_brent: [{ d: '2026-02-01', v: 100 }] }
  const v = commodityView(series, 'usd')
  assert.equal(Object.keys(v).length, COMMODITIES.length)
  assert.equal(v.met_xau[0].v, 2000)
  assert.equal(v.com_brent[0].v, 100)
  assert.deepEqual(v.met_xag, [])
})

test('строки таблицы: цена, изменение за год, нет данных — null', () => {
  const view = { ...commodityView({}, 'usd'), com_wti: [{ d: '2025-05-01', v: 50 }, { d: '2026-05-01', v: 60 }] }
  const rows = commodityRows(view, '2026-06-01')
  const wti = rows.find((r) => r.id === 'com_wti')
  assert.equal(wti.last.v, 60)
  assert.ok(Math.abs(wti.chg1y - 20) < 1e-9)
  assert.equal(rows.find((r) => r.id === 'com_copper').last, null)
})

test('группы покрывают все ряды, у каждого есть подпись и цвет', () => {
  const all = COMMODITY_GROUPS.flatMap((g) => groupIds(g.key)).sort()
  assert.deepEqual(all, COMMODITIES.map((c) => c.id).sort())
  for (const c of COMMODITIES) assert.ok(SERIES[c.id]?.label && SERIES[c.id]?.color, c.id)
})

test('цена словами', () => {
  assert.equal(priceText(3.456), '3,46')
  assert.equal(priceText(14473.4), (14473).toLocaleString('ru-RU'))
  assert.equal(priceText(null), '—')
})
