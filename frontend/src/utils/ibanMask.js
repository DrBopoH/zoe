import { reactive } from 'vue'
import { bankTheme } from './bankTheme.js'

// Скрытый IBAN хранится цифрами («UA44-322001-0000-9976»: страна+контрольные, код банка, 4 первые, 4 последние).
// Флаг и значок банка подставляются на лету здесь; своё имя счёта (карандаш) не подходит под шаблон и показывается как есть.
const MASK_RE = /^([A-Z]{2})(\d{2})-(\w{4,6})-(\w{4})-(\w{4})$/

// профиль Binance: «BN-0041» — вместо префикса BN рисуется значок Binance
const BINANCE_RE = /^BN-(\w{4})$/

export const parseMask = (m) => {
  const b = BINANCE_RE.exec(String(m || ''))
  if (b) return { binance: true, tail: b[1] }
  const x = MASK_RE.exec(String(m || ''))
  return x ? { country: x[1], check: x[2], bank: x[3], head: x[4], tail: x[5] } : null
}

// Значок Binance: логотип из public/banks/binance.svg, если положишь; без файла — монограмма в цвете Binance
export const binanceMark = () => ({ letter: 'B', color: bankTheme('binance').color, name: 'Binance', logo: '/banks/binance.svg' })

// флаг по коду страны: UA рисуем SVG (эмодзи-флаги не показываются в Windows), остальные — эмодзи
export const flagEmoji = (cc) => [...String(cc).toUpperCase()].map(c => String.fromCodePoint(127397 + c.charCodeAt(0))).join('')

// Код банка (МФО) → значок. Логотип подхватывается из public/banks/<МФО>.svg для любого кода (кладёшь файл — он появляется);
// без файла — монограмма в цвете банка. Код, которого нет в списке, — нейтральная монограмма из его первых цифр.
// Нужен новый банк — одна строка здесь.
const BANKS = {
  322001: { key: 'mono', letter: 'M', name: 'monobank' },
  305299: { key: 'privat', letter: 'П', name: 'ПриватБанк' },
  300335: { key: 'raif', letter: 'R', name: 'Райффайзен' },
  380946: { key: 'icu', letter: 'I', name: 'ICU' }
}

// логотипы, которых нет в public/banks: один раз не нашли — больше не запрашиваем (общее на все компоненты)
export const failedLogos = reactive(new Set())
export const bankMark = (code) => {
  const b = BANKS[code]
  // логотип ищем для любого кода (не только из списка): положил <код>.svg в public/banks — он появился
  if (!b) return { letter: String(code).slice(0, 2), color: '#64748b', name: `Код банка ${code}`, logo: `/banks/${code}.svg`, small: true }
  return { letter: b.letter, color: bankTheme(b.key).color, name: b.name, logo: `/banks/${code}.svg` }
}
