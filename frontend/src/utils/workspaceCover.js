// Обложка дела (рабочего пространства): цвет и/или картинка, чистая логика без Vue. Карточка рисует верхние 80% обложкой,
// а снизу кладёт тёмный градиент с названием; здесь — палитра, CSS обложки и проверка имени для формы.
export const NAME_MAX = 60 // как на бэкенде (db.maxWorkspaceName)
export const IMAGE_MAX = 400_000 // символов в data-URL, как на бэкенде: больше сервер не примет
export const IMAGE_URL_MAX = 2000 // символов в ссылке на обложку, как на бэкенде
export const COVER_SIDE = 640 // длинная сторона картинки после сжатия на фронте, px

// Спокойные насыщенные цвета: на тёмной теме читаются и не спорят с акцентом приложения (зелёный — один из них)
export const PALETTE = ['#10b981', '#0ea5e9', '#6366f1', '#a855f7', '#ec4899', '#f43f5e', '#f97316', '#eab308', '#14b8a6', '#64748b']
export const FALLBACK_COLOR = '#334155'

/** Имя дела для записи и проверки: пробелы по краям и повторы схлопываются. */
export const cleanName = (name) => String(name ?? '').trim().replace(/\s+/g, ' ')

/** Можно ли создавать: имя непустое и не длиннее NAME_MAX (считаются символы, а не UTF-16 единицы). */
export const nameOk = (name) => {
  const n = cleanName(name)
  return n.length > 0 && [...n].length <= NAME_MAX
}

/** Ссылка http(s) на картинку без пробелов, кавычек и скобок (её кладут в CSS url("…")) или data:image — как проверяет бэкенд. */
export const imageOk = (v) => typeof v === 'string' && (/^data:image\/(png|jpeg|webp);base64,[A-Za-z0-9+/=]+$/.test(v) || (v.length <= IMAGE_URL_MAX && /^https?:\/\/[^\s"'()\\]+$/.test(v)))

/** Цвет #rrggbb (в любом регистре); иначе пусто. */
export const validColor = (c) => (/^#[0-9a-fA-F]{6}$/.test(String(c ?? '').trim()) ? String(c).trim().toLowerCase() : '')

/** Цвет обложки по умолчанию для имени: один и тот же для того же имени, разные для разных. */
export function colorFor(name) {
  let h = 0
  for (const ch of cleanName(name)) h = (h * 31 + ch.codePointAt(0)) >>> 0
  return PALETTE[h % PALETTE.length]
}

/**
 * CSS верхней части карточки: картинка поверх цвета (если есть), цвет подложкой. Без обложки — нейтральный цвет.
 * Возвращает объект для :style.
 */
export function coverStyle(ws) {
  const color = validColor(ws?.color) || FALLBACK_COLOR
  const image = imageOk(ws?.image) ? ws.image : ''
  return image
    ? { backgroundColor: color, backgroundImage: `url("${image}")`, backgroundSize: 'cover', backgroundPosition: 'center' }
    : { backgroundColor: color, backgroundImage: `linear-gradient(135deg, rgb(255 255 255 / 0.14), rgb(0 0 0 / 0.18))` }
}

/** Размер после сжатия: длинная сторона не больше side, пропорции целы, меньше не растягиваем. */
export function fitSize(w, h, side = COVER_SIDE) {
  const k = Math.min(1, side / Math.max(w, h, 1))
  return { w: Math.max(1, Math.round(w * k)), h: Math.max(1, Math.round(h * k)) }
}

// Сжатие файла в data-URL (только браузер): картинка уменьшается до COVER_SIDE и пишется JPEG, качество снижается, пока не влезет в IMAGE_MAX.
export async function readCoverImage(file, { side = COVER_SIDE } = {}) {
  if (!file || !/^image\//.test(file.type)) throw new Error('Нужна картинка (png, jpeg, webp)')
  const bmp = await createImageBitmap(file)
  const { w, h } = fitSize(bmp.width, bmp.height, side)
  const canvas = document.createElement('canvas')
  canvas.width = w
  canvas.height = h
  canvas.getContext('2d').drawImage(bmp, 0, 0, w, h)
  bmp.close?.()
  for (const q of [0.85, 0.72, 0.6, 0.45, 0.3]) {
    const url = canvas.toDataURL('image/jpeg', q)
    if (url.length <= IMAGE_MAX) return url
  }
  throw new Error('Картинка слишком тяжёлая даже после сжатия')
}
