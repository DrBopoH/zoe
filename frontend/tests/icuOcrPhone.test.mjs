import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parseIcuScreen, mergeEntries, priceCandidates } from '../src/utils/icuOcr.js'
import { ytmFromPrice } from '../src/utils/marketTable.js'

// Четыре скриншота с телефона (Telegram-копии), по три прохода распознавания каждый: 15.11.2026, в день купона 15.11.
const shots = Object.values(JSON.parse(readFileSync(new URL('./fixtures/icu-phone-1115.json', import.meta.url), 'utf8')))
const today = '2026-11-15'

// Купон бумаги подбирается так, чтобы цена с экрана давала показанную доходность (реестра НБУ в тесте нет)
const couponFor = (maturity, price, ytm) => {
  let lo = 0, hi = 40
  for (let i = 0; i < 60; i++) {
    const mid = (lo + hi) / 2
    if (ytmFromPrice({ maturity, coupon_rate: mid, nominal: 1000 }, price, today) > ytm) hi = mid
    else lo = mid
  }
  return (lo + hi) / 2
}
const bond = (n, maturity, price, ytm) => ({ id: `UA4000${n}`, maturity, nominal: 1000, coupon_rate: price ? couponFor(maturity, price, ytm) : 10 })
const registry = [
  bond('239008', '2028-06-14', 1069.9, 16.1), bond('236475', '2028-09-27', 1058.72, 16.1), bond('237556', '2028-11-15', 1125.56, 16.2),
  bond('235782', '2028-11-29', 1119.1, 16.2), bond('239107', '2029-02-07', 1047.78, 16.4), bond('238968', '2029-03-07', 1062.93, 16.5),
  bond('239131', '2029-03-21', 1036.46, 16.5), bond('239081', '2030-02-20', 1040.02, 17), bond('239115', '2027-09-15', 1030.51, 15),
  bond('239149', '2028-01-19', 1048.55, 15.65), bond('231625', '2028-02-09', 1063.29, 15.6), bond('237804', '2028-03-22', 1053.87, 15.7),
  bond('235196', '2028-04-12', 1044.36, 16.1), bond('239040', '2028-04-26', 1012.95, 16.1), bond('238281', '2026-12-16'),
  bond('236228', '2027-03-10'), bond('233704', '2027-04-14'), bond('230270', '2027-07-07', 1088.27, 15.3),
  bond('239016', '2027-07-21', 1050.8, 15.3), bond('231195', '2027-08-25', 1058.38, 15.3),
  bond('197545', '2027-05-10') // настоящий выпуск реестра, которого на экране ICU нет
]

const entries = () => mergeEntries(shots.map((texts) => mergeEntries(texts.map((t) => parseIcuScreen(t, registry, today)))))
const by = () => Object.fromEntries(entries().map((e) => [e.isin, e]))

test('цена читается без запятой и без двоеточия у «Кількість»', () => {
  assert.ok(priceCandidates('Kinbkicts: 29 972 1012958').includes(1012.95)) // «1 012,95 ₴»: запятая потеряна, ₴ стал цифрой
  assert.ok(priceCandidates('KinbkicTe: 21 283 1119102').includes(1119.1))
  assert.ok(priceCandidates('Kinbkicro: 3 700 112556').includes(1125.56)) // значка нет, запятая потеряна
  assert.ok(priceCandidates('Kinoricie, 10982 1 044,36 e').includes(1044.36)) // двоеточие стало запятой
  assert.deepEqual(priceCandidates('Кількість: 3 768'), [])
})

test('телефонные скриншоты: на экране 20 бумаг, цены прочитаны у всех', () => {
  const m = by()
  const real = Object.values(m).filter((e) => e.isin !== 'UA4000197545')
  assert.equal(real.length, 20)
  assert.equal(m.UA4000237556.price, 1125.56) // купон 15.11 выплачивается сегодня: цена ещё с ним
  assert.equal(m.UA4000237804.price, 1053.87)
  assert.equal(m.UA4000235196.price, 1044.36)
  assert.equal(m.UA4000239040.price, 1012.95)
  assert.equal(m.UA4000239016.price, 1050.8)
  assert.equal(m.UA4000231195.price, 1058.38) // дата соседней карточки не должна отдавать ей чужую цену
  assert.equal(m.UA4000236228.price, 1039.71)
})

test('бумага, которой нет на экране, помечена и не подтверждена; настоящая рядом без пометки', () => {
  const m = by()
  assert.ok(m.UA4000197545.flags.includes('isin-weak')) // дата «10.03.2027» прочиталась как «10.05.2027»
  assert.ok(!m.UA4000236228.flags.includes('isin-weak'))
  const clean = Object.values(m).filter((e) => e.flags.includes('isin-weak')).map((e) => e.isin)
  assert.deepEqual(clean, ['UA4000197545'])
})

test('бумага, которую хоть один проход прочитал уверенно, слабой не остаётся', () => {
  const weak = { isin: 'UA4000236228', maturity: '2027-03-10', kind: 'SIM', shown: 14.1, ytm: 14.8, price: null, flags: ['isin-weak'] }
  const sure = { ...weak, flags: [], price: 1039.71 }
  assert.deepEqual(mergeEntries([[weak], [sure]])[0].flags, [])
  assert.deepEqual(mergeEntries([[weak]])[0].flags, ['isin-weak'])
})
