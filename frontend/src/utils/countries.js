import { lastPoint, plainPct } from './macro.js'

// Показатели карточки страны в порядке показа. Какой ряд за какой показатель отвечает у страны, решает сервер (реестр стран в /api/macro/catalog):
// здесь только подписи и единицы. Нет показателя у страны — строки в карточке нет.
export const COUNTRY_METRICS = [
  { key: 'policy', label: 'Ключевая ставка', unit: '%' },
  { key: 'cpi', label: 'Инфляция г/г', unit: '%' },
  { key: 'cpi_core', label: 'Базовая инфляция г/г', unit: '%' },
  { key: 'u', label: 'Безработица', unit: '%' },
  { key: 'gdp', label: 'Рост ВВП', unit: '%' },
  { key: 'y10', label: 'Доходность 10 лет', unit: '%' },
  { key: 'fx', label: 'Курс к гривне', unit: '₴' }
]

const money = (v) => `${String(Math.round(v * 100) / 100).replace('.', ',')} ₴`

// Все id рядов, нужные карточкам стран (без повторов): по ним читается кеш одним запросом
export const countryIds = (countries) => [...new Set((countries || []).flatMap((c) => Object.values(c.metrics || {})))]

// Строки карточки страны: подпись, id ряда, последняя точка и текст значения («—», пока данных нет)
export function countryRows(country, series) {
  return COUNTRY_METRICS.filter((m) => country?.metrics?.[m.key]).map((m) => {
    const id = country.metrics[m.key]
    const last = lastPoint(series?.[id])
    return { key: m.key, label: m.label, id, unit: m.unit, last, text: last ? (m.unit === '%' ? plainPct(last.v) : money(last.v)) : '—' }
  })
}
