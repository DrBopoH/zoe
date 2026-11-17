import { valueAt, lastPoint, changeSince } from './macro.js'
import { intText, fixedText } from './numfmt.js'
// Сырьё: цены в долларах (FRED) и металлы из курсов НБУ (гривен за унцию). Экран показывает всё либо в долларах, либо в гривнах:
// пересчёт — по курсу доллара НБУ на дату каждой точки (курс — последний не позже даты). Чистые функции, без интерфейса.
// native: в чём цена приходит — usd (доллары), cent (центы за фунт: сахар, кофе), uah (гривны: металлы НБУ).
export const COMMODITIES = [
  { id: 'met_xau', src: 'xau_uah', label: 'Золото', per: 'унцию', native: 'uah', group: 'precious' },
  { id: 'met_xag', src: 'xag_uah', label: 'Серебро', per: 'унцию', native: 'uah', group: 'precious' },
  { id: 'met_xpt', src: 'xpt_uah', label: 'Платина', per: 'унцию', native: 'uah', group: 'precious' },
  { id: 'met_xpd', src: 'xpd_uah', label: 'Палладий', per: 'унцию', native: 'uah', group: 'precious' },
  { id: 'com_brent', src: 'com_brent', label: 'Нефть Brent', per: 'баррель', native: 'usd', group: 'oil' },
  { id: 'com_wti', src: 'com_wti', label: 'Нефть WTI', per: 'баррель', native: 'usd', group: 'oil' },
  { id: 'com_gas_us', src: 'com_gas_us', label: 'Газ США (Henry Hub)', per: 'MMBtu', native: 'usd', group: 'gas' },
  { id: 'com_gas_eu', src: 'com_gas_eu', label: 'Газ Европа (TTF)', per: 'MMBtu', native: 'usd', group: 'gas' },
  { id: 'com_copper', src: 'com_copper', label: 'Медь', per: 'тонну', native: 'usd', group: 'base' },
  { id: 'com_alum', src: 'com_alum', label: 'Алюминий', per: 'тонну', native: 'usd', group: 'base' },
  { id: 'com_nickel', src: 'com_nickel', label: 'Никель', per: 'тонну', native: 'usd', group: 'base' },
  { id: 'com_wheat', src: 'com_wheat', label: 'Пшеница', per: 'тонну', native: 'usd', group: 'grain' },
  { id: 'com_corn', src: 'com_corn', label: 'Кукуруза', per: 'тонну', native: 'usd', group: 'grain' },
  { id: 'com_rice', src: 'com_rice', label: 'Рис', per: 'тонну', native: 'usd', group: 'grain' },
  { id: 'com_soy', src: 'com_soy', label: 'Соя', per: 'тонну', native: 'usd', group: 'grain' },
  { id: 'com_cocoa', src: 'com_cocoa', label: 'Какао', per: 'тонну', native: 'usd', group: 'grain' },
  { id: 'com_sugar', src: 'com_sugar', label: 'Сахар', per: 'фунт', native: 'cent', group: 'softs' },
  { id: 'com_coffee', src: 'com_coffee', label: 'Кофе (арабика)', per: 'фунт', native: 'cent', group: 'softs' }
]

export const COMMODITY_GROUPS = [
  { key: 'precious', title: 'Драгоценные металлы', per: 'унцию', hint: 'Курсы НБУ. Золото — ориентир «защитных» денег, платина и палладий — ещё и промышленный спрос' },
  { key: 'oil', title: 'Нефть', per: 'баррель', hint: 'Ежедневно, EIA через FRED. Brent — мировой ориентир, WTI — американский' },
  { key: 'gas', title: 'Природный газ', per: 'MMBtu', hint: 'Henry Hub — ежедневно, Европа — помесячно (Всемирный банк). Разрыв между ними — цена доставки и дефицита' },
  { key: 'base', title: 'Промышленные металлы', per: 'тонну', hint: 'Помесячно, Всемирный банк. Медь называют барометром мировой экономики' },
  { key: 'grain', title: 'Зерно, соя и какао', per: 'тонну', hint: 'Помесячно, Всемирный банк. Пшеница и кукуруза — главный экспорт Украины' },
  { key: 'softs', title: 'Сахар и кофе', per: 'фунт', hint: 'Помесячно, Всемирный банк. В источнике цена в центах за фунт; в долларах и гривнах показана за фунт' }
]

export const groupIds = (key) => COMMODITIES.filter((c) => c.group === key).map((c) => c.id)

// Один ряд в выбранной валюте. mode: 'usd' | 'uah'. Нет курса доллара на дату — точка пропускается (только там, где курс нужен).
export function convertSeries(points, usd, native, mode) {
  if (!points?.length) return []
  const out = []
  for (const p of points) {
    let v = p.v
    if (native === 'cent') v /= 100
    if ((native === 'uah') !== (mode === 'uah')) {
      const r = valueAt(usd, p.d)
      if (!(r > 0)) continue
      v = mode === 'uah' ? v * r : v / r
    }
    out.push({ d: p.d, v })
  }
  return out
}

// Все ряды экрана под ключами COMMODITIES[].id: series — ряды сырья и металлов (com_*, xau_uah…, usd_uah)
export function commodityView(series, mode) {
  const usd = series?.usd_uah
  const out = {}
  for (const c of COMMODITIES) out[c.id] = convertSeries(series?.[c.src], usd, c.native, mode)
  return out
}

// Строки таблицы: последняя цена, дата и изменение за год. view — результат commodityView.
export function commodityRows(view, today) {
  return COMMODITIES.map((c) => {
    const last = lastPoint(view[c.id])
    return { id: c.id, label: c.label, per: c.per, last, chg1y: changeSince(view[c.id], today, 1) }
  })
}

// Цена с точностью по размеру: дешёвое (газ, сахар) — две десятичные, крупное — целое с пробелами
export function priceText(v) {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  if (Math.abs(v) >= 100) return intText(v)
  return fixedText(Math.round(v * 100) / 100, 2)
}
