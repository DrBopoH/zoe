// Режим прокрутки стопки уведомлений: где пользователь «держит» стопку.
//   top    — у начала: видны новые (по умолчанию, пока стопка не переполнена)
//   bottom — у конца: видны старые, которые вот-вот уйдут
//   free   — где-то посередине: ничего не двигаем, место держит браузерный scroll anchoring
// Режим запоминается по событию scroll, то есть ДО изменения высоты; после появления/ухода тоста стопку возвращают
// в тот же режим — иначе при смене высоты ползунок «не знает себе места».
const EPS = 2

export const scrollMode = (top, scrollHeight, clientHeight) => {
  if (scrollHeight - clientHeight <= EPS) return 'top'
  if (top <= EPS) return 'top'
  if (scrollHeight - clientHeight - top <= EPS) return 'bottom'
  return 'free'
}

// Куда поставить scrollTop после изменения размера; null — не трогать
export const scrollTarget = (mode, scrollHeight, clientHeight) => {
  if (mode === 'top') return 0
  if (mode === 'bottom') return Math.max(0, scrollHeight - clientHeight)
  return null
}
