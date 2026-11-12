import { counterpartyName } from './personLayout.js'

// Алиас контрагента → Лицо. Сырые алиасы из операций видны только в «Бесхозе» и резолвере конфликтов;
// в остальных местах (история операций) имя контрагента подменяется на привязанное Лицо.
//
// Бэкенд уже решил, чьё это (transaction.resolved_user_id — по алиасу и по привязанным счетам/картам), здесь только подстановка.

export const usersById = (users) => new Map((users || []).map(u => [String(u.id), u]))

export const personOf = (t, byId) => (t && t.resolved_user_id && byId ? byId.get(String(t.resolved_user_id)) || null : null)

// Заменяет сырое имя контрагента внутри текста (заголовок, описание) на имя Лица
export const swapAlias = (text, t, person) => {
  if (!text || !person) return text
  const raw = counterpartyName(t)
  if (!raw) return text
  const i = text.toLowerCase().indexOf(raw.toLowerCase())
  return i < 0 ? text : text.slice(0, i) + person.name + text.slice(i + raw.length)
}
