import test from 'node:test'
import assert from 'node:assert/strict'
import { SERIES, realWageGrowth, FED_INFLATION_TARGET, unitText } from '../src/utils/macro.js'

test('у каждого ряда цен, труда и активности есть подпись и цвет', () => {
  for (const id of ['us_corecpi_yoy', 'us_corecpi_mom', 'us_pce_yoy', 'us_corepce_yoy', 'us_ppi_yoy', 'us_shelter_yoy', 'us_nfp', 'us_claims', 'us_jolts', 'us_wages_yoy', 'us_u6', 'us_eci_yoy', 'us_retail_yoy', 'us_indpro_yoy', 'us_housing', 'us_homeprice_yoy']) {
    assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
  }
})

test('реальные зарплаты: заработок минус инфляция на тот же месяц', () => {
  const wages = [{ d: '2026-08-01', v: 4.2 }, { d: '2026-09-01', v: 3.9 }]
  const cpi = [{ d: '2026-08-01', v: 3.1 }, { d: '2026-09-01', v: 3.0 }, { d: '2026-10-01', v: 2.9 }]
  assert.deepEqual(realWageGrowth(wages, cpi), { d: '2026-09-01', v: 0.9 })
  assert.deepEqual(realWageGrowth([{ d: '2026-09-01', v: 2 }], [{ d: '2026-08-01', v: 3.1 }]), { d: '2026-09-01', v: -1.1 })
  assert.equal(realWageGrowth([], cpi), null)
  assert.equal(realWageGrowth(wages, []), null, 'нет инфляции — нет пары')
  assert.equal(realWageGrowth(wages, [{ d: '2026-10-01', v: 3 }]), null, 'инфляция начинается позже заработка')
})

test('цель ФРС и подписи единиц для тысяч и человек', () => {
  assert.equal(FED_INFLATION_TARGET, 2)
  assert.equal(unitText(212000, 'чел.'), '212 тыс. чел.')
  assert.equal(unitText(1221, 'тыс.'), '1221 тыс.')
  assert.equal(unitText(-35.4, 'тыс.'), '-35,4 тыс.')
})
