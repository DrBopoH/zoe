import test from 'node:test'
import assert from 'node:assert/strict'
import { feedStatus, agoText, lagText, seriesRows, filterSeries, seriesCsv, healthSummary } from '../src/utils/sources.js'

test('состояние источника: ошибка важнее всего, затем пусто, устарел, неполный, свежий', () => {
  assert.equal(feedStatus({ error: 'HTTP 429', cached: 5, series: 5, stale: 0 }), 'error')
  assert.equal(feedStatus({ cached: 0, series: 5, stale: 0 }), 'empty')
  assert.equal(feedStatus({ cached: 5, series: 5, stale: 2 }), 'stale')
  assert.equal(feedStatus({ cached: 4, series: 5, stale: 0 }), 'partial')
  assert.equal(feedStatus({ cached: 5, series: 5, stale: 0 }), 'ok')
})

test('возраст обновления и отставание словами', () => {
  const now = Date.UTC(2026, 10, 15, 12) 
  const at = (s) => Math.floor(now / 1000) - s
  assert.equal(agoText(0, now), 'нет')
  assert.equal(agoText(at(10), now), 'только что')
  assert.equal(agoText(at(5 * 60), now), '5 мин назад')
  assert.equal(agoText(at(3 * 3600), now), '3 ч назад')
  assert.equal(agoText(at(2 * 86400), now), '2 дн. назад')
  assert.equal(lagText(0), 'сегодня')
  assert.equal(lagText(12), '12 дн.')
  assert.equal(lagText(150), '5 мес.')
  assert.equal(lagText(550), '1,5 г.')
})

const health = {
  feeds: [{ id: 'us', title: 'Ставка ФРС · FRED', series: 2, cached: 2, stale: 1 }, { id: 'fx', title: 'Курсы · НБУ', series: 1, cached: 1, stale: 0 }],
  series: [
    { id: 'fed_funds', feed: 'us', freq: 'месяц', points: 800, from: '1954-07-01', to: '2026-10-01', lag_days: 45, stale: false },
    { id: 'us_vix', feed: 'us', freq: 'неделя', points: 1700, from: '1990-01-05', to: '2025-01-03', lag_days: 700, stale: true },
    { id: 'usd_uah', feed: 'fx', freq: 'день', points: 9000, from: '2000-01-01', to: '2026-11-14', lag_days: 1, stale: false },
    { id: 'orphan_x', feed: '', freq: 'год', points: 3, from: '2020-01-01', to: '2022-01-01', lag_days: 1400, stale: true }
  ]
}

test('каталог: подписи из справочника графиков, поиск по словам, фильтры', () => {
  const rows = seriesRows(health)
  assert.equal(rows.length, 4)
  assert.equal(rows.find((r) => r.id === 'us_vix').label, 'VIX (индекс страха)')
  assert.equal(rows.find((r) => r.id === 'orphan_x').feedTitle, '')
  assert.deepEqual(filterSeries(rows, {}).length, 4)
  assert.deepEqual(filterSeries(rows, { query: 'vix' }).map((r) => r.id), ['us_vix'])
  assert.deepEqual(filterSeries(rows, { query: 'ФРС fed' }).map((r) => r.id), ['fed_funds'])
  assert.deepEqual(filterSeries(rows, { query: 'нбу' }).map((r) => r.id), ['usd_uah'])
  assert.deepEqual(filterSeries(rows, { feed: 'us' }).map((r) => r.id), ['fed_funds', 'us_vix'])
  assert.deepEqual(filterSeries(rows, { staleOnly: true }).map((r) => r.id), ['us_vix', 'orphan_x'])
  assert.deepEqual(filterSeries(rows, { query: 'zzz' }), [])
  assert.deepEqual(seriesRows({}), [])
})

test('CSV ряда и сводка источников', () => {
  assert.equal(seriesCsv([{ d: '2026-01-01', v: 1.5 }, { d: '2026-02-01', v: 2 }]), 'date,value\n2026-01-01,1.5\n2026-02-01,2\n')
  assert.equal(seriesCsv(undefined), 'date,value\n')
  assert.deepEqual(healthSummary(health.feeds), { total: 2, ok: 1, stale: 1, error: 0, empty: 0 })
  assert.deepEqual(healthSummary(undefined), { total: 0, ok: 0, stale: 0, error: 0, empty: 0 })
})
